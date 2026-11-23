// Package doctor checks that tmuxizer's environment is ready, without
// changing anything.
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/devalentineomonya/tmuxizer/internal/config"
	"github.com/devalentineomonya/tmuxizer/internal/setup"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

// Status is a check's outcome.
type Status int

const (
	// OK means the check passed.
	OK Status = iota
	// Warn means a feature is degraded but tmuxizer works.
	Warn
	// Fail means tmuxizer cannot work.
	Fail
)

// Check is one line of the report.
type Check struct {
	Name   string
	Status Status
	Detail string
	Hint   string // how to fix, shown for Warn and Fail
}

// Env holds everything the checks read, so tests can fake it.
type Env struct {
	Look        tools.LookPath
	TmuxVersion func() (string, error) // output of `tmux -V`
	Home        string
	Shell       string // base name of $SHELL
	ZDotDir     string
	Path        []string // directories on $PATH
	Exe         string   // this binary's path
	Source      string   // checkout it was built from, "" if unknown
	Config      func() (config.Config, error)
	ConfigPath  string
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)`)

// Run performs every check.
func Run(e Env) []Check {
	cfg, cfgErr := e.Config()
	checks := []Check{
		e.tmux(),
		e.optional("git", "branch and change status in the preview"),
		e.iwashere(),
		e.editors(cfg.Editor),
		e.agents(),
		e.config(cfgErr),
		e.roots(cfg.Roots),
		e.hook(),
		e.onPath(),
	}
	if e.Source != "" {
		checks = append(checks, e.source())
	}
	return checks
}

// Failed reports whether any check failed outright.
func Failed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}

func (e Env) tmux() Check {
	c := Check{Name: "tmux"}
	if _, err := e.Look("tmux"); err != nil {
		c.Status, c.Detail, c.Hint = Fail, "not installed", "install tmux with your package manager"
		return c
	}
	out, err := e.TmuxVersion()
	m := versionRe.FindStringSubmatch(out)
	if err != nil || m == nil {
		c.Detail = "found"
		return c
	}
	c.Detail = strings.TrimSpace(out)
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 3 || (major == 3 && minor < 1) {
		c.Status, c.Hint = Warn, "the split layout needs tmux 3.1 or newer"
	}
	return c
}

func (e Env) optional(bin, feature string) Check {
	if _, err := e.Look(bin); err != nil {
		return Check{Name: bin, Status: Warn, Detail: "not installed", Hint: "needed for " + feature}
	}
	return Check{Name: bin, Detail: "found"}
}

func (e Env) iwashere() Check {
	if _, err := e.Look("iwashere"); err != nil {
		return Check{
			Name: "iwashere", Status: Warn, Detail: "not installed",
			Hint: "run `tmuxizer setup` to build it; sessions and time tracking are off without it",
		}
	}
	return Check{Name: "iwashere", Detail: "found, session tracking is on"}
}

func (e Env) editors(preferred string) Check {
	found := tools.Editors(e.Look, preferred)
	if len(found) == 0 {
		return Check{Name: "editors", Status: Warn, Detail: "none found", Hint: "install zed, cursor, code, nvim, helix or vim"}
	}
	return Check{Name: "editors", Detail: names(found)}
}

func (e Env) agents() Check {
	found := tools.Agents(e.Look)
	if len(found) == 0 {
		return Check{Name: "agents", Detail: "none installed (claude, codex, gemini, ... are detected automatically)"}
	}
	return Check{Name: "agents", Detail: names(found)}
}

func (e Env) config(err error) Check {
	path := tilde(e.ConfigPath, e.Home)
	switch {
	case err != nil:
		return Check{Name: "config", Status: Fail, Detail: err.Error(), Hint: "fix or delete " + path}
	case !fileExists(e.ConfigPath):
		return Check{Name: "config", Detail: "none, using defaults (`tmuxizer setup` writes a starter file)"}
	}
	return Check{Name: "config", Detail: path}
}

func (e Env) roots(roots []string) Check {
	var ok []string
	for _, r := range roots {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			ok = append(ok, tilde(r, e.Home))
		}
	}
	if len(ok) == 0 {
		return Check{
			Name: "project folders", Status: Warn, Detail: "none of the configured folders exist",
			Hint: "set `roots` in " + tilde(e.ConfigPath, e.Home),
		}
	}
	return Check{Name: "project folders", Detail: strings.Join(ok, ", ")}
}

func (e Env) hook() Check {
	rc, ok := setup.RCPath(e.Shell, e.Home, e.ZDotDir)
	switch {
	case !ok:
		return Check{Name: "shell hook", Detail: fmt.Sprintf("%q is not supported; use `tmuxizer init <bash|zsh|fish>`", e.Shell)}
	case !setup.HasHook(rc):
		return Check{
			Name: "shell hook", Status: Warn, Detail: "not in " + tilde(rc, e.Home),
			Hint: "run `tmuxizer setup` to open the picker in new terminals",
		}
	}
	return Check{Name: "shell hook", Detail: tilde(rc, e.Home)}
}

func (e Env) onPath() Check {
	dir := filepath.Dir(e.Exe)
	for _, p := range e.Path {
		if p == dir {
			return Check{Name: "PATH", Detail: tilde(dir, e.Home) + " is on PATH"}
		}
	}
	return Check{
		Name: "PATH", Status: Warn, Detail: tilde(dir, e.Home) + " is not on PATH",
		Hint: "add it to PATH so new terminals can run `tmuxizer`",
	}
}

func (e Env) source() Check {
	if _, err := os.Stat(filepath.Join(e.Source, "go.mod")); err != nil {
		return Check{
			Name: "update source", Status: Warn, Detail: tilde(e.Source, e.Home) + " is gone",
			Hint: "`tmuxizer update` needs the checkout it was built from; reinstall with `make install`",
		}
	}
	return Check{Name: "update source", Detail: tilde(e.Source, e.Home)}
}

func names(ts []tools.Tool) string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return strings.Join(out, ", ")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func tilde(p, home string) string {
	if home != "" {
		if rest, ok := strings.CutPrefix(p, home+"/"); ok {
			return "~/" + rest
		}
	}
	return p
}
