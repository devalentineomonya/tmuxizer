package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/devalentineomonya/tmuxizer/internal/config"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/report"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
	"github.com/devalentineomonya/tmuxizer/internal/ui"
)

// runTime prints time spent per project: tmuxizer time [today|week|all].
func runTime(args []string) error {
	period := report.Week
	if len(args) > 0 {
		p, ok := report.Parse(args[0])
		if !ok {
			return fmt.Errorf("usage: tmuxizer time [today|week|all]")
		}
		period = p
	}

	cfg, _ := config.Load()
	state, err := store.New(config.StateDir()).Load()
	if err != nil {
		return err
	}
	projects := cachedProjects(state)
	if len(projects) == 0 {
		projects = project.Discover(cfg.Roots, cfg.Depth, cfg.Ignore)
	}
	if cwd, err := os.Getwd(); err == nil {
		if p, ok := project.Containing(cwd); ok {
			projects = append(projects, p)
		}
	}

	th := theme.New(cfg.Accent)
	width := 80
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		width = w
	}
	rep := report.Build(projects, period, time.Now())
	fmt.Println(th.Accent.Render("▌time") + th.Strong.Render(" spent") + th.Dim.Render("  "+period.String()))
	fmt.Println()
	fmt.Println(ui.RenderReport(th, rep, width))
	return nil
}
