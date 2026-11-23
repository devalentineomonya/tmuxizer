package update

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fake struct {
	calls    []string
	replies  map[string]string // command prefix -> output
	failures map[string]bool
	renames  [][2]string
}

func (f *fake) run(_, name string, args ...string) (string, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	for prefix := range f.failures {
		if strings.HasPrefix(cmd, prefix) {
			return "", errors.New("failed")
		}
	}
	for prefix, out := range f.replies {
		if strings.HasPrefix(cmd, prefix) {
			return out, nil
		}
	}
	return "", nil
}

func (f *fake) updater(t *testing.T) (Updater, *bytes.Buffer) {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return Updater{
		Exe: "/bin/tmuxizer", Source: src, Module: "example.com/tmuxizer", Version: "v1",
		Out: out, Run: f.run,
		Rename: func(a, b string) error { f.renames = append(f.renames, [2]string{a, b}); return nil },
	}, out
}

func TestUpdateFromTrackedCheckout(t *testing.T) {
	f := &fake{replies: map[string]string{
		"git rev-parse": "origin/main\n",
		"git pull":      "Updating a..b\nFast-forward\n",
		"git describe":  "v2-3-gabc\n",
	}}
	u, out := f.updater(t)
	if err := u.Update(); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(f.calls, "\n")
	for _, want := range []string{"git pull --ff-only", "go build -ldflags -s -w -X main.version=v2-3-gabc -X main.source=" + u.Source + " -o /bin/tmuxizer.new ./cmd/tmuxizer"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	if len(f.renames) != 1 || f.renames[0] != [2]string{"/bin/tmuxizer.new", "/bin/tmuxizer"} {
		t.Errorf("renames = %v", f.renames)
	}
	if !strings.Contains(out.String(), "v1 → v2-3-gabc") {
		t.Errorf("output:\n%s", out)
	}
}

func TestUpdateWithoutRemoteJustRebuilds(t *testing.T) {
	f := &fake{failures: map[string]bool{"git rev-parse": true}}
	u, out := f.updater(t)
	if err := u.Update(); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "git pull") {
			t.Error("must not pull without an upstream")
		}
	}
	if !strings.Contains(out.String(), "no remote branch") {
		t.Errorf("output:\n%s", out)
	}
}

func TestFailedPullOrBuildKeepsOldBinary(t *testing.T) {
	for _, fail := range []string{"git pull", "go build"} {
		f := &fake{replies: map[string]string{"git rev-parse": "origin/main"}, failures: map[string]bool{fail: true}}
		u, _ := f.updater(t)
		if err := u.Update(); err == nil {
			t.Errorf("%s failure should be reported", fail)
		}
		if len(f.renames) != 0 {
			t.Errorf("%s failure must not replace the binary", fail)
		}
	}
}

func TestMissingSourceExplainsFix(t *testing.T) {
	f := &fake{}
	u, _ := f.updater(t)
	u.Source = filepath.Join(t.TempDir(), "gone")
	if err := u.Update(); err == nil || !strings.Contains(err.Error(), "make install") {
		t.Errorf("err = %v", err)
	}
}

func TestGoInstallWithoutSource(t *testing.T) {
	f := &fake{}
	u, _ := f.updater(t)
	u.Source = ""
	if err := u.Update(); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0] != "go install example.com/tmuxizer/cmd/tmuxizer@latest" {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestCheckCountsCommitsBehind(t *testing.T) {
	f := &fake{replies: map[string]string{"git rev-list": "3\n"}}
	u, _ := f.updater(t)
	if n, err := u.Check(); err != nil || n != 3 {
		t.Errorf("behind = %d, %v", n, err)
	}
	u.Source = ""
	if _, err := u.Check(); err == nil {
		t.Error("no checkout means nothing to check")
	}
}
