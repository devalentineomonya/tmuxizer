package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
)

func (m *Model) updateResume(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.orphans) == 0 {
		m.screen = picker
		return m, nil
	}
	o := m.orphans[m.orphan]
	switch msg.String() {
	case "j", "down":
		m.orphan = min(m.orphan+1, len(m.orphans)-1)
	case "k", "up":
		m.orphan = max(m.orphan-1, 0)
	case "r", "enter":
		return m.finish(Result{Action: Launch, Plan: m.planFromPrefs(o.Project, o.Prefs, launcher.Resume)})
	case "n":
		return m.finish(Result{Action: Launch, Plan: m.planFromPrefs(o.Project, o.Prefs, launcher.Fresh)})
	case "d":
		m.orphans = append(m.orphans[:m.orphan], m.orphans[m.orphan+1:]...)
		m.orphan = clamp(m.orphan, 0, max(len(m.orphans)-1, 0))
		if len(m.orphans) == 0 {
			m.screen = picker
		}
		dismiss := m.deps.Dismiss
		return m, func() tea.Msg {
			if dismiss == nil {
				return nil
			}
			return dismissMsg{err: dismiss(o)}
		}
	case "s", "esc", "q":
		m.screen = picker
	}
	return m, nil
}

func (m *Model) viewResume() string {
	th := m.th
	var b strings.Builder
	b.WriteString(" " + th.Accent.Render("welcome back") + "\n")
	b.WriteString(" " + th.Dim.Render("these sessions did not end cleanly") + "\n\n")

	for i, o := range m.orphans {
		bar, name := " ", th.Strong.Render(o.Project.Name)
		if i == m.orphan {
			bar, name = th.Accent.Render("▌"), th.Accent.Render(o.Project.Name)
		}
		detail := "started " + ago(o.Started, m.deps.Now())
		if o.Work != nil {
			detail += " · " + truncate(o.Work.Title, 40)
		}
		b.WriteString(" " + bar + " " + pad(name, 24) + th.Dim.Render(detail) + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n " + th.Bad.Render(m.notice) + "\n")
	}
	b.WriteString("\n" + m.help("r", "resume", "n", "start new", "d", "dismiss", "s", "skip"))
	return "\n" + b.String()
}
