package setup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeEnv struct {
	home     string
	missing  map[string]bool
	runs     []string
	cfgWrote bool
	cfgBody  string
}

func (f *fakeEnv) options(input string, yes bool) (Options, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return Options{
		In:         strings.NewReader(input),
		Out:        out,
		Yes:        yes,
		Shell:      "zsh",
		Home:       f.home,
		InstallDir: filepath.Join(f.home, ".local", "bin"),
		Roots:      []string{filepath.Join(f.home, "code")},
		Look: func(bin string) (string, error) {
			if f.missing[bin] {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + bin, nil
		},
		Run: func(_, name string, args ...string) error {
			f.runs = append(f.runs, name+" "+strings.Join(args, " "))
			if name == "git" { // fake clone: create the repo layout
				_ = os.MkdirAll(filepath.Join(args[len(args)-1], "cmd"), 0o755)
				_ = os.WriteFile(filepath.Join(args[len(args)-1], "cmd", "main.go"), nil, 0o644)
			}
			return nil
		},
		Template: func(roots []string) string { return "roots = " + strings.Join(roots, ",") },
		WriteConfig: func(body string) (bool, error) {
			f.cfgBody = body
			return !f.cfgWrote, nil
		},
	}, out
}

func TestSetupInstallsHookWhenConfirmed(t *testing.T) {
	f := &fakeEnv{home: t.TempDir()}
	o, out := f.options("y\n", false)
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !HasHook(filepath.Join(f.home, ".zshrc")) {
		t.Errorf("hook missing:\n%s", out)
	}
	if !strings.Contains(f.cfgBody, "code") {
		t.Errorf("template did not get the roots: %q", f.cfgBody)
	}
}

func TestSetupSkipsWhenDeclinedOrNoInput(t *testing.T) {
	for _, input := range []string{"n\n", ""} {
		f := &fakeEnv{home: t.TempDir()}
		o, _ := f.options(input, false)
		if err := Run(o); err != nil {
			t.Fatal(err)
		}
		if HasHook(filepath.Join(f.home, ".zshrc")) {
			t.Errorf("input %q must not install the hook", input)
		}
	}
}

func TestSetupYesAnswersEverything(t *testing.T) {
	f := &fakeEnv{home: t.TempDir(), missing: map[string]bool{"iwashere": true}}
	o, _ := f.options("", true)
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !HasHook(filepath.Join(f.home, ".zshrc")) {
		t.Error("--yes should install the hook")
	}
	if len(f.runs) != 2 || !strings.HasPrefix(f.runs[0], "git clone --depth 1 https://github.com/Murchoid/iwashere") ||
		!strings.HasPrefix(f.runs[1], "go build -o "+filepath.Join(f.home, ".local", "bin", "iwashere")+" ./cmd") {
		t.Errorf("runs = %v", f.runs)
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	f := &fakeEnv{home: t.TempDir(), cfgWrote: true}
	o, _ := f.options("", true)
	_ = Run(o)
	first, _ := os.ReadFile(filepath.Join(f.home, ".zshrc"))

	o, out := f.options("", true)
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(f.home, ".zshrc"))
	if string(first) != string(second) {
		t.Error("a second run must not change the shell file")
	}
	if !strings.Contains(out.String(), "already in") {
		t.Errorf("should say the hook exists:\n%s", out)
	}
}

func TestSetupFailsWithoutTmux(t *testing.T) {
	f := &fakeEnv{home: t.TempDir(), missing: map[string]bool{"tmux": true}}
	o, out := f.options("", true)
	if err := Run(o); err == nil || !strings.Contains(out.String(), "tmux is not installed") {
		t.Errorf("err = %v, out:\n%s", err, out)
	}
}

func TestSetupReportsIwashereBuildFailure(t *testing.T) {
	f := &fakeEnv{home: t.TempDir(), missing: map[string]bool{"iwashere": true, "go": true}}
	o, out := f.options("", true)
	if err := Run(o); err != nil {
		t.Fatalf("a failed optional install must not fail setup: %v", err)
	}
	if !strings.Contains(out.String(), "go is required") || !strings.Contains(out.String(), "manual steps") {
		t.Errorf("missing guidance:\n%s", out)
	}
}

func TestSetupRemove(t *testing.T) {
	f := &fakeEnv{home: t.TempDir()}
	o, _ := f.options("", true)
	_ = Run(o)

	o, out := f.options("", true)
	o.Remove = true
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if HasHook(filepath.Join(f.home, ".zshrc")) || !strings.Contains(out.String(), "removed the hook") {
		t.Errorf("hook still present:\n%s", out)
	}
}

func TestMainPackageLayouts(t *testing.T) {
	for _, layout := range []struct{ file, want string }{
		{"cmd/iwashere/main.go", "./cmd/iwashere"},
		{"cmd/main.go", "./cmd"},
		{"main.go", "."},
	} {
		dir := t.TempDir()
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, layout.file)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, layout.file), nil, 0o644)
		if got, err := mainPackage(dir); err != nil || got != layout.want {
			t.Errorf("%s: got %q, %v", layout.file, got, err)
		}
	}
	if _, err := mainPackage(t.TempDir()); err == nil {
		t.Error("an empty repo has no main package")
	}
}
