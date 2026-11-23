package ui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
	"github.com/devalentineomonya/tmuxizer/internal/manifest"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/stack"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

type rowKind int

const (
	rowRun rowKind = iota
	rowEditor
	rowAgent
	rowTrack
	rowTitle
)

// formRow is one line of the launch options: either a value cycled with
// h/l (run, editor) or an on/off toggle (agents, tracking).
type formRow struct {
	kind    rowKind
	label   string
	choices []string // cycle rows: display text, first is "none"
	index   int      // cycle rows: selected choice
	on      bool     // toggle rows
	tool    tools.Tool
	pinned  bool // value comes from the project's manifest
}

func (r formRow) cycles() bool { return r.kind == rowRun || r.kind == rowEditor }

type launchForm struct {
	project  project.Project
	stack    stack.Stack
	manifest manifestInfo
	rows     []formRow
	cursor   int

	title     textinput.Model // iwashere session title template
	titlePrev string          // value to restore when an edit is cancelled
	editing   bool            // keys go to the title input
}

// newLaunchForm builds the options for a project. A value pinned in the
// project's manifest wins; otherwise the remembered choice, then detection.
func (m *Model) newLaunchForm(p project.Project) launchForm {
	s := m.stackOf(p.Path)
	mf := m.manifestOf(p.Path)
	prefs := m.deps.State.Prefs[p.Path]
	f := launchForm{project: p, stack: s, manifest: mf}

	runs := []string{"none"}
	runs = appendUnique(runs, []string{mf.Run})
	runs = appendUnique(runs, scriptCommands(s))
	run := formRow{kind: rowRun, label: "run", choices: runs}
	switch {
	case mf.Run != "":
		run.index, run.pinned = indexOf(runs, mf.Run), true
	case prefs.Saved:
		run.index = indexOf(runs, prefs.Run)
	case s.Run != nil:
		run.index = indexOf(runs, s.Run.Command)
	}
	f.rows = append(f.rows, run)

	if len(m.deps.Editors) > 0 {
		names := []string{"none"}
		for _, e := range m.deps.Editors {
			names = append(names, e.Name)
		}
		ed := formRow{kind: rowEditor, label: "editor", choices: names, index: 1}
		switch {
		case mf.Editor != "":
			ed.index, ed.pinned = indexOf(names, mf.Editor), true
		case prefs.Saved:
			ed.index = indexOf(names, prefs.Editor)
		}
		f.rows = append(f.rows, ed)
	}

	for _, a := range m.deps.Agents {
		row := formRow{kind: rowAgent, label: a.Name, tool: a, on: prefs.Saved && contains(prefs.Agents, a.Name)}
		if mf.Agents != nil {
			row.on, row.pinned = contains(mf.Agents, a.Name), true
		}
		f.rows = append(f.rows, row)
	}
	if m.deps.Tracking {
		row := formRow{kind: rowTrack, label: "iwashere", on: !prefs.Saved || prefs.Track}
		if mf.Track != nil {
			row.on, row.pinned = *mf.Track, true
		}
		f.rows = append(f.rows, row)

		f.title = textinput.New()
		f.title.Prompt = ""
		f.title.CharLimit = 80
		f.title.Width = 44
		f.title.SetValue(firstNonEmpty(mf.Session, m.deps.TitleTemplate, launcher.DefaultTitle))
		f.rows = append(f.rows, formRow{kind: rowTitle, label: "session", pinned: mf.Session != ""})
	}
	return f
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func scriptCommands(s stack.Stack) []string {
	var cmds []string
	if s.Run != nil {
		cmds = append(cmds, s.Run.Command) // predicted command first
	}
	for _, sc := range s.Scripts {
		cmds = append(cmds, sc.Command)
	}
	return cmds
}

func appendUnique(dst, src []string) []string {
	for _, v := range src {
		if v != "" && !contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

func contains(list []string, v string) bool { return slices.Contains(list, v) }

// indexOf returns the position of v, or 0 ("none") when absent.
func indexOf(list []string, v string) int {
	if v == "" {
		return 0
	}
	return max(slices.Index(list, v), 0)
}

// plan converts the form state to a launch plan.
func (f launchForm) plan() launcher.Plan {
	p := launcher.Plan{Project: f.project}
	for _, r := range f.rows {
		switch r.kind {
		case rowRun:
			if r.index > 0 {
				p.Run = r.choices[r.index]
			}
		case rowEditor:
			// resolved by the caller, which owns the editor list
		case rowAgent:
			if r.on {
				p.Agents = append(p.Agents, r.tool)
			}
		case rowTrack:
			p.Track = r.on
		case rowTitle:
			p.Title = strings.TrimSpace(f.title.Value())
		}
	}
	return p
}

func (m *Model) updateOptions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.launch
	if f.editing {
		return m.editTitle(msg)
	}
	row := &f.rows[f.cursor]
	switch msg.String() {
	case "e", "i":
		if row.kind == rowTitle {
			f.editing, f.titlePrev = true, f.title.Value()
			return m, f.title.Focus()
		}
	case "j", "down", "tab":
		f.cursor = (f.cursor + 1) % len(f.rows)
	case "k", "up", "shift+tab":
		f.cursor = (f.cursor + len(f.rows) - 1) % len(f.rows)
	case "l", "right":
		cycle(row, 1)
	case "h", "left":
		cycle(row, -1)
	case " ", "x":
		switch {
		case row.cycles():
			cycle(row, 1)
		case row.kind == rowTitle:
			f.editing, f.titlePrev = true, f.title.Value()
			return m, f.title.Focus()
		default:
			row.on = !row.on
		}
	case "enter":
		if f.manifest.err != nil {
			return m, nil // the error is shown; fix the file first
		}
		return m.finish(Result{Action: Launch, Plan: m.resolve(f.plan(), *f)})
	case "esc", "q":
		m.screen = picker
	}
	return m, nil
}

// editTitle handles keys while the session title is being edited.
func (m *Model) editTitle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.launch
	switch msg.String() {
	case "enter":
		f.editing = false
		f.title.Blur()
	case "esc":
		f.title.SetValue(f.titlePrev)
		f.editing = false
		f.title.Blur()
	default:
		var cmd tea.Cmd
		f.title, cmd = f.title.Update(msg)
		return m, cmd
	}
	return m, nil
}

// resolve fills in the editor, which the form tracks only by name.
func (m *Model) resolve(p launcher.Plan, f launchForm) launcher.Plan {
	for _, r := range f.rows {
		if r.kind == rowEditor && r.index > 0 { // resolved here, not in plan()
			if t, ok := tools.ByName(m.deps.Editors, r.choices[r.index]); ok {
				p.Editor = &t
			}
		}
	}
	return p
}

func cycle(r *formRow, delta int) {
	if !r.cycles() {
		r.on = !r.on
		return
	}
	r.index = (r.index + delta + len(r.choices)) % len(r.choices)
}

func (m *Model) viewOptions() string {
	th := m.th
	f := m.launch
	var b strings.Builder

	b.WriteString(" " + th.Accent.Render(f.project.Name) + "  " + th.Dim.Render(shortPath(f.project.Path)) + "\n")
	b.WriteString(" " + th.Dim.Render(describeStack(f.stack)) + "\n\n")

	for i, r := range f.rows {
		bar := " "
		label := pad(r.label, 12)
		if i == f.cursor {
			bar = th.Accent.Render("▌")
			label = th.Accent.Render(label)
		}
		b.WriteString(" " + bar + " " + label + m.rowValue(r, i == f.cursor) + "\n")
	}
	b.WriteString(m.manifestNotes(f))
	b.WriteString("\n" + m.optionsHelp())

	return lipgloss.NewStyle().MarginTop(1).Render(b.String())
}

func (m *Model) optionsHelp() string {
	f := m.launch
	switch {
	case f.editing:
		return m.help("type", "the title", "enter", "accept", "esc", "cancel")
	case f.rows[f.cursor].kind == rowTitle:
		return m.help("e", "edit title", "j/k", "move", "enter", "launch", "esc", "back")
	}
	return m.help("j/k", "move", "h/l", "change", "space", "toggle", "enter", "launch", "esc", "back")
}

// manifestNotes lists what the project's manifest adds beyond the rows.
func (m *Model) manifestNotes(f launchForm) string {
	th := m.th
	mf := f.manifest
	if !mf.present {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n " + th.Dim.Render(manifest.FileName) + "\n")
	if mf.err != nil {
		b.WriteString(" " + th.Bad.Render(truncate(mf.err.Error(), max(m.width-3, 10))) + "\n")
		return b.String()
	}
	if mf.Layout == manifest.Split {
		b.WriteString("   " + th.Info.Render("split") + th.Dim.Render("  app runs beside the shell") + "\n")
	}
	for _, w := range mf.Windows {
		b.WriteString("   " + th.Info.Render("+ "+pad(w.Name, 10)) + th.Dim.Render(truncate(w.Command, max(m.width-18, 10))) + "\n")
	}
	return b.String()
}

func (m *Model) rowValue(r formRow, active bool) string {
	value := m.rowChoice(r, active)
	if r.pinned {
		value += m.th.Dim.Render("  · pinned")
	}
	return value
}

func (m *Model) rowChoice(r formRow, active bool) string {
	th := m.th
	if r.kind == rowTitle {
		return m.titleValue()
	}
	if r.cycles() {
		v := r.choices[r.index]
		if r.index == 0 {
			return th.Dim.Render(v)
		}
		if active {
			return th.Dim.Render("‹ ") + th.Strong.Render(v) + th.Dim.Render(" ›")
		}
		return th.Strong.Render(v)
	}
	if r.on {
		return th.Good.Render("● on")
	}
	return th.Dim.Render("○ off")
}

// titleValue shows the session title template and, when it has tokens, how
// it will read.
func (m *Model) titleValue() string {
	th, f := m.th, m.launch
	if f.editing {
		return f.title.View()
	}
	val := f.title.Value()
	style := th.Strong
	for _, r := range f.rows {
		if r.kind == rowTrack && !r.on {
			style = th.Dim // not recorded this launch
		}
	}
	out := style.Render(val)
	if strings.Contains(val, "{") {
		branch := m.gits[f.project.Path].Branch
		out += th.Dim.Render("  → " + launcher.RenderTitle(val, f.project.Name, branch, m.deps.Now()))
	}
	return out
}

func describeStack(s stack.Stack) string {
	if s.Primary() == "" {
		return "no language detected"
	}
	parts := []string{string(s.Primary())}
	for _, p := range []string{s.Manager, s.Framework} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// planFromPrefs rebuilds a plan from remembered choices; tools that are no
// longer installed are dropped.
func (m *Model) planFromPrefs(p project.Project, prefs store.Prefs, mode launcher.Mode) launcher.Plan {
	plan := launcher.Plan{Project: p, Run: prefs.Run, Track: prefs.Track, Mode: mode}
	if t, ok := tools.ByName(m.deps.Editors, prefs.Editor); ok {
		plan.Editor = &t
	}
	for _, name := range prefs.Agents {
		if t, ok := tools.ByName(m.deps.Agents, name); ok {
			plan.Agents = append(plan.Agents, t)
		}
	}
	return plan
}
