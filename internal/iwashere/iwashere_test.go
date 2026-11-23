package iwashere

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSession stores a session file the way iwashere does.
func writeSession(t *testing.T, dir, id, state, start string) {
	t.Helper()
	sessions := filepath.Join(dir, dirName, "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"` + id + `","title":"t-` + id + `","state":"` + state + `","start_time":"` + start + `"}`
	if err := os.WriteFile(filepath.Join(sessions, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type recorder struct{ calls []string }

func (r *recorder) client() *Client {
	return NewWithRun(func(_ string, args ...string) error {
		r.calls = append(r.calls, strings.Join(args, " "))
		return nil
	})
}

func TestCurrentSessionPicksNewestOpen(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "old", "ongoing", "2026-01-01T10:00:00Z")
	writeSession(t, dir, "new", "paused", "2026-01-02T10:00:00Z")
	writeSession(t, dir, "done", "ended", "2026-01-03T10:00:00Z")

	s, ok := CurrentSession(dir)
	if !ok || s.ID != "new" {
		t.Errorf("current = %+v, %v; want id new", s, ok)
	}
}

func TestCurrentSessionNoneOpen(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "done", "ended", "2026-01-03T10:00:00Z")
	if _, ok := CurrentSession(dir); ok {
		t.Error("ended sessions are not current")
	}
	if _, ok := CurrentSession(t.TempDir()); ok {
		t.Error("no store means no session")
	}
}

func TestStartIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	var r recorder
	c := r.client()

	_ = c.Start(dir, "work")
	if len(r.calls) != 1 || r.calls[0] != "session start work" {
		t.Fatalf("calls = %v", r.calls)
	}

	writeSession(t, dir, "a", "ongoing", "2026-01-01T10:00:00Z")
	_ = c.Start(dir, "again")
	if len(r.calls) != 1 {
		t.Errorf("Start must not stack sessions, calls = %v", r.calls)
	}
}

func TestEndAndContinueOnlyActWhenNeeded(t *testing.T) {
	dir := t.TempDir()
	var r recorder
	c := r.client()

	_ = c.End(dir)
	_ = c.Continue(dir)
	if len(r.calls) != 0 {
		t.Fatalf("nothing open, yet calls = %v", r.calls)
	}

	writeSession(t, dir, "a", "paused", "2026-01-01T10:00:00Z")
	_ = c.Continue(dir)
	_ = c.End(dir)
	if strings.Join(r.calls, "|") != "session continue|session end" {
		t.Errorf("calls = %v", r.calls)
	}
}

func TestInitSkipsWhenParentInitialized(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "pkg")
	_ = os.MkdirAll(child, 0o755)

	var r recorder
	_ = r.client().Init(child)
	if len(r.calls) != 0 {
		t.Errorf("init should be skipped, calls = %v", r.calls)
	}
	_ = r.client().Init(t.TempDir())
	if len(r.calls) != 1 {
		t.Errorf("init should run in a fresh dir, calls = %v", r.calls)
	}
}

func TestLastNotePicksNewestFile(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(dir, dirName, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, msg string, age time.Duration) {
		path := filepath.Join(notes, name)
		if err := os.WriteFile(path, []byte(`{"message":"`+msg+`","created_at":"2026-01-01T10:00:00Z"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(-age)
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", "older", 2*time.Hour)
	write("b.json", "newer", time.Minute)

	n, ok := LastNote(dir)
	if !ok || n.Message != "newer" {
		t.Errorf("LastNote = %+v, %v", n, ok)
	}
	if _, ok := LastNote(t.TempDir()); ok {
		t.Error("no notes should report none")
	}
}
