package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devalentineomonya/tmuxizer/internal/config"
)

func env(t *testing.T, missing ...string) Env {
	t.Helper()
	home := t.TempDir()
	gone := map[string]bool{}
	for _, m := range missing {
		gone[m] = true
	}
	return Env{
		Look: func(bin string) (string, error) {
			if gone[bin] {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + bin, nil
		},
		TmuxVersion: func() (string, error) { return "tmux 3.7b", nil },
		Home:        home,
		Shell:       "zsh",
		Path:        []string{"/usr/bin"},
		Exe:         "/usr/bin/tmuxizer",
		ConfigPath:  filepath.Join(home, "config.toml"),
		Config:      func() (config.Config, error) { return config.Config{Roots: []string{home}}, nil },
	}
}

func find(checks []Check, name string) Check {
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	return Check{Name: "<missing " + name + ">"}
}

func TestHealthySetupHasNoFailures(t *testing.T) {
	e := env(t)
	if err := os.WriteFile(filepath.Join(e.Home, ".zshrc"), []byte("# >>> tmuxizer >>>\nx\n# <<< tmuxizer <<<\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := Run(e)
	if Failed(checks) {
		t.Errorf("unexpected failure: %+v", checks)
	}
	for _, name := range []string{"tmux", "iwashere", "shell hook", "PATH"} {
		if c := find(checks, name); c.Status != OK {
			t.Errorf("%s = %+v, want OK", name, c)
		}
	}
}

func TestMissingTmuxFails(t *testing.T) {
	checks := Run(env(t, "tmux"))
	if c := find(checks, "tmux"); c.Status != Fail || c.Hint == "" || !Failed(checks) {
		t.Errorf("tmux = %+v", c)
	}
}

func TestOldTmuxWarns(t *testing.T) {
	e := env(t)
	e.TmuxVersion = func() (string, error) { return "tmux 3.0a", nil }
	if c := find(Run(e), "tmux"); c.Status != Warn || !strings.Contains(c.Hint, "3.1") {
		t.Errorf("tmux = %+v", c)
	}
	e.TmuxVersion = func() (string, error) { return "tmux next-3.5", nil }
	if c := find(Run(e), "tmux"); c.Status != OK {
		t.Errorf("next-3.5 should pass, got %+v", c)
	}
}

func TestOptionalToolsWarnNotFail(t *testing.T) {
	checks := Run(env(t, "iwashere", "git", "zed", "nvim", "vim", "cursor", "code", "hx"))
	if Failed(checks) {
		t.Error("optional tools must never fail the run")
	}
	for _, name := range []string{"iwashere", "git", "editors"} {
		if c := find(checks, name); c.Status != Warn {
			t.Errorf("%s = %+v, want Warn", name, c)
		}
	}
}

func TestBrokenConfigFails(t *testing.T) {
	e := env(t)
	e.Config = func() (config.Config, error) { return config.Config{}, errors.New("parse error") }
	if c := find(Run(e), "config"); c.Status != Fail || !strings.Contains(c.Detail, "parse error") {
		t.Errorf("config = %+v", c)
	}
}

func TestHookAndPathWarnings(t *testing.T) {
	e := env(t)
	e.Path = []string{"/somewhere/else"}
	checks := Run(e)
	if find(checks, "shell hook").Status != Warn {
		t.Error("missing hook should warn")
	}
	if find(checks, "PATH").Status != Warn {
		t.Error("exe off PATH should warn")
	}
}

func TestMissingRootsWarn(t *testing.T) {
	e := env(t)
	e.Config = func() (config.Config, error) { return config.Config{Roots: []string{"/nope/nothing"}}, nil }
	if c := find(Run(e), "project folders"); c.Status != Warn {
		t.Errorf("roots = %+v", c)
	}
}

func TestSourceCheckOnlyWhenKnown(t *testing.T) {
	e := env(t)
	if find(Run(e), "update source").Name != "<missing update source>" {
		t.Error("no source recorded, no check")
	}
	e.Source = filepath.Join(e.Home, "gone")
	if c := find(Run(e), "update source"); c.Status != Warn {
		t.Errorf("source = %+v", c)
	}
}
