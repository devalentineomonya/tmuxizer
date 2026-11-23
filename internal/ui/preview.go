package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/git"
	"github.com/devalentineomonya/tmuxizer/internal/iwashere"
	"github.com/devalentineomonya/tmuxizer/internal/manifest"
	"github.com/devalentineomonya/tmuxizer/internal/report"
	"github.com/devalentineomonya/tmuxizer/internal/stack"
)

const (
	labelWidth     = 9
	previewScripts = 6
)

// workInfo is what iwashere knows about a project.
type workInfo struct {
	session *iwashere.Session
	note    *iwashere.Note
	time    report.Summary
}

// manifestInfo is a project's parsed .tmuxizer.toml.
type manifestInfo struct {
	manifest.Manifest
	present bool
	err     error
}

// workOf reads iwashere data once per project; the picker is short-lived.
func (m *Model) workOf(path string) workInfo {
	w, ok := m.works[path]
	if ok {
		return w
	}
	if s, found := iwashere.CurrentSession(path); found {
		w.session = &s
	}
	if n, found := iwashere.LastNote(path); found {
		w.note = &n
	}
	w.time = report.Summarize(path, m.deps.Now())
	m.works[path] = w
	return w
}

func (m *Model) manifestOf(path string) manifestInfo {
	mi, ok := m.manifests[path]
	if !ok {
		mi.present = manifest.Exists(path)
		mi.Manifest, mi.err = manifest.Load(path)
		m.manifests[path] = mi
	}
	return mi
}

// renderPreview draws the detail panel for the selected project.
func (m *Model) renderPreview(width int) string {
	panel := m.th.Panel(width - 4)
	e, ok := m.selected()
	if !ok {
		return panel.Render("")
	}
	th := m.th
	inner := width - 6
	s := m.stackOf(e.project.Path)

	var b strings.Builder
	line := func(text string) { b.WriteString(text + "\n") }
	field := func(label, value string) {
		line(th.Dim.Render(pad(label, labelWidth)) + truncate(value, inner-labelWidth))
	}
	gap := func() { line("") }

	title := th.Accent.Render(truncate(e.project.Name, inner-2))
	if e.live() {
		title += " " + th.Good.Render("●")
	}
	line(title)
	line(th.Dim.Render(truncate(shortPath(e.project.Path), inner)))
	gap()

	field("stack", m.stackLine(s))
	if s.Run != nil {
		field("run", th.Strong.Render(s.Run.Command))
	}
	if len(s.Scripts) > 0 {
		field("scripts", th.Dim.Render(scriptNames(s.Scripts, previewScripts)))
	}
	if s.Docker {
		field("docker", th.Info.Render("compose / Dockerfile"))
	}
	if mi := m.manifestOf(e.project.Path); mi.present {
		field("config", m.manifestLine(mi))
	}
	gap()

	m.gitLines(e.project.Path, field)
	gap()

	if e.live() {
		field("tmux", th.Good.Render("● "+e.session))
	}
	w := m.workOf(e.project.Path)
	if w.session != nil {
		field("session", sessionState(th.Good, th.Warn, w.session.State)+" "+th.Dim.Render(w.session.Title))
	}
	if w.note != nil {
		field("note", th.Info.Render("“"+w.note.Message+"”")+th.Dim.Render(" · "+ago(w.note.CreatedAt, m.deps.Now())))
	}
	if t := w.time; t.Sessions > 0 {
		line := th.Strong.Render(report.Human(t.All)) + th.Dim.Render(" total")
		if t.Today > 0 {
			line += th.Dim.Render(" · ") + th.Good.Render(report.Human(t.Today)+" today")
		}
		field("time", line+th.Dim.Render(" · "+sessions(t.Sessions)))
	}
	field("opened", th.Dim.Render(ago(e.opened, m.deps.Now())))

	return panel.Render(strings.TrimRight(b.String(), "\n"))
}

func (m *Model) stackLine(s stack.Stack) string {
	if s.Primary() == "" {
		return m.th.Dim.Render("unrecognised")
	}
	out := m.th.Language(s.Primary()).Bold(true).Render(string(s.Primary()))
	for _, extra := range []string{s.Manager, s.Framework} {
		if extra != "" {
			out += m.th.Dim.Render(" · ") + extra
		}
	}
	return out
}

func (m *Model) manifestLine(mi manifestInfo) string {
	if mi.err != nil {
		return m.th.Bad.Render("invalid: " + mi.err.Error())
	}
	parts := []string{string(mi.Layout)}
	if n := len(mi.Windows); n > 0 {
		parts = append(parts, fmt.Sprintf("+%d windows", n))
	}
	return m.th.Info.Render(manifest.FileName) + m.th.Dim.Render(" · "+strings.Join(parts, " · "))
}

// gitLines renders branch, change counts and the last commit.
func (m *Model) gitLines(path string, field func(label, value string)) {
	th := m.th
	info, ok := m.gits[path]
	switch {
	case !ok:
		field("git", th.Dim.Render("…"))
		return
	case !info.Repo:
		field("git", th.Dim.Render("not a repository"))
		return
	}

	head := th.Strong.Render(info.Branch)
	if info.Clean() {
		head += " " + th.Good.Render("✓ clean")
	} else {
		head += " " + th.Warn.Render("● dirty")
	}
	if info.Ahead > 0 {
		head += " " + th.Info.Render(fmt.Sprintf("↑%d", info.Ahead))
	}
	if info.Behind > 0 {
		head += " " + th.Warn.Render(fmt.Sprintf("↓%d", info.Behind))
	}
	field("git", head)

	if counts := m.changeCounts(info); counts != "" {
		field("", counts)
	}
	if info.Subject != "" {
		field("commit", info.Subject+th.Dim.Render(" · "+ago(info.When, m.deps.Now())))
	}
}

// changeCounts colours each kind of change; zero kinds are left out.
func (m *Model) changeCounts(info git.Info) string {
	th := m.th
	var parts []string
	add := func(n int, style lipgloss.Style, format string) {
		if n > 0 {
			parts = append(parts, style.Render(fmt.Sprintf(format, n)))
		}
	}
	add(info.Staged, th.Good, "+%d staged")
	add(info.Modified, th.Warn, "~%d modified")
	add(info.Untracked, th.Info, "?%d new")
	add(info.Conflicts, th.Bad, "!%d conflicts")
	return strings.Join(parts, "  ")
}

// sessionState colours an iwashere state: active in good, paused in warn.
func sessionState(good, warn lipgloss.Style, state string) string {
	if state == "paused" {
		return warn.Render(state)
	}
	return good.Render(state)
}

// scriptNames lists up to limit script names, then a "+n" remainder.
func scriptNames(scripts []stack.Script, limit int) string {
	names := make([]string, 0, limit+1)
	for i, sc := range scripts {
		if i == limit {
			names = append(names, fmt.Sprintf("+%d", len(scripts)-limit))
			break
		}
		names = append(names, sc.Name)
	}
	return strings.Join(names, " ")
}
