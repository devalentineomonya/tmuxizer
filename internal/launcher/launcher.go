// Package launcher turns a chosen project and options into a tmux session.
package launcher

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/devalentineomonya/tmuxizer/internal/git"
	"github.com/devalentineomonya/tmuxizer/internal/iwashere"
	"github.com/devalentineomonya/tmuxizer/internal/manifest"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/tmux"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

const (
	termWindow = "term"
	appWindow  = "app"
	// hookIndex keeps our session-closed hook apart from the user's own.
	hookIndex = 742
	// splitPercent is the side pane's share of the window in the split layout.
	splitPercent = 35
)

// Mode says how a launch relates to an earlier, unfinished session.
type Mode int

const (
	// Fresh starts a new work session, ending any stale one.
	Fresh Mode = iota
	// Resume rebuilds the layout and continues the stale work session.
	Resume
)

// Plan is everything needed to open a project.
type Plan struct {
	Project project.Project
	Run     string // command for the app window, "" for none
	Editor  *tools.Tool
	Agents  []tools.Tool
	Track   bool   // record an iwashere session
	Title   string // session title template; "" uses the manifest, config or default
	Mode    Mode
}

// Prefs converts the plan to the choices worth remembering.
func (p Plan) Prefs() store.Prefs {
	prefs := store.Prefs{Saved: true, Run: p.Run, Track: p.Track}
	if p.Editor != nil {
		prefs.Editor = p.Editor.Name
	}
	for _, a := range p.Agents {
		prefs.Agents = append(prefs.Agents, a.Name)
	}
	return prefs
}

// Launcher builds and tears down sessions.
type Launcher struct {
	Tmux  *tmux.Client
	Work  *iwashere.Client
	Store *store.Store
	Exe   string // path of this binary, used by the session-closed hook
	Now   func() time.Time

	// TitleTemplate is the configured default session title.
	TitleTemplate string
	// Branch returns the current git branch of dir, for the {branch} token.
	Branch func(dir string) string

	// Detach starts a GUI tool in dir without waiting for it.
	Detach func(bin string, args []string, dir string) error
}

// New wires a Launcher to the real tmux, iwashere and filesystem.
func New(st *store.Store, exe string) *Launcher {
	return &Launcher{
		Tmux:   tmux.New(),
		Work:   iwashere.New(),
		Store:  st,
		Exe:    exe,
		Now:    time.Now,
		Detach: detach,
		Branch: func(dir string) string { return git.Status(dir).Branch },
	}
}

// Launch opens the project's session and enters it. Outside tmux it replaces
// the process on success, so it only returns on failure.
func (l *Launcher) Launch(p Plan) error {
	name, live, err := l.sessionFor(p.Project)
	if err != nil {
		return err
	}
	if !live {
		if err := l.build(name, p); err != nil {
			_ = l.Tmux.KillSession(name)
			return err
		}
	}
	return l.Tmux.Enter(name)
}

// Close ends a project's running session. The session-closed hook normally
// ends the work session and forgets the state; Closed runs here as well in
// case the hook is missing, and it is safe to run twice.
func (l *Launcher) Close(proj project.Project) error {
	name, live, err := l.sessionFor(proj)
	if err != nil || !live {
		return err
	}
	if err := l.Tmux.KillSession(name); err != nil {
		return err
	}
	return l.Closed(name)
}

// Live reports the tmux session already running for a project, if any.
func (l *Launcher) Live(proj project.Project) (string, bool) {
	name, live, err := l.sessionFor(proj)
	return name, err == nil && live
}

func (l *Launcher) build(name string, p Plan) error {
	dir := p.Project.Path
	mf, err := manifest.Load(dir)
	if err != nil {
		return err
	}
	if err := l.Tmux.NewSession(name, dir, termWindow); err != nil {
		return err
	}
	if err := l.startApp(name, dir, p.Run, mf.Layout); err != nil {
		return err
	}
	for _, w := range mf.Windows {
		if err := l.Tmux.NewWindow(name, filepath.Join(dir, w.Dir), w.Name, tmux.KeepOpen(w.Command)); err != nil {
			return err
		}
	}
	for _, t := range p.windows() {
		if err := l.open(name, dir, t); err != nil {
			return err
		}
	}
	if err := l.Tmux.SelectWindow(name, termWindow); err != nil {
		return err
	}
	if err := l.Tmux.SetHook("session-closed", hookIndex, l.hookCommand()); err != nil {
		return err
	}
	l.track(p, mf.Session)
	return l.record(name, p)
}

// startApp runs the app command in its own window or, for the split layout,
// in a side pane of the shell window.
func (l *Launcher) startApp(session, dir, run string, layout manifest.Layout) error {
	if run == "" {
		return nil
	}
	if layout == manifest.Split {
		return l.Tmux.Split(session, termWindow, dir, tmux.KeepOpen(run), splitPercent)
	}
	return l.Tmux.NewWindow(session, dir, appWindow, tmux.KeepOpen(run))
}

// windows lists the editor and agents in launch order.
func (p Plan) windows() []tools.Tool {
	var ts []tools.Tool
	if p.Editor != nil {
		ts = append(ts, *p.Editor)
	}
	return append(ts, p.Agents...)
}

func (l *Launcher) open(session, dir string, t tools.Tool) error {
	if t.Kind == tools.Window {
		return l.Detach(t.Bin, strings.Fields(t.Args), dir)
	}
	return l.Tmux.NewWindow(session, dir, t.Name, tmux.KeepOpen(t.Command()))
}

// track starts or continues the iwashere session. A tracking failure never
// blocks opening the project, so it is deliberately not returned.
func (l *Launcher) track(p Plan, manifestTitle string) {
	if !p.Track || !iwashere.Available() {
		return
	}
	dir := p.Project.Path
	_ = l.Work.Init(dir)
	if p.Mode == Fresh {
		_ = l.Work.End(dir)
	} else {
		_ = l.Work.Continue(dir)
	}
	_ = l.Work.Start(dir, l.title(p, manifestTitle))
}

// title picks the session title: the launch choice, then the project
// manifest, then the user's config, then the built-in default.
func (l *Launcher) title(p Plan, manifestTitle string) string {
	tmpl := firstNonEmpty(p.Title, manifestTitle, l.TitleTemplate, DefaultTitle)
	branch := ""
	if strings.Contains(tmpl, "{branch}") && l.Branch != nil {
		branch = l.Branch(p.Project.Path)
	}
	if t := RenderTitle(tmpl, p.Project.Name, branch, l.Now()); t != "" {
		return t
	}
	return RenderTitle(DefaultTitle, p.Project.Name, branch, l.Now())
}

// DefaultTitle is the session title template when none is configured.
const DefaultTitle = "{project} - {date} {time}"

// RenderTitle fills a title template. Tokens: {project}, {branch}, {date}
// ("Jan 2") and {time} ("15:04"). An empty branch leaves no stray separators.
func RenderTitle(tmpl, project, branch string, now time.Time) string {
	out := strings.NewReplacer(
		"{project}", project,
		"{branch}", branch,
		"{date}", now.Format("Jan 2"),
		"{time}", now.Format("15:04"),
	).Replace(tmpl)
	return strings.Join(strings.Fields(out), " ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (l *Launcher) record(name string, p Plan) error {
	now := l.Now()
	return l.Store.Update(func(st *store.State) {
		st.Opened[p.Project.Path] = now
		st.Prefs[p.Project.Path] = p.Prefs()
		st.Sessions[name] = store.Session{Path: p.Project.Path, Started: now, Prefs: p.Prefs()}
	})
}

// hookCommand is run by tmux when any session closes. It must run in the
// foreground: a background job is lost when the last session ends and the
// tmux server exits with it.
func (l *Launcher) hookCommand() string {
	return fmt.Sprintf(`run-shell "%s _closed '#{hook_session_name}'"`, strings.ReplaceAll(l.Exe, `"`, `\"`))
}

// Closed is the session-closed hook: it ends the work session and forgets
// the tmux session. Sessions tmuxizer did not create are ignored.
func (l *Launcher) Closed(name string) error {
	st, err := l.Store.Load()
	if err != nil {
		return err
	}
	sess, ok := st.Sessions[name]
	if !ok {
		return nil
	}
	if sess.Prefs.Track {
		_ = l.Work.End(sess.Path)
	}
	return l.Store.Update(func(st *store.State) { delete(st.Sessions, name) })
}

// Orphan is a session tmuxizer started whose tmux session no longer exists,
// typically after a reboot or crash.
type Orphan struct {
	Name    string
	Project project.Project
	Started time.Time
	Prefs   store.Prefs
	Work    *iwashere.Session // the unfinished work session, if recorded
}

// Orphans lists sessions that did not end cleanly, oldest first.
func (l *Launcher) Orphans(st store.State) []Orphan {
	var out []Orphan
	for name, s := range st.Sessions {
		if l.Tmux.HasSession(name) {
			continue
		}
		o := Orphan{Name: name, Project: project.New(s.Path), Started: s.Started, Prefs: s.Prefs}
		if cur, ok := iwashere.CurrentSession(s.Path); ok {
			o.Work = &cur
		}
		out = append(out, o)
	}
	sortOrphans(out)
	return out
}

// Dismiss ends an orphan's work session and forgets it.
func (l *Launcher) Dismiss(o Orphan) error {
	if o.Prefs.Track {
		_ = l.Work.End(o.Project.Path)
	}
	return l.Store.Update(func(st *store.State) { delete(st.Sessions, o.Name) })
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// sessionFor picks a tmux session name for the project and says whether it
// is already running. Names that belong to another directory are skipped.
func (l *Launcher) sessionFor(proj project.Project) (name string, live bool, err error) {
	candidates := []string{sanitize(proj.Name)}
	parent := filepath.Base(filepath.Dir(proj.Path))
	candidates = append(candidates, sanitize(parent+"-"+proj.Name))
	for i := 2; i < 10; i++ {
		candidates = append(candidates, fmt.Sprintf("%s-%d", candidates[0], i))
	}

	for _, c := range candidates {
		if !l.Tmux.HasSession(c) {
			return c, false, nil
		}
		if path, perr := l.Tmux.SessionPath(c); perr == nil && path == proj.Path {
			return c, true, nil
		}
	}
	return "", false, fmt.Errorf("no free tmux session name for %s", proj.Name)
}

func sortOrphans(os []Orphan) {
	sort.Slice(os, func(i, j int) bool { return os[i].Started.Before(os[j].Started) })
}

func sanitize(s string) string {
	s = strings.Trim(unsafeName.ReplaceAllString(s, "_"), "_")
	if s == "" {
		return "project"
	}
	return s
}

func detach(bin string, args []string, dir string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", bin, err)
	}
	return cmd.Process.Release()
}
