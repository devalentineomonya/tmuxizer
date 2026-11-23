// Package store persists tmuxizer's own state: recents, per-project launch
// choices, the project cache, and the sessions it started.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Prefs are the launch choices remembered for a project.
type Prefs struct {
	Saved  bool     `json:"saved"`  // false until the user has launched once
	Run    string   `json:"run"`    // command to start, "" for none
	Editor string   `json:"editor"` // editor name, "" for none
	Agents []string `json:"agents"` // agent names
	Track  bool     `json:"track"`  // record an iwashere session
}

// Session is a tmux session tmuxizer created.
type Session struct {
	Path    string    `json:"path"`
	Started time.Time `json:"started"`
	Prefs   Prefs     `json:"prefs"`
}

// CachedProject is a discovered project, kept for instant startup.
type CachedProject struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// State is the on-disk document.
type State struct {
	Opened   map[string]time.Time `json:"opened"`   // project path -> last launch
	Prefs    map[string]Prefs     `json:"prefs"`    // project path -> choices
	Sessions map[string]Session   `json:"sessions"` // tmux session name -> session
	Projects []CachedProject      `json:"projects"`
}

// Store reads and writes State under a lock so the TUI and the tmux
// session-closed hook can touch it at the same time.
type Store struct{ dir string }

// New returns a Store rooted at dir.
func New(dir string) *Store { return &Store{dir: dir} }

// Load returns the current state. A missing file is an empty state.
func (s *Store) Load() (State, error) {
	return s.read()
}

// Update applies fn to the latest state and saves the result.
func (s *Store) Update(fn func(*State)) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(s.dir, "state.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock state: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck

	st, err := s.read()
	if err != nil {
		return err
	}
	fn(&st)
	return s.write(st)
}

func (s *Store) path() string { return filepath.Join(s.dir, "state.json") }

func (s *Store) read() (State, error) {
	var st State
	raw, err := os.ReadFile(s.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return st, fmt.Errorf("read state: %w", err)
	default:
		if err := json.Unmarshal(raw, &st); err != nil {
			return State{}, fmt.Errorf("parse state: %w", err)
		}
	}
	st.init()
	return st, nil
}

// write replaces the file atomically so a crash never leaves it half-written.
func (s *Store) write(st State) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, "state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after the rename
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close state: %w", err)
	}
	return os.Rename(tmp.Name(), s.path())
}

func (st *State) init() {
	if st.Opened == nil {
		st.Opened = map[string]time.Time{}
	}
	if st.Prefs == nil {
		st.Prefs = map[string]Prefs{}
	}
	if st.Sessions == nil {
		st.Sessions = map[string]Session{}
	}
}
