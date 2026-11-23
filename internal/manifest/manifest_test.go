package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadMissingIsDefault(t *testing.T) {
	m, err := Load(t.TempDir())
	if err != nil || m.Layout != Windows || m.Run != "" {
		t.Errorf("Load = %+v, %v", m, err)
	}
	if Exists(t.TempDir()) {
		t.Error("Exists should be false without a file")
	}
}

func TestLoadFull(t *testing.T) {
	dir := write(t, `
run = "pnpm dev"
editor = "nvim"
agents = ["claude"]
track = false
layout = "split"

[[windows]]
name = "logs"
command = "docker compose logs -f"

[[windows]]
name = "api"
command = "air"
dir = "apps/api"
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Run != "pnpm dev" || m.Editor != "nvim" || m.Layout != Split || len(m.Windows) != 2 {
		t.Errorf("manifest = %+v", m)
	}
	if m.Track == nil || *m.Track {
		t.Error("track = false must be distinguishable from unset")
	}
	if m.Windows[1].Dir != "apps/api" || !Exists(dir) {
		t.Errorf("windows = %+v", m.Windows)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := map[string]struct{ body, want string }{
		"unknown key":  {`runn = "x"`, "unknown key runn"},
		"bad layout":   {`layout = "grid"`, "layout"},
		"no name":      {"[[windows]]\ncommand = \"x\"", "needs a name"},
		"no command":   {"[[windows]]\nname = \"x\"", "needs a command"},
		"reserved":     {"[[windows]]\nname = \"term\"\ncommand = \"x\"", "reserved"},
		"duplicate":    {"[[windows]]\nname = \"a\"\ncommand = \"x\"\n[[windows]]\nname = \"a\"\ncommand = \"y\"", "twice"},
		"escaping dir": {"[[windows]]\nname = \"a\"\ncommand = \"x\"\ndir = \"../x\"", "inside the project"},
		"absolute dir": {"[[windows]]\nname = \"a\"\ncommand = \"x\"\ndir = \"/etc\"", "inside the project"},
		"syntax":       {"run = ", "line 1"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
