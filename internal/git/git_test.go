package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	out := []byte("# branch.oid abc\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n" +
		"1 .M N... 100644 100644 100644 a b file.go\n" + // modified
		"1 M. N... 100644 100644 100644 a b staged.go\n" + // staged
		"1 MM N... 100644 100644 100644 a b both.go\n" + // staged and modified
		"u UU N... 100644 100644 100644 100644 a b c conflict.go\n" +
		"? new.txt\n")
	got := parse(out)
	want := Info{Repo: true, Branch: "main", Changed: 5, Staged: 2, Modified: 2, Untracked: 1, Conflicts: 1, Ahead: 2, Behind: 1}
	if got != want {
		t.Errorf("parse = %+v, want %+v", got, want)
	}
	if got.Clean() {
		t.Error("dirty tree reported clean")
	}
}

func TestParseLog(t *testing.T) {
	when, subject := parseLog([]byte("1700000000\x00fix: thing\n"))
	if subject != "fix: thing" || when.Unix() != 1700000000 {
		t.Errorf("got %v %q", when, subject)
	}
	if _, s := parseLog([]byte("garbage")); s != "" {
		t.Errorf("garbage should yield nothing, got %q", s)
	}
}

func TestStatusOutsideRepo(t *testing.T) {
	if Status(t.TempDir()).Repo {
		t.Error("temp dir is not a repository")
	}
}

func TestStatusInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "first commit")
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	info := Status(dir)
	if !info.Repo || info.Branch != "main" || info.Untracked != 1 || info.Subject != "first commit" || info.When.IsZero() {
		t.Errorf("info = %+v", info)
	}
}
