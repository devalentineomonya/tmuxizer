// Package config loads user settings from ~/.config/tmuxizer/config.toml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config holds user settings. Every field is optional.
type Config struct {
	Roots  []string `toml:"roots"`  // directories scanned for projects
	Depth  int      `toml:"depth"`  // how deep to scan below each root
	Ignore []string `toml:"ignore"` // directory names never entered
	Editor string   `toml:"editor"` // preferred editor binary
	Accent string   `toml:"accent"` // ANSI index (0-15), name, or #hex

	// SessionName is the iwashere session title template. Tokens:
	// {project} {branch} {date} {time}.
	SessionName string `toml:"session_name"`
}

var defaultRoots = []string{"workspace", "projects", "code", "dev", "src", "repos", "Projects"}

var defaultIgnore = []string{"node_modules", "vendor", "dist", "build", "target", "venv", "__pycache__"}

// Load reads the config file. A missing file yields defaults, not an error.
func Load() (Config, error) {
	cfg := Config{}
	raw, err := os.ReadFile(Path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return cfg.withDefaults(), fmt.Errorf("read config: %w", err)
	default:
		if err := toml.Unmarshal(raw, &cfg); err != nil {
			return Config{}.withDefaults(), fmt.Errorf("parse %s: %w", Path(), err)
		}
	}
	return cfg.withDefaults(), nil
}

// Path is the config file location.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(home(), ".config")
	}
	return filepath.Join(dir, "tmuxizer", "config.toml")
}

// StateDir is where tmuxizer keeps its runtime state.
func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "tmuxizer")
	}
	return filepath.Join(home(), ".local", "state", "tmuxizer")
}

func (c Config) withDefaults() Config {
	if c.Depth <= 0 {
		c.Depth = 3
	}
	if len(c.Roots) == 0 {
		c.Roots = DefaultRoots()
	}
	for i, r := range c.Roots {
		c.Roots[i] = expandHome(r)
	}
	c.Ignore = append(c.Ignore, defaultIgnore...)
	return c
}

// DefaultRoots returns the common project folders that exist under $HOME.
func DefaultRoots() []string {
	var roots []string
	for _, name := range defaultRoots {
		dir := filepath.Join(home(), name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}
	return roots
}

// Template returns a commented starter config that scans roots.
func Template(roots []string) string {
	quoted := make([]string, len(roots))
	for i, r := range roots {
		quoted[i] = strconv.Quote(tilde(r))
	}
	return `# tmuxizer config. Every key is optional.

# Folders scanned for projects (git repos and Go, TypeScript/JavaScript,
# Python and PHP projects).
roots = [` + strings.Join(quoted, ", ") + `]

# How many levels below each root to look.
# depth = 3

# Folder names to skip, added to node_modules, vendor, dist, ...
# ignore = ["archive"]

# Preferred editor. Defaults to $EDITOR, then zed, cursor, code, nvim, helix, vim.
# editor = "nvim"

# Accent colour: an ANSI name or index (0-15). Other colours follow your terminal theme.
# accent = "magenta"

# Title of the iwashere session. Tokens: {project} {branch} {date} {time}
# session_name = "{project} - {date} {time}"
`
}

// tilde shortens a path under $HOME to "~/...".
func tilde(p string) string {
	if rest, ok := strings.CutPrefix(p, home()+"/"); ok {
		return "~/" + rest
	}
	return p
}

// Write saves content to the config path unless a file is already there.
// It reports whether it wrote one.
func Write(content string) (bool, error) {
	path := Path()
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, nil
}

func expandHome(p string) string {
	if p == "~" {
		return home()
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home(), rest)
	}
	return p
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
