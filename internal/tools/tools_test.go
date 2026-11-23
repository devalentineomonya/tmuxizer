package tools

import (
	"errors"
	"testing"
)

func lookIn(installed ...string) LookPath {
	return func(bin string) (string, error) {
		for _, i := range installed {
			if i == bin {
				return "/usr/bin/" + bin, nil
			}
		}
		return "", errors.New("not found")
	}
}

func names(ts []Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

func TestAgentsOnlyInstalled(t *testing.T) {
	got := names(Agents(lookIn("claude", "codex")))
	if len(got) != 2 || got[0] != "claude" || got[1] != "codex" {
		t.Errorf("agents = %v", got)
	}
	if len(Agents(lookIn())) != 0 {
		t.Error("no agents installed should yield none")
	}
}

func TestEditorsPreferredFirst(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	got := names(Editors(lookIn("zed", "nvim", "vim"), "nvim"))
	if got[0] != "nvim" || len(got) != 3 {
		t.Errorf("editors = %v, want nvim first", got)
	}
	got = names(Editors(lookIn("zed", "nvim"), ""))
	if got[0] != "zed" {
		t.Errorf("default order should lead with zed, got %v", got)
	}
}

func TestEditorsHonourEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "/usr/bin/vim")
	if got := Editors(lookIn("zed", "vim"), ""); got[0].Name != "vim" {
		t.Errorf("EDITOR should win, got %v", names(got))
	}
}

func TestCommand(t *testing.T) {
	if got := (Tool{Bin: "claude"}).Command(); got != "claude" {
		t.Errorf("got %q", got)
	}
	if got := (Tool{Bin: "nvim", Args: "."}).Command(); got != "nvim ." {
		t.Errorf("got %q", got)
	}
}
