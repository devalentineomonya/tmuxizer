// Package project finds the projects on disk and decides what counts as one.
package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devalentineomonya/tmuxizer/internal/stack"
)

// Project is a directory tmuxizer can open.
type Project struct {
	Name string
	Path string
}

// New builds a Project from its directory.
func New(path string) Project {
	return Project{Name: filepath.Base(path), Path: path}
}

// IsProject reports whether dir is a project root: a git repo or a
// directory holding a supported language's manifest.
func IsProject(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	return stack.Recognised(dir)
}

// Discover walks roots up to depth levels and returns every project found.
// It does not descend into a project, so nested packages are not listed.
func Discover(roots []string, depth int, ignore []string) []Project {
	skip := make(map[string]bool, len(ignore))
	for _, name := range ignore {
		skip[name] = true
	}

	seen := map[string]bool{}
	var found []Project
	var walk func(dir string, level int)
	walk = func(dir string, level int) {
		if IsProject(dir) {
			if !seen[dir] {
				seen[dir] = true
				found = append(found, New(dir))
			}
			return
		}
		if level >= depth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || strings.HasPrefix(name, ".") || skip[name] {
				continue
			}
			walk(filepath.Join(dir, name), level+1)
		}
	}
	for _, root := range roots {
		walk(filepath.Clean(root), 0)
	}
	return found
}

// Containing returns the project root enclosing dir, walking up the tree.
func Containing(dir string) (Project, bool) {
	home, _ := os.UserHomeDir()
	for {
		if dir == home {
			return Project{}, false // dotfile repos in $HOME are not projects
		}
		if IsProject(dir) {
			return New(dir), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Project{}, false
		}
		dir = parent
	}
}
