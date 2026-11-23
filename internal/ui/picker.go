package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/git"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/stack"
)

const (
	chromeLines  = 4  // header, pane top and bottom borders, help line
	filterLines  = 3  // the filter input box
	splitMinCols = 96 // narrower terminals skip the preview pane
)

// projects returns the list's projects, for rebuilding after a change.
func (m *Model) projects() []project.Project {
	out := make([]project.Project, len(m.all))
	for i, e := range m.all {
		out[i] = e.project
	}
	return out
}

// setProjects rebuilds the list, keeping the cursor on the same project.
func (m *Model) setProjects(projects []project.Project) {
	selected := m.selectedPath()

	m.all = m.all[:0]
	seen := map[string]bool{}
	for _, p := range projects {
		if seen[p.Path] {
			continue
		}
		seen[p.Path] = true
		m.all = append(m.all, entry{
			project: p,
			opened:  m.deps.State.Opened[p.Path],
			session: m.deps.Live[p.Path],
			current: p.Path == m.deps.Current,
		})
	}
	slices.SortFunc(m.all, compareEntries)
	m.applyFilter()

	for i, e := range m.view {
		if e.project.Path == selected {
			m.cursor = i
			break
		}
	}
	m.keepCursorVisible()
}

// compareEntries orders: working directory, running sessions, recents, name.
func compareEntries(a, b entry) int {
	switch {
	case a.current != b.current:
		return boolOrder(a.current)
	case a.live() != b.live():
		return boolOrder(a.live())
	case !a.opened.Equal(b.opened):
		return b.opened.Compare(a.opened)
	}
	return strings.Compare(strings.ToLower(a.project.Name), strings.ToLower(b.project.Name))
}

func boolOrder(first bool) int {
	if first {
		return -1
	}
	return 1
}

func (m *Model) applyFilter() {
	q := m.filter.Value()
	m.view = m.view[:0]
	for _, e := range m.all {
		pos, ok := matchProject(q, e.project)
		if !ok {
			continue
		}
		e.match = pos
		m.view = append(m.view, e)
	}
	m.cursor = clamp(m.cursor, 0, max(len(m.view)-1, 0))
	m.keepCursorVisible()
}

// matchProject fuzzy-matches the name, which can be highlighted. A query
// that is not in the name may still be a substring of the path, so typing a
// parent folder lists its projects; fuzzy matching a path would match nearly
// everything.
func matchProject(query string, p project.Project) ([]int, bool) {
	if pos, ok := fuzzyMatch(query, p.Name); ok {
		return pos, true
	}
	return nil, strings.Contains(strings.ToLower(p.Path), strings.ToLower(query))
}

func (m *Model) selected() (entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return entry{}, false
	}
	return m.view[m.cursor], true
}

func (m *Model) selectedPath() string {
	if e, ok := m.selected(); ok {
		return e.project.Path
	}
	return ""
}

func (m *Model) liveEntries() []entry {
	var live []entry
	for _, e := range m.all {
		if e.live() {
			live = append(live, e)
		}
	}
	return live
}

// showFilter reports whether the filter box is on screen.
func (m *Model) showFilter() bool { return m.filtered || m.filter.Value() != "" }

func (m *Model) listHeight() int {
	rows := m.height - chromeLines
	if len(m.liveEntries()) > 0 {
		rows-- // the live strip
	}
	if m.showFilter() {
		rows -= filterLines
	}
	return max(rows, 1)
}

func (m *Model) keepCursorVisible() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = clamp(m.offset, 0, max(len(m.view)-h, 0))
}

func (m *Model) move(delta int) tea.Cmd {
	m.cursor = clamp(m.cursor+delta, 0, max(len(m.view)-1, 0))
	m.keepCursorVisible()
	return m.gitCmd()
}

// jumpLive moves the selection to the next (dir 1) or previous (dir -1)
// running session, wrapping around. It only selects; enter does the opening.
func (m *Model) jumpLive(dir int) tea.Cmd {
	n := len(m.view)
	for i := 1; i < n; i++ {
		idx := ((m.cursor+dir*i)%n + n) % n
		if m.view[idx].live() {
			return m.move(idx - m.cursor)
		}
	}
	return nil
}

// gitCmd fetches git status for the selected project once.
func (m *Model) gitCmd() tea.Cmd {
	path := m.selectedPath()
	if path == "" {
		return nil
	}
	if _, done := m.gits[path]; done || m.gitFlight[path] {
		return nil
	}
	m.gitFlight[path] = true
	return func() tea.Msg { return gitMsg{path: path, info: git.Status(path)} }
}

func (m *Model) stackOf(path string) stack.Stack {
	s, ok := m.stacks[path]
	if !ok {
		s = stack.Detect(path)
		m.stacks[path] = s
	}
	return s
}

func (m *Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	m.notice = ""

	if m.confirm != "" {
		path := m.confirm
		m.confirm = ""
		if key == "y" {
			return m, m.closeCmd(path)
		}
		return m, nil
	}

	if m.filtered {
		switch key {
		case "enter":
			m.filtered = false
			m.filter.Blur()
			return m, m.gitCmd()
		case "esc":
			m.filtered = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.applyFilter()
			return m, m.gitCmd()
		case "ctrl+n", "down":
			return m, m.move(1)
		case "ctrl+p", "up":
			return m, m.move(-1)
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilter()
		return m, tea.Batch(cmd, m.gitCmd())
	}

	switch key {
	case "j", "down", "ctrl+n":
		return m, m.move(1)
	case "k", "up", "ctrl+p":
		return m, m.move(-1)
	case "ctrl+d":
		return m, m.move(m.listHeight() / 2)
	case "ctrl+u":
		return m, m.move(-m.listHeight() / 2)
	case "g", "home":
		return m, m.move(-len(m.view))
	case "G", "end":
		return m, m.move(len(m.view))
	case "/":
		m.filtered = true
		m.keepCursorVisible()
		return m, m.filter.Focus()
	case "enter":
		return m.open()
	case "l", "right":
		return m, m.jumpLive(1)
	case "h", "left":
		return m, m.jumpLive(-1)
	case "s":
		m.screen, m.statsReady = stats, false
		return m, m.statsCmd()
	case "x":
		if e, ok := m.selected(); ok && e.live() {
			m.confirm = e.project.Path
		}
	case "t":
		if e, ok := m.selected(); ok {
			return m.finish(Result{Action: Shell, Dir: e.project.Path})
		}
	case "q", "esc":
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
			return m, m.gitCmd()
		}
		return m.finish(Result{Action: Quit})
	}
	return m, nil
}

// closeCmd ends a project's session in the background.
func (m *Model) closeCmd(path string) tea.Cmd {
	closeFn := m.deps.Close
	proj := project.New(path)
	return func() tea.Msg {
		if closeFn == nil {
			return closeMsg{path: path, err: fmt.Errorf("not supported")}
		}
		return closeMsg{path: path, err: closeFn(proj)}
	}
}

// open attaches to a running session or shows the launch options.
func (m *Model) open() (tea.Model, tea.Cmd) {
	e, ok := m.selected()
	if !ok {
		return m, nil
	}
	if e.live() {
		return m.finish(Result{Action: Launch, Plan: launcher.Plan{Project: e.project}})
	}
	m.launch = m.newLaunchForm(e.project)
	m.screen = options
	if m.launch.hasTitle() && m.launch.tracked() {
		return m, m.launch.startNaming() // name the session first
	}
	if m.launch.hasTitle() {
		m.launch.cursor = 1 // nothing is recorded, so start on the options
	}
	return m, nil
}

func (m *Model) viewPicker() string {
	avail := max(m.width-2, 6)
	lines := []string{" " + m.headerLine()}
	if strip := m.liveStrip(); strip != "" {
		lines = append(lines, " "+strip)
	}
	if m.showFilter() {
		kind := frameIdle
		if m.filtered {
			kind = frameFocus
		}
		m.filter.Width = max(avail-7, 4) // prompt and cursor take 3 cells
		lines = append(lines, indent(m.frame("Filter", fmt.Sprintf("%d of %d", len(m.view), len(m.all)), m.filter.View(), avail, filterLines, kind), 1))
	}

	h := m.listHeight() + 2
	split := m.width >= splitMinCols && len(m.view) > 0
	leftW := avail
	if split {
		leftW = clamp(avail*2/5, 38, avail-34)
	}
	kind := frameFocus
	if m.filtered {
		kind = frameIdle
	}
	count := ""
	if len(m.view) > 0 {
		count = fmt.Sprintf("%d of %d", m.cursor+1, len(m.view))
	}
	body := m.frame("Projects", count, m.renderList(leftW-4, split), leftW, h, kind)
	if split {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, m.previewFrame(avail-leftW, h))
	}
	screen := strings.Join(append(lines, indent(body, 1), truncate(m.footer(), m.width)), "\n")
	if m.confirm != "" {
		return m.overlay(screen, m.confirmDialog())
	}
	return screen
}

// previewFrame is the details pane for the selected project.
func (m *Model) previewFrame(width, height int) string {
	e, ok := m.selected()
	if !ok {
		return ""
	}
	title := e.project.Name
	if e.live() {
		title += " ●"
	}
	return m.frame(title, "", m.renderPreview(width-4), width, height, frameIdle)
}

// confirmDialog asks before a running session is closed.
func (m *Model) confirmDialog() string {
	th := m.th
	name := project.New(m.confirm).Name
	body := strings.Join([]string{
		th.Strong.Render(name),
		th.Dim.Render("its windows and everything running in them stop"),
		"",
		m.hints("y", "close", "any key", "cancel"),
	}, "\n")
	return m.frame("Close session?", "", body, clamp(m.width-4, 30, 60), 0, frameBad)
}

// headerLine shows the wordmark with the counts, or the filter while typing.
func (m *Model) headerLine() string {
	th := m.th
	left := th.Accent.Render("▌tmux") + th.Strong.Render("izer")
	right := th.Dim.Render(fmt.Sprintf("%d projects", len(m.view)))
	if live := len(m.liveEntries()); live > 0 {
		right += th.Dim.Render(" · ") + th.Good.Render(fmt.Sprintf("%d live", live))
	}
	if m.scanning {
		right += th.Dim.Render(" · scanning…")
	}
	gap := max(m.width-2-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

// liveStrip lists the running sessions on one line.
func (m *Model) liveStrip() string {
	live := m.liveEntries()
	if len(live) == 0 {
		return ""
	}
	parts := make([]string, len(live))
	for i, e := range live {
		parts[i] = m.th.Good.Render("●") + " " + e.project.Name
	}
	return truncate(m.th.Dim.Render("live  ")+strings.Join(parts, "  "), m.width-2)
}

func (m *Model) renderList(width int, hasPreview bool) string {
	h := m.listHeight()
	if len(m.view) == 0 {
		return m.emptyState()
	}
	end := min(m.offset+h, len(m.view))
	lines := make([]string, 0, h)
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.renderRow(m.view[i], i == m.cursor, width, hasPreview))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) emptyState() string {
	th := m.th
	switch {
	case m.scanning:
		return th.Dim.Render("looking for projects…")
	case m.filter.Value() != "":
		return th.Dim.Render("no match for ") + th.Accent.Render(m.filter.Value())
	}
	return strings.Join([]string{
		th.Strong.Render("no projects found"),
		"",
		th.Dim.Render("add folders to scan in"),
		th.Accent.Render("~/.config/tmuxizer/config.toml"),
		"",
		th.Dim.Render(`roots = ["~/workspace"]`),
	}, "\n")
}

func (m *Model) renderRow(e entry, selected bool, width int, hasPreview bool) string {
	th := m.th
	lang := m.stackOf(e.project.Path).Primary()
	badge := th.Language(lang).Render(pad(shortLang(lang), 3))

	mark := " "
	if e.live() {
		mark = th.Good.Render("●")
	}
	bar, base := " ", lipgloss.NewStyle()
	switch {
	case selected:
		bar, base = th.Accent.Render("▌"), th.Accent
	case e.current:
		base = th.Strong
	}

	nameWidth := width - 2 - 3 - 1 // bar+mark, badge, gap
	if !hasPreview {
		// Without a preview pane, show the location inline instead.
		nameWidth = max(nameWidth/2, 12)
	}
	name := truncate(e.project.Name, nameWidth)
	rendered := highlight(name, e.match, base, base.Underline(true).Foreground(th.Accent.GetForeground()))
	row := bar + mark + badge + " " + rendered + strings.Repeat(" ", max(nameWidth-lipgloss.Width(name), 0))
	if !hasPreview {
		row += " " + th.Dim.Render(truncate(shortPath(e.project.Path), width-lipgloss.Width(row)-1))
	}
	return row
}

func shortLang(l stack.Language) string {
	switch l {
	case stack.TypeScript:
		return "ts"
	case stack.JavaScript:
		return "js"
	case stack.Python:
		return "py"
	case stack.PHP:
		return "php"
	case stack.Go:
		return "go"
	}
	return ""
}

func indent(s string, n int) string {
	prefix := strings.Repeat(" ", n)
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

// footer shows a pending question, a notice, or the key hints.
func (m *Model) footer() string {
	switch {
	case m.notice != "":
		return " " + m.th.Bad.Render(m.notice)
	case m.filtered:
		return m.help("enter", "accept", "esc", "clear", "ctrl+n/p", "move")
	}
	// Say what enter will do: attaching is immediate, launching shows options first.
	enter, live := "launch…", false
	if e, ok := m.selected(); ok && e.live() {
		enter, live = "attach to session", true
	}
	hints := []string{"j/k", "move", "enter", enter, "/", "filter", "s", "time"}
	if len(m.liveEntries()) > 0 {
		hints = append(hints, "←/→", "live sessions")
	}
	if live {
		hints = append(hints, "x", "close session")
	}
	return m.help(append(hints, "t", "shell here", "q", "quit")...)
}

// help renders the key hints as a footer line.
func (m *Model) help(pairs ...string) string { return " " + m.hints(pairs...) }

// hints renders alternating key/description pairs.
func (m *Model) hints(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, m.th.Accent.Render(pairs[i])+" "+m.th.Dim.Render(pairs[i+1]))
	}
	return strings.Join(parts, m.th.Dim.Render(" · "))
}
