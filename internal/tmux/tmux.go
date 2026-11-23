// Package tmux is a thin wrapper over the tmux command line.
package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Runner executes a tmux command and returns its combined output.
type Runner func(args ...string) (string, error)

// Client talks to the tmux server.
type Client struct{ run Runner }

// New returns a Client that shells out to the tmux binary.
func New() *Client { return &Client{run: execRunner} }

// NewWithRunner returns a Client using run, for tests.
func NewWithRunner(run Runner) *Client { return &Client{run: run} }

func execRunner(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("tmux %s: %w: %s", args[0], err, text)
	}
	return text, nil
}

// Available reports whether the tmux binary is installed.
func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// Inside reports whether the current process runs inside tmux.
func Inside() bool { return os.Getenv("TMUX") != "" }

// exact makes tmux match a session name literally instead of by prefix.
func exact(name string) string { return "=" + name }

// HasSession reports whether a session with this exact name is alive.
func (c *Client) HasSession(name string) bool {
	_, err := c.run("has-session", "-t", exact(name))
	return err == nil
}

// SessionPath returns the start directory of a live session.
func (c *Client) SessionPath(name string) (string, error) {
	return c.run("display-message", "-p", "-t", exact(name)+":", "#{session_path}")
}

// NewSession creates a detached session whose first window is named window.
func (c *Client) NewSession(name, dir, window string) error {
	_, err := c.run("new-session", "-d", "-s", name, "-c", dir, "-n", window)
	return err
}

// KillSession destroys a session, ignoring one that is already gone.
func (c *Client) KillSession(name string) error {
	_, err := c.run("kill-session", "-t", exact(name))
	return err
}

// NewWindow adds a window running command (a shell string) in dir.
func (c *Client) NewWindow(session, dir, name, command string) error {
	args := []string{"new-window", "-d", "-t", exact(session) + ":", "-c", dir, "-n", name}
	if command != "" {
		args = append(args, command)
	}
	_, err := c.run(args...)
	return err
}

// Split adds a side pane to a window without taking focus. percent is the
// share of the window the new pane gets.
func (c *Client) Split(session, window, dir, command string, percent int) error {
	_, err := c.run("split-window", "-d", "-h", "-l", fmt.Sprintf("%d%%", percent),
		"-t", exact(session)+":"+window, "-c", dir, command)
	return err
}

// SelectWindow focuses a window by name.
func (c *Client) SelectWindow(session, window string) error {
	_, err := c.run("select-window", "-t", exact(session)+":"+window)
	return err
}

// SetHook installs a global hook at a private array index so it never
// replaces hooks the user configured themselves.
func (c *Client) SetHook(event string, index int, command string) error {
	_, err := c.run("set-hook", "-g", fmt.Sprintf("%s[%d]", event, index), command)
	return err
}

// Enter attaches to the session, or switches to it when already inside tmux.
// Outside tmux it replaces the current process and only returns on error.
func (c *Client) Enter(name string) error {
	if Inside() {
		_, err := c.run("switch-client", "-t", exact(name))
		return err
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("tmux not found in PATH")
	}
	return syscall.Exec(bin, []string{"tmux", "attach-session", "-t", exact(name)}, os.Environ())
}

// KeepOpen wraps command so the window falls back to a shell when it exits,
// letting the user read the output instead of losing the window.
func KeepOpen(command string) string {
	return command + `; exec "${SHELL:-sh}"`
}
