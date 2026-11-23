// Package tools detects the editors and AI agents installed on this machine.
package tools

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Kind says how a tool is launched.
type Kind int

const (
	// Terminal tools run inside a tmux window.
	Terminal Kind = iota
	// Window tools open their own GUI window and return immediately.
	Window
)

// Tool is an installed editor or agent.
type Tool struct {
	Name string // display name and stable identifier
	Bin  string // executable name
	Args string // arguments appended to Bin
	Kind Kind
}

// Command returns the shell command that launches the tool in its cwd.
func (t Tool) Command() string {
	if t.Args == "" {
		return t.Bin
	}
	return t.Bin + " " + t.Args
}

var (
	knownEditors = []Tool{
		{"zed", "zed", ".", Window},
		{"cursor", "cursor", ".", Window},
		{"code", "code", ".", Window},
		{"nvim", "nvim", ".", Terminal},
		{"helix", "hx", ".", Terminal},
		{"vim", "vim", ".", Terminal},
	}
	knownAgents = []Tool{
		{"claude", "claude", "", Terminal},
		{"antigravity", "antigravity", ".", Window},
		{"gemini", "gemini", "", Terminal},
		{"codex", "codex", "", Terminal},
		{"opencode", "opencode", "", Terminal},
		{"aider", "aider", "", Terminal},
	}
)

// LookPath finds an executable; it matches exec.LookPath for injection in tests.
type LookPath func(file string) (string, error)

// Editors returns installed editors. preferred, then $VISUAL and $EDITOR,
// are moved to the front so the first entry is the best default.
func Editors(look LookPath, preferred string) []Tool {
	found := installed(look, knownEditors)
	for _, want := range []string{filepath.Base(os.Getenv("EDITOR")), filepath.Base(os.Getenv("VISUAL")), preferred} {
		found = promote(found, want)
	}
	return found
}

// Agents returns installed AI agents in a fixed order.
func Agents(look LookPath) []Tool {
	return installed(look, knownAgents)
}

// Default uses the real PATH.
func Default() LookPath { return exec.LookPath }

func installed(look LookPath, candidates []Tool) []Tool {
	var out []Tool
	for _, t := range candidates {
		if _, err := look(t.Bin); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// promote moves the tool matching name or binary to the front.
func promote(list []Tool, name string) []Tool {
	for i, t := range list {
		if t.Name == name || t.Bin == name {
			return append([]Tool{t}, append(list[:i:i], list[i+1:]...)...)
		}
	}
	return list
}

// ByName finds a tool in list.
func ByName(list []Tool, name string) (Tool, bool) {
	for _, t := range list {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
