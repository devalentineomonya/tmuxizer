package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/git"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/report"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

func makeProject(t *testing.T, name, file, content string) project.Project {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return project.New(dir)
}

func writeFile(t *testing.T, p project.Project, name, content string) {
	t.Helper()
	path := filepath.Join(p.Path, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newModel(t *testing.T, d Deps) *Model {
	t.Helper()
	d.Theme = theme.New("")
	d.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	m := New(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.Update(msg)
	}
}

func TestVimNavigationClamps(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	m := newModel(t, Deps{Projects: []project.Project{a, b}})

	press(m, "j", "j", "j")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (clamped)", m.cursor)
	}
	press(m, "k", "k")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	press(m, "G")
	if m.cursor != 1 {
		t.Errorf("G should jump to the end, cursor = %d", m.cursor)
	}
}

func TestRecentProjectsSortFirst(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	st := store.State{Opened: map[string]time.Time{b.Path: time.Now()}}
	m := newModel(t, Deps{Projects: []project.Project{a, b}, State: st})
	if got := m.view[0].project.Name; got != "beta" {
		t.Errorf("first = %s, want the recently opened beta", got)
	}
}

func TestFilter(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	m := newModel(t, Deps{Projects: []project.Project{a, b}})

	press(m, "/", "b", "t")
	if len(m.view) != 1 || m.view[0].project.Name != "beta" {
		t.Fatalf("filtered view = %+v", m.view)
	}
	press(m, "esc") // leave filter input, clearing it
	if len(m.view) != 2 {
		t.Errorf("esc should clear the filter, view has %d", len(m.view))
	}
}

func TestOpenAndLaunchUsesPredictedRun(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{"scripts":{"dev":"vite"}}`)
	editors := []tools.Tool{{Name: "nvim", Bin: "nvim", Args: ".", Kind: tools.Terminal}}
	agents := []tools.Tool{{Name: "claude", Bin: "claude", Kind: tools.Terminal}}
	m := newModel(t, Deps{Projects: []project.Project{p}, Editors: editors, Agents: agents})

	press(m, "enter") // open options
	if m.screen != options {
		t.Fatal("enter should show the launch options")
	}
	press(m, "j", "j", " ") // editor row -> agent row, toggle claude
	press(m, "enter")

	got := m.result
	if got.Action != Launch || got.Plan.Run != "npm run dev" {
		t.Fatalf("result = %+v", got)
	}
	if got.Plan.Editor == nil || got.Plan.Editor.Name != "nvim" {
		t.Errorf("editor should default to the best installed one, got %+v", got.Plan.Editor)
	}
	if len(got.Plan.Agents) != 1 || got.Plan.Agents[0].Name != "claude" {
		t.Errorf("agents = %+v", got.Plan.Agents)
	}
}

func TestRememberedChoicesWin(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{"scripts":{"dev":"vite","build":"vite build"}}`)
	st := store.State{Prefs: map[string]store.Prefs{p.Path: {Saved: true, Run: "", Track: false}}}
	m := newModel(t, Deps{Projects: []project.Project{p}, State: st})

	press(m, "enter", "enter")
	if m.result.Plan.Run != "" {
		t.Errorf("saved 'no run' choice should stick, got %q", m.result.Plan.Run)
	}
}

func TestLiveProjectSkipsOptions(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{}`)
	m := newModel(t, Deps{Projects: []project.Project{p}, Live: map[string]string{p.Path: "web"}})
	press(m, "enter")
	if m.result.Action != Launch || m.screen != picker {
		t.Errorf("live project should attach directly, result = %+v", m.result)
	}
}

func TestRightJumpsBetweenLiveProjectsWithoutOpening(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	c := makeProject(t, "gamma", "go.mod", "module c")
	d := makeProject(t, "delta", "go.mod", "module d")
	m := newModel(t, Deps{
		Projects: []project.Project{a, b, c, d},
		Live:     map[string]string{b.Path: "beta", d.Path: "delta"},
	})
	name := func() string { return m.view[m.cursor].project.Name }

	first := name() // live projects sort first
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	second := name()
	if !m.view[m.cursor].live() || second == first {
		t.Fatalf("right should move to the other live project, got %s then %s", first, second)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if name() != first {
		t.Errorf("right should wrap to %s, got %s", first, name())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if name() != second {
		t.Errorf("left should go back to %s, got %s", second, name())
	}
	if m.done || m.screen != picker {
		t.Errorf("moving must not open anything: done=%v screen=%v", m.done, m.screen)
	}
	press(m, "enter")
	if m.result.Action != Launch {
		t.Errorf("enter should attach, result = %+v", m.result)
	}
}

func TestRightWithoutLiveProjectsStaysPut(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	m := newModel(t, Deps{Projects: []project.Project{a, b}})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cursor != 0 || m.screen != picker || m.done {
		t.Errorf("right with no live sessions must do nothing: cursor=%d screen=%v", m.cursor, m.screen)
	}
}

func TestLaunchOptionsGoBackToPicker(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyBackspace}, {Type: tea.KeyRunes, Runes: []rune("q")}} {
		p := makeProject(t, "web", "go.mod", "module w")
		m := newModel(t, Deps{Projects: []project.Project{p}})
		press(m, "enter")
		if m.screen != options {
			t.Fatalf("enter should show the launch options, screen = %v", m.screen)
		}
		if !strings.Contains(m.View(), "esc back") {
			t.Errorf("footer should name the way back:\n%s", m.View())
		}
		m.Update(key)
		if m.screen != picker || m.done {
			t.Errorf("%q should return to the picker: screen=%v done=%v", key.String(), m.screen, m.done)
		}
	}
}

func TestShellKey(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module w")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "t")
	if m.result.Action != Shell || m.result.Dir != p.Path {
		t.Errorf("result = %+v", m.result)
	}
}

func TestResumeScreenShownFirst(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{}`)
	orphan := launcher.Orphan{Name: "web", Project: p, Prefs: store.Prefs{Saved: true, Run: "npm start"}}
	m := newModel(t, Deps{Projects: []project.Project{p}, Orphans: []launcher.Orphan{orphan}})

	if m.screen != resume {
		t.Fatal("orphans should open the resume prompt")
	}
	if !strings.Contains(m.View(), "Welcome back") {
		t.Error("resume view missing heading")
	}
	press(m, "r")
	if m.result.Plan.Mode != launcher.Resume || m.result.Plan.Run != "npm start" {
		t.Errorf("resume plan = %+v", m.result.Plan)
	}
}

func TestResumeSkipAndNew(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{}`)
	orphan := launcher.Orphan{Name: "web", Project: p}

	m := newModel(t, Deps{Projects: []project.Project{p}, Orphans: []launcher.Orphan{orphan}})
	press(m, "s")
	if m.screen != picker {
		t.Error("s should skip to the picker")
	}

	m = newModel(t, Deps{Projects: []project.Project{p}, Orphans: []launcher.Orphan{orphan}})
	press(m, "n")
	if m.result.Plan.Mode != launcher.Fresh {
		t.Errorf("n should start fresh, got %+v", m.result.Plan)
	}
}

func TestViewsRenderAtSmallSizes(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{"scripts":{"dev":"vite"}}`)
	m := newModel(t, Deps{Projects: []project.Project{p}})
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 8}, {Width: 100, Height: 12}, {Width: 200, Height: 50}} {
		m.Update(size)
		if m.View() == "" {
			t.Errorf("empty view at %dx%d", size.Width, size.Height)
		}
	}
}

func TestFuzzyMatchPositions(t *testing.T) {
	pos, ok := fuzzyMatch("tmz", "tmuxizer")
	if !ok || len(pos) != 3 || pos[0] != 0 || pos[1] != 1 || pos[2] != 5 {
		t.Errorf("positions = %v, %v", pos, ok)
	}
	if _, ok := fuzzyMatch("zt", "tmuxizer"); ok {
		t.Error("order must matter")
	}
	if _, ok := fuzzyMatch("TM", "tmuxizer"); !ok {
		t.Error("matching is case-insensitive")
	}
	if pos, ok := fuzzyMatch("", "x"); !ok || pos != nil {
		t.Error("empty query matches everything")
	}
}

func TestFilterRecordsNameMatch(t *testing.T) {
	p := makeProject(t, "tmuxizer", "go.mod", "module x")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "/", "t", "z")
	if len(m.view) != 1 || len(m.view[0].match) != 2 {
		t.Fatalf("view = %+v", m.view)
	}
	if !strings.Contains(m.View(), "tmuxizer") {
		t.Error("highlighted name must still render")
	}
}

func TestPathOnlyMatchStillListed(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "/")
	for _, r := range filepath.Base(filepath.Dir(p.Path)) {
		press(m, string(r))
	}
	if len(m.view) != 1 || m.view[0].match != nil {
		t.Errorf("path match should list the project without name highlights: %+v", m.view)
	}
}

func TestHeaderAndLiveStrip(t *testing.T) {
	a := makeProject(t, "alpha", "go.mod", "module a")
	b := makeProject(t, "beta", "go.mod", "module b")
	m := newModel(t, Deps{Projects: []project.Project{a, b}, Live: map[string]string{b.Path: "beta"}})
	view := m.View()
	for _, want := range []string{"tmuxizer", "2 projects", "1 live", "live"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if m.view[0].project.Name != "beta" {
		t.Errorf("live project should sort first, got %s", m.view[0].project.Name)
	}
}

func TestCloseNeedsConfirmation(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	var closed []string
	m := newModel(t, Deps{
		Projects: []project.Project{p},
		Live:     map[string]string{p.Path: "web"},
		Close:    func(pr project.Project) error { closed = append(closed, pr.Path); return nil },
	})

	press(m, "x")
	if !strings.Contains(m.View(), "Close session?") {
		t.Fatal("x should ask before closing")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd != nil || len(closed) != 0 {
		t.Fatal("any other key cancels")
	}

	press(m, "x")
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("y should start the close")
	}
	m.Update(cmd())
	if len(closed) != 1 || closed[0] != p.Path {
		t.Errorf("closed = %v", closed)
	}
	if m.view[0].live() || strings.Contains(m.View(), "1 live") {
		t.Error("project should no longer be live after closing")
	}
}

func TestCloseIgnoredForStoppedProject(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "x")
	if m.confirm != "" {
		t.Error("nothing to close")
	}
}

func TestCloseFailureShowsNotice(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	m := newModel(t, Deps{
		Projects: []project.Project{p},
		Live:     map[string]string{p.Path: "web"},
		Close:    func(project.Project) error { return os.ErrPermission },
	})
	press(m, "x", "y")
	m.Update(m.closeCmd(p.Path)())
	if !strings.Contains(m.View(), "could not close") || !m.view[0].live() {
		t.Error("failed close must report and keep the session listed")
	}
}

func TestManifestPinsChoices(t *testing.T) {
	p := makeProject(t, "web", "package.json", `{"scripts":{"dev":"vite"}}`)
	writeFile(t, p, ".tmuxizer.toml", "run = \"make dev\"\nagents = [\"claude\"]\ntrack = false\n")
	st := store.State{Prefs: map[string]store.Prefs{p.Path: {Saved: true, Run: "npm run dev", Track: true}}}
	agents := []tools.Tool{{Name: "claude", Bin: "claude"}, {Name: "codex", Bin: "codex"}}
	m := newModel(t, Deps{Projects: []project.Project{p}, State: st, Agents: agents, Tracking: true})

	press(m, "enter")
	if !strings.Contains(m.View(), "pinned") {
		t.Error("pinned rows should be marked")
	}
	press(m, "enter")
	plan := m.result.Plan
	if plan.Run != "make dev" || plan.Track || len(plan.Agents) != 1 || plan.Agents[0].Name != "claude" {
		t.Errorf("plan = %+v", plan)
	}
}

func TestManifestWindowsListedOnOptions(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	writeFile(t, p, ".tmuxizer.toml", "layout = \"split\"\n[[windows]]\nname = \"logs\"\ncommand = \"tail -f log\"\n")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "enter")
	view := m.View()
	for _, want := range []string{"logs", "tail -f log", "split"} {
		if !strings.Contains(view, want) {
			t.Errorf("options should show %q:\n%s", want, view)
		}
	}
}

func TestInvalidManifestBlocksLaunch(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	writeFile(t, p, ".tmuxizer.toml", "runn = \"x\"\n")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	press(m, "enter", "enter")
	if m.result.Action == Launch {
		t.Error("an invalid manifest must not launch")
	}
	if !strings.Contains(m.View(), "unknown key runn") {
		t.Error("the parse error should be visible")
	}
}

func TestPreviewShowsGitAndWork(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	writeFile(t, p, ".iwashere/notes/n.json", `{"message":"fix the login redirect","created_at":"2026-01-01T00:00:00Z"}`)
	writeFile(t, p, ".iwashere/sessions/s.json", `{"id":"s","title":"web - Jan 1","state":"paused","start_time":"2026-01-01T00:00:00Z"}`)
	m := newModel(t, Deps{Projects: []project.Project{p}})
	m.gits[p.Path] = git.Info{Repo: true, Branch: "main", Changed: 3, Staged: 1, Modified: 2, Ahead: 1, Subject: "add thing"}

	view := m.View()
	for _, want := range []string{"main", "dirty", "+1 staged", "~2 modified", "add thing", "fix the login redirect", "paused"} {
		if !strings.Contains(view, want) {
			t.Errorf("preview missing %q:\n%s", want, view)
		}
	}
}

func TestBurstOfKeysIsReplayed(t *testing.T) {
	var ps []project.Project
	for _, n := range []string{"a", "b", "c", "d"} {
		ps = append(ps, makeProject(t, n, "go.mod", "module x"))
	}
	m := newModel(t, Deps{Projects: ps})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jj")})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 after a coalesced \"jj\"", m.cursor)
	}

	// While filtering, the text must arrive whole.
	press(m, "/")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")})
	if m.filter.Value() != "ab" {
		t.Errorf("filter = %q, want ab", m.filter.Value())
	}
}

func TestPathFilterIsSubstringNotFuzzy(t *testing.T) {
	p := project.Project{Name: "web", Path: "/home/me/workspace/web"}
	if _, ok := matchProject("wk", p); ok {
		t.Error("scattered letters in the path must not match")
	}
	if _, ok := matchProject("workspace", p); !ok {
		t.Error("a folder name in the path should match")
	}
	if pos, ok := matchProject("wb", p); !ok || len(pos) != 2 {
		t.Errorf("name should still match fuzzily: %v %v", pos, ok)
	}
}

func trackingModel(t *testing.T, extra func(p project.Project)) (*Model, project.Project) {
	t.Helper()
	p := makeProject(t, "web", "go.mod", "module x")
	if extra != nil {
		extra(p)
	}
	return newModel(t, Deps{Projects: []project.Project{p}, Tracking: true, TitleTemplate: "{project} - {date} {time}"}), p
}

func TestSessionTitleShowsRenderedPreview(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter")
	view := m.View()
	for _, want := range []string{"Session name", "{project} - {date} {time}", "→ web - Jan 1 00:00"} {
		if !strings.Contains(view, want) {
			t.Errorf("options missing %q:\n%s", want, view)
		}
	}
}

func TestSessionTitleCanBeRenamed(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter") // the dialog opens with the name input focused
	if !m.launch.editing {
		t.Fatal("the session name should be asked for first")
	}
	m.launch.title.SetValue("")
	for _, r := range "Billing fix" {
		press(m, string(r))
	}
	press(m, "enter", "enter") // accept the edit, then launch
	if got := m.result.Plan.Title; got != "Billing fix" {
		t.Errorf("title = %q", got)
	}
}

func TestSessionTitleEditCanBeCancelled(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter")
	press(m, "x", "y", "z")
	press(m, "esc")
	if m.launch.editing || m.launch.title.Value() != "{project} - {date} {time}" {
		t.Errorf("esc should restore the title, got %q (editing=%v)", m.launch.title.Value(), m.launch.editing)
	}
	if m.screen != options {
		t.Error("esc while editing must not leave the options screen")
	}
}

func TestEditingKeepsNavigationKeysAsText(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter")
	m.launch.title.SetValue("")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjq")}) // a burst, not navigation
	if m.launch.title.Value() != "jjq" || m.screen != options {
		t.Errorf("title = %q, screen = %v", m.launch.title.Value(), m.screen)
	}
}

func TestManifestPinsSessionTitle(t *testing.T) {
	m, _ := trackingModel(t, func(p project.Project) {
		writeFile(t, p, ".tmuxizer.toml", "session = \"{project} on {branch}\"\n")
	})
	press(m, "enter")
	if !strings.Contains(m.View(), "pinned") {
		t.Error("manifest title should be marked pinned")
	}
	press(m, "enter", "enter") // accept the name, then launch
	if m.result.Plan.Title != "{project} on {branch}" {
		t.Errorf("title = %q", m.result.Plan.Title)
	}
}

func timedProject(t *testing.T) (*Model, project.Project) {
	return trackingModel(t, func(p project.Project) {
		writeFile(t, p, ".iwashere/sessions/s.json",
			`{"id":"s","title":"t","state":"ended","total_time":"1h30m0s","start_time":"2025-12-31T22:00:00Z","end_time":"2026-01-01T00:00:00Z"}`)
	})
}

func TestPreviewShowsTimeSpent(t *testing.T) {
	m, _ := timedProject(t)
	view := m.View()
	for _, want := range []string{"1h 30m", "total", "1 session"} {
		if !strings.Contains(view, want) {
			t.Errorf("preview missing %q:\n%s", want, view)
		}
	}
}

func TestTimeScreenCyclesPeriods(t *testing.T) {
	m, _ := timedProject(t)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m.screen != stats || cmd == nil {
		t.Fatalf("s should open the time screen, screen = %v", m.screen)
	}
	m.Update(cmd())
	view := m.View()
	for _, want := range []string{"Time spent", "today", "web", "1h 30m", "total"} {
		if !strings.Contains(view, want) {
			t.Errorf("time screen missing %q:\n%s", want, view)
		}
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.statsPeriod != report.Week || m.statsReady {
		t.Errorf("l should move to the week and reload, period = %v ready = %v", m.statsPeriod, m.statsReady)
	}
	m.Update(cmd())
	if !m.statsReady || len(m.stats.Rows) != 1 {
		t.Errorf("week report = %+v", m.stats)
	}

	press(m, "esc")
	if m.screen != picker {
		t.Error("esc returns to the picker")
	}
}

func TestStaleReportIsIgnored(t *testing.T) {
	m, _ := timedProject(t)
	press(m, "s")
	m.Update(statsMsg(report.Report{Period: report.All, Total: time.Hour})) // answer for a period no longer shown
	if m.statsReady {
		t.Error("a report for another period must not be shown")
	}
}

func TestTimeScreenEmptyState(t *testing.T) {
	p := makeProject(t, "web", "go.mod", "module x")
	m := newModel(t, Deps{Projects: []project.Project{p}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m.Update(cmd())
	if !strings.Contains(m.View(), "no time recorded") {
		t.Errorf("empty state missing:\n%s", m.View())
	}
}

func TestSessionNameIsRequired(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter")
	if !m.launch.editing || m.launch.cursor != 0 {
		t.Fatalf("name input should be focused first: editing=%v cursor=%d", m.launch.editing, m.launch.cursor)
	}
	m.launch.title.SetValue("   ")
	press(m, "enter")
	if !m.launch.editing || !m.launch.nameErr || m.done {
		t.Fatalf("a blank name must keep the input open: editing=%v err=%v done=%v", m.launch.editing, m.launch.nameErr, m.done)
	}
	if !strings.Contains(m.View(), "needs a name") {
		t.Errorf("view should explain why:\n%s", m.View())
	}
	press(m, "a")
	if m.launch.nameErr {
		t.Error("typing clears the error")
	}
}

func TestEmptyNameBlocksLaunchFromAnyRow(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter", "enter") // accept the default name, leave the input
	m.launch.title.SetValue("")
	press(m, "j", "enter") // launch from another row
	if m.done || !m.launch.editing || m.launch.cursor != 0 {
		t.Errorf("launch with no name should jump back to the input: done=%v editing=%v cursor=%d", m.done, m.launch.editing, m.launch.cursor)
	}
}

func TestUntrackedLaunchNeedsNoName(t *testing.T) {
	m, _ := trackingModel(t, func(p project.Project) {
		writeFile(t, p, ".tmuxizer.toml", "track = false\n")
	})
	press(m, "enter")
	if m.launch.editing {
		t.Error("nothing is recorded, so the name input should not grab focus")
	}
	m.launch.title.SetValue("")
	press(m, "enter")
	if !m.done {
		t.Error("an untracked launch should not require a name")
	}
}

func TestNameCanBeEditedAgainAfterAccepting(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter", "enter") // open, accept the name
	if m.launch.editing || m.launch.cursor != 1 {
		t.Fatalf("accepting should move on to the options: editing=%v cursor=%d", m.launch.editing, m.launch.cursor)
	}
	press(m, "k", "enter") // back to the name, enter edits it
	if !m.launch.editing || m.launch.cursor != 0 {
		t.Fatalf("enter on the name row should edit it: editing=%v cursor=%d", m.launch.editing, m.launch.cursor)
	}
	m.launch.title.SetValue("")
	for _, r := range "Fix auth" {
		press(m, string(r))
	}
	press(m, "enter", "enter")
	if m.result.Plan.Title != "Fix auth" {
		t.Errorf("title = %q", m.result.Plan.Title)
	}
}

func TestDialogFadesAndShadowsBackground(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter")
	for i, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > m.width {
			t.Errorf("line %d is %d cells wide, terminal is %d", i, w, m.width)
		}
	}
	if !strings.Contains(m.View(), "░░") {
		t.Error("dialog should cast a shadow")
	}
}

func TestSlashJumpsToNameFromAnyRow(t *testing.T) {
	m, _ := trackingModel(t, nil)
	press(m, "enter", "enter", "j", "j") // accept the name, move down two rows
	press(m, "/")
	if !m.launch.editing || m.launch.cursor != 0 {
		t.Errorf("/ should edit the name: editing=%v cursor=%d", m.launch.editing, m.launch.cursor)
	}
	press(m, "/")
	if m.launch.title.Value() != "{project} - {date} {time}/" {
		t.Errorf("/ while editing is text, got %q", m.launch.title.Value())
	}
}
