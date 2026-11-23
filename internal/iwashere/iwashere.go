// Package iwashere integrates with the iwashere CLI
// (https://github.com/Murchoid/iwashere) to record work sessions.
package iwashere

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	dirName   = ".iwashere"
	bin       = "iwashere"
	ongoing   = "ongoing"
	paused    = "paused"
	continued = "continued"
)

// Session mirrors the fields of iwashere's session files that we read.
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Branch    string    `json:"branch"`
	TotalTime Duration  `json:"total_time"`
}

// Open reports whether the session can still be worked on.
func (s Session) Open() bool {
	switch s.State {
	case ongoing, paused, continued:
		return true
	}
	return false
}

// Client drives the iwashere binary.
type Client struct {
	run func(dir string, args ...string) error
}

// New returns a Client that shells out to iwashere.
func New() *Client { return &Client{run: execRun} }

// NewWithRun returns a Client with a custom runner, for tests.
func NewWithRun(run func(dir string, args ...string) error) *Client { return &Client{run: run} }

func execRun(dir string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iwashere %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Available reports whether the iwashere binary is installed.
func Available() bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// Initialized reports whether dir, or a parent, already has an iwashere store.
func Initialized(dir string) bool {
	for {
		if info, err := os.Stat(filepath.Join(dir, dirName)); err == nil && info.IsDir() {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// Init creates the iwashere store in dir. It never forces over an existing one.
func (c *Client) Init(dir string) error {
	if Initialized(dir) {
		return nil
	}
	return c.run(dir, "init")
}

// Start begins a session unless one is already open.
func (c *Client) Start(dir, title string) error {
	if _, ok := CurrentSession(dir); ok {
		return nil
	}
	return c.run(dir, "session", "start", title)
}

// Continue resumes a paused session; other states are left alone.
func (c *Client) Continue(dir string) error {
	if s, ok := CurrentSession(dir); ok && s.State == paused {
		return c.run(dir, "session", "continue")
	}
	return nil
}

// End closes the open session, if any.
func (c *Client) End(dir string) error {
	if _, ok := CurrentSession(dir); !ok {
		return nil
	}
	return c.run(dir, "session", "end")
}

// CurrentSession returns the newest open session recorded in dir.
func CurrentSession(dir string) (Session, bool) {
	var latest Session
	found := false
	for _, s := range Sessions(dir) {
		if s.Open() && (!found || s.StartTime.After(latest.StartTime)) {
			latest, found = s, true
		}
	}
	return latest, found
}
