// Package setup installs tmuxizer's shell hook and starter config.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	markBegin = "# >>> tmuxizer >>>"
	markEnd   = "# <<< tmuxizer <<<"
)

// Snippets open the picker in new interactive terminals, but not inside
// tmux (where the picker is run on demand) and not when output is piped.
var snippets = map[string]string{
	"bash": `if [[ $- == *i* && -z "$TMUX" && -t 1 ]] && command -v tmuxizer >/dev/null; then
  tmuxizer
fi`,
	"zsh": `if [[ -o interactive && -z "$TMUX" && -t 1 ]] && (( $+commands[tmuxizer] )); then
  tmuxizer
fi`,
	"fish": `if status is-interactive; and not set -q TMUX; and isatty stdout; and type -q tmuxizer
    tmuxizer
end`,
}

// Snippet returns the shell code that opens the picker.
func Snippet(shell string) (string, bool) {
	s, ok := snippets[shell]
	return s, ok
}

// Shells lists the supported shells.
func Shells() []string { return []string{"bash", "zsh", "fish"} }

// RCPath returns the startup file the hook is added to.
func RCPath(shell, home, zdotdir string) (string, bool) {
	switch shell {
	case "zsh":
		if zdotdir != "" {
			return filepath.Join(zdotdir, ".zshrc"), true
		}
		return filepath.Join(home, ".zshrc"), true
	case "bash":
		return filepath.Join(home, ".bashrc"), true
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), true
	}
	return "", false
}

// HasBlock reports whether content already carries the tmuxizer block.
func HasBlock(content string) bool { return strings.Contains(content, markBegin) }

// HasHook reports whether the startup file at path has the block.
func HasHook(path string) bool {
	raw, err := os.ReadFile(path)
	return err == nil && HasBlock(string(raw))
}

// AddBlock appends the marked block to content, once.
func AddBlock(content, snippet string) string {
	if HasBlock(content) {
		return content
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + markBegin + "\n" + snippet + "\n" + markEnd + "\n"
}

// RemoveBlock deletes the marked block and the blank line AddBlock put
// before it, leaving the rest untouched.
func RemoveBlock(content string) string {
	start := strings.Index(content, markBegin)
	if start < 0 {
		return content
	}
	rel := strings.Index(content[start:], markEnd)
	if rel < 0 {
		return content // unbalanced markers: do not guess
	}
	after := strings.TrimPrefix(content[start+rel+len(markEnd):], "\n")
	before := content[:start]
	if strings.HasSuffix(before, "\n\n") {
		before = before[:len(before)-1] // the spacer AddBlock inserted
	}
	return before + after
}

// InstallHook adds the block to the startup file at path, creating the file
// if needed. It reports whether anything changed.
func InstallHook(path, snippet string) (bool, error) {
	return rewrite(path, func(c string) string { return AddBlock(c, snippet) })
}

// RemoveHook deletes the block from the startup file at path.
func RemoveHook(path string) (bool, error) {
	return rewrite(path, RemoveBlock)
}

// rewrite applies fn to the file and saves it atomically, keeping its mode
// and any symlink it was reached through.
func rewrite(path string, fn func(string) string) (bool, error) {
	// Dotfile managers symlink rc files; write through the link, not over it.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	before := string(raw)
	after := fn(before)
	if after == before {
		return false, nil
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmuxizer-*")
	if err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after the rename
	if _, err := tmp.WriteString(after); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return false, fmt.Errorf("replace %s: %w", path, err)
	}
	return true, nil
}
