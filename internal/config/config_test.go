package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Depth != 3 {
		t.Errorf("depth = %d, want 3", cfg.Depth)
	}
	if len(cfg.Ignore) == 0 {
		t.Error("default ignore list missing")
	}
}

func TestLoadReadsFileAndExpandsHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", "/home/test")
	if err := os.MkdirAll(filepath.Join(dir, "tmuxizer"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "roots = [\"~/code\"]\ndepth = 5\neditor = \"nvim\"\naccent = \"blue\"\nignore = [\"tmp\"]\n"
	if err := os.WriteFile(Path(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Roots[0] != "/home/test/code" || cfg.Depth != 5 || cfg.Editor != "nvim" || cfg.Accent != "blue" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Ignore[0] != "tmp" {
		t.Errorf("user ignore entries should come first, got %v", cfg.Ignore)
	}
}

func TestLoadBrokenFileStillReturnsUsableConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_ = os.MkdirAll(filepath.Join(dir, "tmuxizer"), 0o755)
	_ = os.WriteFile(Path(), []byte("roots = ["), 0o644)

	cfg, err := Load()
	if err == nil {
		t.Error("a broken file should be reported")
	}
	if cfg.Depth != 3 {
		t.Error("defaults must still apply")
	}
}

func TestTemplateIsValidAndLoadable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", "/home/test")

	created, err := Write(Template([]string{"/home/test/code", "/srv/work"}))
	if err != nil || !created {
		t.Fatalf("Write = %v, %v", created, err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("the template must parse: %v", err)
	}
	if len(cfg.Roots) != 2 || cfg.Roots[0] != "/home/test/code" || cfg.Roots[1] != "/srv/work" {
		t.Errorf("roots = %v", cfg.Roots)
	}
	if raw, _ := os.ReadFile(Path()); !strings.Contains(string(raw), `"~/code"`) {
		t.Errorf("home paths should be written with ~:\n%s", raw)
	}
}

func TestWriteNeverOverwrites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = os.MkdirAll(filepath.Dir(Path()), 0o755)
	_ = os.WriteFile(Path(), []byte("depth = 9\n"), 0o644)

	created, err := Write("roots = []")
	if err != nil || created {
		t.Fatalf("Write = %v, %v", created, err)
	}
	if raw, _ := os.ReadFile(Path()); string(raw) != "depth = 9\n" {
		t.Errorf("existing config was changed: %q", raw)
	}
}
