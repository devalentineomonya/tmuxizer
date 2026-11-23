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
	nameErr   bool            // the title was left empty
}

// hasTitle reports whether the form asks for a session name.
func (f launchForm) hasTitle() bool { return len(f.rows) > 0 && f.rows[0].kind == rowTitle }

// tracked reports whether this launch records an iwashere session, which is
// the only thing the name is for.
func (f launchForm) tracked() bool {
	for _, r := range f.rows {
		if r.kind == rowTrack {
			return r.on
		}
	}
	return false
}

// needsName reports whether an empty title would block the launch.
func (f launchForm) needsName() bool {
	return f.hasTitle() && f.tracked() && strings.TrimSpace(f.title.Value()) == ""
}

// startNaming puts the cursor in the name input.
func (f *launchForm) startNaming() tea.Cmd {
	f.cursor, f.editing, f.titlePrev = 0, true, f.title.Value()
	return f.title.Focus()
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
		name := formRow{kind: rowTitle, label: "session name", pinned: mf.Session != ""}
		f.rows = append([]formRow{name}, f.rows...)
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
	case "/":
		if f.hasTitle() { // jump to the name from any row, like the picker's filter
			return m, f.startNaming()
		}
	case "e", "i":
		if row.kind == rowTitle {
			return m, f.startNaming()
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
			return m, f.startNaming()
		default:
			row.on = !row.on
		}
	case "enter":
		if f.manifest.err != nil {
			return m, nil // the error is shown; fix the file first
		}
		if row.kind == rowTitle {
			return m, f.startNaming() // enter on an input edits it
		}
		if f.needsName() {
			f.nameErr = true
			return m, f.startNaming()
		}
		return m.finish(Result{Action: Launch, Plan: m.resolve(f.plan(), *f)})
	case "esc", "q", "backspace":
		m.screen = picker
	}
	return m, nil
}

// editTitle handles keys while the session title is being edited.
func (m *Model) editTitle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.launch
	switch msg.String() {
	case "enter":
		if f.needsName() {
			f.nameErr = true // a session must be named; stay in the input
			return m, nil
		}
		f.editing, f.nameErr = false, false
		f.title.Blur()
		f.cursor = min(f.cursor+1, len(f.rows)-1) // on to the options
	case "esc":
		f.title.SetValue(f.titlePrev)
		f.editing, f.nameErr = false, false
		f.title.Blur()
	default:
		var cmd tea.Cmd
		f.title, cmd = f.title.Update(msg)
		f.nameErr = false
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

// viewOptions shows the launch dialog over the dimmed picker.
func (m *Model) viewOptions() string {
	return m.overlay(m.viewPicker(), m.optionsDialog())
}

func (m *Model) optionsDialog() string {
	th, f := m.th, m.launch
	width := clamp(m.width-4, 30, 76)
	inner := width - 4
	compact := m.height < 24

	var lines []string
	cursorLine := 0
	add := func(s string) { lines = append(lines, strings.Split(s, "\n")...) }
	spacer := func() {
		if !compact {
			add("")
		}
	}

	add(th.Strong.Render(f.project.Name) + "  " + th.Dim.Render(truncate(shortPath(f.project.Path), max(inner-lipgloss.Width(f.project.Name)-2, 8))))
	if !compact {
		add(th.Dim.Render(describeStack(f.stack)))
	}
	spacer()
	m.launch.title.Width = max(inner-5, 4) // box border, padding and cursor

	for i, r := range f.rows {
		active := i == f.cursor
		if r.kind == rowTitle {
			cursorLine = len(lines) + 1
			kind := frameIdle
			switch {
			case f.nameErr:
				kind = frameBad
			case active:
				kind = frameFocus
			}
			tag := ""
			if r.pinned {
				tag = "pinned"
			}
			add(m.frame("Session name", tag, m.titleInput(), inner, 3, kind))
			if note := m.titleNote(); note != "" && (!compact || f.nameErr) {
				add(note)
			}
			spacer()
			continue
		}
		bar, label := " ", pad(r.label, 12)
		if active {
			bar, label = th.Accent.Render("▌"), th.Accent.Render(label)
			cursorLine = len(lines)
		}
		add(bar + " " + label + m.rowValue(r, active))
	}
	if notes := strings.TrimRight(m.manifestNotes(f), "\n"); notes != "" {
		for _, l := range strings.Split(notes, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				add(l)
			}
		}
	}
	spacer()
	help := truncate(strings.TrimPrefix(m.optionsHelp(), " "), inner)

	body := window(lines, cursorLine, max(m.height-3, 1)) // frame borders and the help line
	return m.frame("Launch", "", strings.Join(append(body, help), "\n"), width, 0, frameFocus)
}

func (m *Model) optionsHelp() string {
	f := m.launch
	switch {
	case f.editing:
		return m.help("enter", "accept name", "esc", "cancel")
	case f.rows[f.cursor].kind == rowTitle:
		return m.help("enter", "edit name", "j/k", "move", "esc", "back")
	}
	if f.hasTitle() {
		return m.help("enter", "launch", "/", "edit name", "j/k", "move", "h/l", "change", "space", "toggle", "esc", "back")
	}
	return m.help("enter", "launch", "j/k", "move", "h/l", "change", "space", "toggle", "esc", "back")
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

// titleInput is the text shown inside the session-name box.
func (m *Model) titleInput() string {
	th, f := m.th, m.launch
	if f.editing {
		return f.title.View()
	}
	style := th.Strong
	for _, r := range f.rows {
		if r.kind == rowTrack && !r.on {
			style = th.Dim // not recorded this launch
		}
	}
	return style.Render(f.title.Value())
}

// titleNote explains the name: an error, or how a template will read.
func (m *Model) titleNote() string {
	th, f := m.th, m.launch
	if f.nameErr {
		return " " + th.Bad.Render("a session needs a name")
	}
	if val := f.title.Value(); strings.Contains(val, "{") {
		branch := m.gits[f.project.Path].Branch
		return " " + th.Dim.Render("→ "+launcher.RenderTitle(val, f.project.Name, branch, m.deps.Now()))
	}
	return ""
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
