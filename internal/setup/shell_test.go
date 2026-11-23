package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockRoundTrip(t *testing.T) {
	original := "export A=1\nalias ll='ls -l'\n"
	added := AddBlock(original, "tmuxizer")
	if !HasBlock(added) || !strings.HasPrefix(added, original) {
		t.Fatalf("block not appended cleanly:\n%s", added)
	}
	if AddBlock(added, "tmuxizer") != added {
		t.Error("adding twice must not duplicate the block")
	}
	if got := RemoveBlock(added); got != original {
		t.Errorf("remove should restore the file exactly:\n got %q\nwant %q", got, original)
	}
}

func TestBlockInEmptyAndUnterminatedFiles(t *testing.T) {
	if got := RemoveBlock(AddBlock("", "x")); got != "" {
		t.Errorf("empty file round trip = %q", got)
	}
	if got := AddBlock("no newline", "x"); !strings.HasPrefix(got, "no newline\n\n"+markBegin) {
		t.Errorf("got %q", got)
	}
	broken := "a\n" + markBegin + "\nno end marker\n"
	if RemoveBlock(broken) != broken {
		t.Error("unbalanced markers must be left alone")
	}
}

func TestRemoveKeepsContentAfterBlock(t *testing.T) {
	c := "before\n" + markBegin + "\nx\n" + markEnd + "\nafter\n"
	if got := RemoveBlock(c); got != "before\nafter\n" {
		t.Errorf("got %q", got)
	}
}

func TestInstallAndRemoveHookOnDisk(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(rc, []byte("export A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := InstallHook(rc, "tmuxizer")
	if err != nil || !changed || !HasHook(rc) {
		t.Fatalf("install = %v, %v", changed, err)
	}
	if changed, _ := InstallHook(rc, "tmuxizer"); changed {
		t.Error("second install should be a no-op")
	}
	if info, _ := os.Stat(rc); info.Mode().Perm() != 0o600 {
		t.Errorf("mode changed to %v", info.Mode().Perm())
	}
	if changed, _ := RemoveHook(rc); !changed || HasHook(rc) {
		t.Error("remove should delete the block")
	}
	if raw, _ := os.ReadFile(rc); string(raw) != "export A=1\n" {
		t.Errorf("rc not restored: %q", raw)
	}
}

func TestInstallWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "zshrc")
	link := filepath.Join(dir, ".zshrc")
	_ = os.MkdirAll(filepath.Dir(target), 0o755)
	_ = os.WriteFile(target, []byte("x\n"), 0o644)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallHook(link, "tmuxizer"); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink must survive")
	}
	if !HasHook(target) {
		t.Error("the block should land in the link's target")
	}
}

func TestInstallCreatesMissingFile(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".config", "fish", "config.fish")
	if changed, err := InstallHook(rc, "x"); err != nil || !changed || !HasHook(rc) {
		t.Errorf("got %v, %v", changed, err)
	}
}

func TestRCPathAndSnippet(t *testing.T) {
	if p, _ := RCPath("zsh", "/h", ""); p != "/h/.zshrc" {
		t.Errorf("zsh = %s", p)
	}
	if p, _ := RCPath("zsh", "/h", "/z"); p != "/z/.zshrc" {
		t.Errorf("ZDOTDIR = %s", p)
	}
	if p, _ := RCPath("fish", "/h", ""); p != "/h/.config/fish/config.fish" {
		t.Errorf("fish = %s", p)
	}
	if _, ok := RCPath("nu", "/h", ""); ok {
		t.Error("unsupported shell")
	}
	for _, sh := range Shells() {
		if s, ok := Snippet(sh); !ok || !strings.Contains(s, "tmuxizer") {
			t.Errorf("snippet for %s missing", sh)
		}
	}
}
