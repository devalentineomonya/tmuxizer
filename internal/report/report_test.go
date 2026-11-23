package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devalentineomonya/tmuxizer/internal/project"
)

// Wednesday 25 Nov 2026, 15:00 UTC.
var now = time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)

func session(t *testing.T, dir, id, state, total, start, end string) {
	t.Helper()
	sessions := filepath.Join(dir, ".iwashere", "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"` + id + `","state":"` + state + `","total_time":"` + total + `","start_time":"` + start + `","end_time":"` + end + `"}`
	if err := os.WriteFile(filepath.Join(sessions, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	dir := t.TempDir()
	zero := "0001-01-01T00:00:00Z"
	session(t, dir, "today", "ended", "1h0m0s", "2026-11-25T09:00:00Z", "2026-11-25T10:00:00Z")
	session(t, dir, "monday", "ended", "2h0m0s", "2026-11-23T08:00:00Z", "2026-11-23T10:00:00Z")
	session(t, dir, "friday", "ended", "3h0m0s", "2026-11-20T08:00:00Z", "2026-11-20T11:00:00Z")
	session(t, dir, "running", "ongoing", "0s", "2026-11-25T14:00:00Z", zero)
	return dir
}

func TestSummarizeSplitsPeriods(t *testing.T) {
	got := Summarize(fixture(t), now)
	want := Summary{Today: 2 * time.Hour, Week: 4 * time.Hour, All: 7 * time.Hour, Sessions: 4}
	if got != want {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize(t.TempDir(), now); got != (Summary{}) {
		t.Errorf("summary = %+v, want zero", got)
	}
}

func TestPausedAndContinuedElapsed(t *testing.T) {
	dir := t.TempDir()
	session(t, dir, "paused", "paused", "30m0s", "2026-11-25T09:00:00Z", "2026-11-25T09:30:00Z")
	session(t, dir, "cont", "continued", "30m0s", "2026-11-25T14:30:00Z", "2026-11-25T10:00:00Z")
	// paused: 30m. continued: 30m banked + 30m since restart.
	if got := Summarize(dir, now).All; got != 90*time.Minute {
		t.Errorf("all = %v, want 1h30m", got)
	}
}

func TestPeriodSince(t *testing.T) {
	if got := Today.Since(now); !got.Equal(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("today = %v", got)
	}
	if got := Week.Since(now); !got.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("week = %v, want Monday", got)
	}
	sunday := time.Date(2026, 11, 29, 12, 0, 0, 0, time.UTC)
	if got := Week.Since(sunday); !got.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("sunday's week = %v, want the Monday before", got)
	}
	if !All.Since(now).IsZero() {
		t.Error("all time has no start")
	}
}

func TestBuildSortsAndTotals(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	zero := "0001-01-01T00:00:00Z"
	session(t, a, "a", "ended", "1h0m0s", "2026-11-25T09:00:00Z", "2026-11-25T10:00:00Z")
	session(t, b, "b", "ended", "3h0m0s", "2026-11-25T09:00:00Z", "2026-11-25T12:00:00Z")
	_ = zero
	rep := Build([]project.Project{project.New(a), project.New(b), project.New(t.TempDir())}, Today, now)

	if len(rep.Rows) != 2 || rep.Rows[0].Project.Path != b || rep.Total != 4*time.Hour {
		t.Errorf("report = %+v", rep)
	}
}

func TestParseAndHuman(t *testing.T) {
	if p, ok := Parse("today"); !ok || p != Today {
		t.Error("today")
	}
	if p, ok := Parse(""); !ok || p != Week {
		t.Error("default is the week")
	}
	if _, ok := Parse("fortnight"); ok {
		t.Error("unknown periods are rejected")
	}
	for in, want := range map[time.Duration]string{
		10 * time.Second:              "<1m",
		35 * time.Minute:              "35m",
		time.Hour:                     "1h",
		2*time.Hour + 5*time.Minute:   "2h 05m",
		26*time.Hour + 14*time.Minute: "26h 14m",
	} {
		if got := Human(in); got != want {
			t.Errorf("Human(%v) = %q, want %q", in, got, want)
		}
	}
}
