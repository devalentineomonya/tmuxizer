package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/devalentineomonya/tmuxizer/internal/iwashere"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/tmux"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

// fakeTmux records commands and answers has-session / display-message.
type fakeTmux struct {
	calls    []string
	sessions map[string]string // name -> start dir
}

func (f *fakeTmux) run(args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	switch args[0] {
	case "has-session":
		if _, ok := f.sessions[strings.TrimPrefix(args[2], "=")]; ok {
			return "", nil
		}
		return "", errors.New("no session")
	case "display-message":
		name := strings.TrimSuffix(strings.TrimPrefix(args[3], "="), ":")
		return f.sessions[name], nil
	case "new-session":
		f.sessions[args[3]] = args[5]
	case "kill-session":
		delete(f.sessions, strings.TrimPrefix(args[2], "="))
	}
	return "", nil
}

func (f *fakeTmux) did(prefix string) bool {
	return slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, prefix) })
}

type harness struct {
	l        *Launcher
	tmux     *fakeTmux
	iw       [][]string
	detached []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{tmux: &fakeTmux{sessions: map[string]string{}}}
	h.l = &Launcher{
		Tmux:  tmux.NewWithRunner(h.tmux.run),
		Work:  iwashere.NewWithRun(func(_ string, args ...string) error { h.iw = append(h.iw, args); return nil }),
		Store: store.New(t.TempDir()),
		Exe:   "/bin/tmuxizer",
		Now:   func() time.Time { return time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC) },
		Detach: func(bin string, _ []string, _ string) error {
			h.detached = append(h.detached, bin)
			return nil
		},
	}
	return h
}

func TestBuildCreatesWindowsInOrder(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	plan := Plan{
		Project: project.New(dir),
		Run:     "pnpm dev",
		Editor:  &tools.Tool{Name: "zed", Bin: "zed", Args: ".", Kind: tools.Window},
		Agents:  []tools.Tool{{Name: "claude", Bin: "claude", Kind: tools.Terminal}},
	}
	if err := h.l.build("app1", plan); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"new-session -d -s app1",
		"new-window -d -t =app1: -c " + dir + " -n app pnpm dev",
		"new-window -d -t =app1: -c " + dir + " -n claude claude",
		"select-window -t =app1:term",
		"set-hook -g session-closed[742]",
	}
	for _, prefix := range want {
		if !h.tmux.did(prefix) {
			t.Errorf("missing tmux call %q in %v", prefix, h.tmux.calls)
		}
	}
	if !slices.Equal(h.detached, []string{"zed"}) {
		t.Errorf("GUI editor should be detached, got %v", h.detached)
	}
}

func writeManifest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".tmuxizer.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitLayoutPutsAppBesideShell(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	writeManifest(t, dir, "layout = \"split\"\n")
	if err := h.l.build("s", Plan{Project: project.New(dir), Run: "pnpm dev"}); err != nil {
		t.Fatal(err)
	}
	if !h.tmux.did("split-window -d -h -l 35% -t =s:term -c " + dir + " pnpm dev") {
		t.Errorf("missing split in %v", h.tmux.calls)
	}
	if h.tmux.did("new-window -d -t =s: -c " + dir + " -n app") {
		t.Error("split layout must not open an app window")
	}
}

func TestManifestWindowsOpenInOrder(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	writeManifest(t, dir, "[[windows]]\nname = \"logs\"\ncommand = \"tail -f log\"\n"+
		"[[windows]]\nname = \"api\"\ncommand = \"air\"\ndir = \"apps/api\"\n")
	plan := Plan{Project: project.New(dir), Run: "make dev", Agents: []tools.Tool{{Name: "claude", Bin: "claude"}}}
	if err := h.l.build("m", plan); err != nil {
		t.Fatal(err)
	}

	var windows []string
	for _, c := range h.tmux.calls {
		if strings.HasPrefix(c, "new-window") {
			fields := strings.Fields(c)
			windows = append(windows, fields[slices.Index(fields, "-n")+1])
		}
	}
	if !slices.Equal(windows, []string{"app", "logs", "api", "claude"}) {
		t.Errorf("window order = %v", windows)
	}
	if !h.tmux.did("new-window -d -t =m: -c " + filepath.Join(dir, "apps/api") + " -n api air") {
		t.Errorf("window dir not applied: %v", h.tmux.calls)
	}
}

func TestInvalidManifestAbortsBeforeTouchingTmux(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	writeManifest(t, dir, "layout = \"grid\"\n")
	err := h.l.build("bad", Plan{Project: project.New(dir)})
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Errorf("err = %v", err)
	}
	if h.tmux.did("new-session") {
		t.Error("no session should be created for an invalid manifest")
	}
}

func TestCloseKillsSessionAndForgetsIt(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	proj := project.New(dir)
	h.tmux.sessions[proj.Name] = dir
	_ = h.l.Store.Update(func(s *store.State) {
		s.Sessions[proj.Name] = store.Session{Path: dir}
	})

	if err := h.l.Close(proj); err != nil {
		t.Fatal(err)
	}
	if _, alive := h.tmux.sessions[proj.Name]; alive {
		t.Error("session should be killed")
	}
	st, _ := h.l.Store.Load()
	if _, ok := st.Sessions[proj.Name]; ok {
		t.Error("state should forget the session")
	}
	if err := h.l.Close(proj); err != nil {
		t.Errorf("closing a stopped project is a no-op, got %v", err)
	}
}

func TestBuildRecordsState(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	if err := h.l.build("x", Plan{Project: project.New(dir), Run: "go run ."}); err != nil {
		t.Fatal(err)
	}
	st, _ := h.l.Store.Load()
	if st.Sessions["x"].Path != dir || st.Prefs[dir].Run != "go run ." || st.Opened[dir].IsZero() {
		t.Errorf("state not recorded: %+v", st)
	}
}

func TestSessionForReusesLiveAndAvoidsCollisions(t *testing.T) {
	h := newHarness(t)
	a := project.Project{Name: "api", Path: "/work/a/api"}
	b := project.Project{Name: "api", Path: "/work/b/api"}

	h.tmux.sessions["api"] = a.Path
	name, live, _ := h.l.sessionFor(a)
	if name != "api" || !live {
		t.Errorf("same path should reuse: got %q live=%v", name, live)
	}
	name, live, _ = h.l.sessionFor(b)
	if name != "b-api" || live {
		t.Errorf("different path should get a new name: got %q live=%v", name, live)
	}
}

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		"my.app": "my_app", "a:b": "a_b", "ok-name_1": "ok-name_1", "...": "project",
	} {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClosedEndsTrackedSessionAndForgets(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	_ = h.l.Store.Update(func(s *store.State) {
		s.Sessions["x"] = store.Session{Path: dir, Prefs: store.Prefs{Track: true}}
	})
	if err := h.l.Closed("x"); err != nil {
		t.Fatal(err)
	}
	// End only calls the CLI when an open session exists on disk.
	st, _ := h.l.Store.Load()
	if _, ok := st.Sessions["x"]; ok {
		t.Error("session should be forgotten")
	}
	if err := h.l.Closed("not-ours"); err != nil {
		t.Errorf("unknown sessions must be ignored, got %v", err)
	}
}

func TestOrphansAreDeadSessionsOnly(t *testing.T) {
	h := newHarness(t)
	h.tmux.sessions["alive"] = "/a"
	st := store.State{Sessions: map[string]store.Session{
		"alive": {Path: "/a"},
		"dead":  {Path: "/d", Started: time.Now()},
	}}
	got := h.l.Orphans(st)
	if len(got) != 1 || got[0].Name != "dead" {
		t.Fatalf("orphans = %+v, want only dead", got)
	}
}

func TestHookCommandQuotesBinary(t *testing.T) {
	l := &Launcher{Exe: `/opt/my "bin"/tmuxizer`}
	got := l.hookCommand()
	if !strings.Contains(got, `/opt/my \"bin\"/tmuxizer _closed`) {
		t.Errorf("hook command not escaped: %s", got)
	}
}

func TestRenderTitle(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	tests := []struct{ tmpl, branch, want string }{
		{"{project} - {date} {time}", "", "api - Jan 2 15:04"},
		{"{project} ({branch})", "main", "api (main)"},
		{"{project} {branch} work", "", "api work"}, // no stray double space
		{"plain title", "main", "plain title"},
	}
	for _, tc := range tests {
		if got := RenderTitle(tc.tmpl, "api", tc.branch, now); got != tc.want {
			t.Errorf("RenderTitle(%q, branch=%q) = %q, want %q", tc.tmpl, tc.branch, got, tc.want)
		}
	}
}

func TestTitleResolutionOrder(t *testing.T) {
	h := newHarness(t)
	h.l.TitleTemplate = "cfg {project}"
	h.l.Branch = func(string) string { return "main" }
	plan := Plan{Project: project.Project{Name: "api", Path: "/x/api"}}

	check := func(label, want string, p Plan, manifestTitle string) {
		t.Helper()
		if got := h.l.title(p, manifestTitle); got != want {
			t.Errorf("%s: title = %q, want %q", label, got, want)
		}
	}
	check("config", "cfg api", plan, "")
	check("manifest beats config", "man api", plan, "man {project}")
	plan.Title = "chosen on {branch}"
	check("launch choice beats manifest", "chosen on main", plan, "man {project}")

	plan.Title = "   "
	h.l.TitleTemplate = ""
	check("default", "api - Jan 2 15:04", plan, "")
	h.l.Branch = func(string) string { return "" }
	check("blank render falls back", "api - Jan 2 15:04", plan, "{branch}")
}
