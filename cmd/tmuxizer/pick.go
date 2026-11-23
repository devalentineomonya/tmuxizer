package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/devalentineomonya/tmuxizer/internal/config"
	"github.com/devalentineomonya/tmuxizer/internal/iwashere"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
	"github.com/devalentineomonya/tmuxizer/internal/tmux"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
	"github.com/devalentineomonya/tmuxizer/internal/ui"
)

// closedCommand is invoked by the tmux session-closed hook, not by users.
const closedCommand = "_closed"

func pick() error {
	// `t` opens a shell whose startup file runs this picker again; skip that one.
	if consumeShellSkip(config.StateDir(), os.Getppid(), time.Now()) {
		return nil
	}
	if !tmux.Available() {
		return errors.New("tmux is not installed")
	}
	cfg, err := config.Load()
	if err != nil {
		// A broken config should not lock the user out of their projects.
		fmt.Fprintln(os.Stderr, "tmuxizer:", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	st := store.New(config.StateDir())
	lch := launcher.New(st, exe)
	lch.TitleTemplate = cfg.SessionName

	state, err := st.Load()
	if err != nil {
		return err
	}

	deps := ui.Deps{
		Theme:         theme.New(cfg.Accent),
		Projects:      cachedProjects(state),
		Rescan:        rescan(cfg, st),
		State:         state,
		Live:          liveProjects(lch, state),
		Editors:       tools.Editors(tools.Default(), cfg.Editor),
		Agents:        tools.Agents(tools.Default()),
		Tracking:      iwashere.Available(),
		TitleTemplate: cfg.SessionName,
		Orphans:       lch.Orphans(state),
		Dismiss:       lch.Dismiss,
		Close:         lch.Close,
	}
	if cwd, err := os.Getwd(); err == nil {
		if p, ok := project.Containing(cwd); ok {
			deps.Current = p.Path
			deps.Projects = append(deps.Projects, p)
		}
	}

	res, err := ui.Run(deps)
	if err != nil {
		return err
	}
	switch res.Action {
	case ui.Launch:
		return lch.Launch(res.Plan)
	case ui.Shell:
		return execShell(res.Dir)
	}
	return nil
}

func cachedProjects(st store.State) []project.Project {
	out := make([]project.Project, len(st.Projects))
	for i, c := range st.Projects {
		out[i] = project.Project{Name: c.Name, Path: c.Path}
	}
	return out
}

// rescan discovers projects in the background and refreshes the cache.
func rescan(cfg config.Config, st *store.Store) func() []project.Project {
	return func() []project.Project {
		found := project.Discover(cfg.Roots, cfg.Depth, cfg.Ignore)
		_ = st.Update(func(s *store.State) {
			s.Projects = s.Projects[:0]
			for _, p := range found {
				s.Projects = append(s.Projects, store.CachedProject{Name: p.Name, Path: p.Path})
			}
		})
		return found
	}
}

// liveProjects maps each project path to its running tmux session.
func liveProjects(l *launcher.Launcher, st store.State) map[string]string {
	live := map[string]string{}
	for _, s := range st.Sessions {
		if name, ok := l.Live(project.New(s.Path)); ok {
			live[s.Path] = name
		}
	}
	return live
}

// sessionClosed handles the tmux session-closed hook.
func sessionClosed(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: tmuxizer _closed <session>")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return launcher.New(store.New(config.StateDir()), exe).Closed(args[0])
}
