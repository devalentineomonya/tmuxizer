// Package manifest reads a project's optional .tmuxizer.toml, which pins how
// tmuxizer opens that project.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// FileName is the manifest's name inside a project.
const FileName = ".tmuxizer.toml"

// Layout controls where the app command runs.
type Layout string

const (
	// Windows runs the app in its own tmux window (default).
	Windows Layout = "windows"
	// Split runs the app in a side pane next to the shell.
	Split Layout = "split"
)

// Reserved window names belong to tmuxizer's own windows.
var reserved = map[string]bool{"term": true, "app": true}

// Window is an extra tmux window opened with the project.
type Window struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
	Dir     string `toml:"dir"` // relative to the project, optional
}

// Manifest holds the project's pinned choices. Empty fields are not pinned
// and fall back to remembered choices and detection.
type Manifest struct {
	Run     string   `toml:"run"`
	Editor  string   `toml:"editor"`
	Agents  []string `toml:"agents"`
	Track   *bool    `toml:"track"`
	Session string   `toml:"session"` // iwashere session title template
	Layout  Layout   `toml:"layout"`
	Windows []Window `toml:"windows"`
}

// Exists reports whether dir has a manifest.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}

// Load reads dir's manifest. A missing file is not an error: it returns the
// zero Manifest with the default layout.
func Load(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{Layout: Windows}, nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", FileName, err)
	}

	var m Manifest
	dec := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", FileName, describe(err))
	}
	if m.Layout == "" {
		m.Layout = Windows
	}
	if err := m.validate(); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", FileName, err)
	}
	return m, nil
}

// describe turns go-toml's errors into one readable line.
func describe(err error) error {
	if strict, ok := errors.AsType[*toml.StrictMissingError](err); ok {
		keys := make([]string, 0, len(strict.Errors))
		for _, e := range strict.Errors {
			keys = append(keys, strings.Join(e.Key(), "."))
		}
		return fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
	}
	if de, ok := errors.AsType[*toml.DecodeError](err); ok {
		row, col := de.Position()
		return fmt.Errorf("line %d, column %d: %s", row, col, de.Error())
	}
	return err
}

func (m Manifest) validate() error {
	if m.Layout != Windows && m.Layout != Split {
		return fmt.Errorf("layout %q must be %q or %q", m.Layout, Windows, Split)
	}
	seen := map[string]bool{}
	for i, w := range m.Windows {
		switch {
		case strings.TrimSpace(w.Name) == "":
			return fmt.Errorf("windows[%d] needs a name", i)
		case strings.TrimSpace(w.Command) == "":
			return fmt.Errorf("window %q needs a command", w.Name)
		case reserved[w.Name]:
			return fmt.Errorf("window name %q is reserved", w.Name)
		case seen[w.Name]:
			return fmt.Errorf("window name %q is used twice", w.Name)
		case w.Dir != "" && !filepath.IsLocal(w.Dir):
			return fmt.Errorf("window %q: dir must stay inside the project", w.Name)
		}
		seen[w.Name] = true
	}
	return nil
}
