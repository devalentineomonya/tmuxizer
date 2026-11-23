package ui

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/report"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
)

type statsMsg report.Report

// RenderReport draws time per project as rows with proportional bars. The
// time screen and `tmuxizer time` share it.
func RenderReport(th theme.Theme, rep report.Report, width int) string {
	if len(rep.Rows) == 0 {
		return th.Dim.Render("  no time recorded " + rep.Period.String() + "\n  open a project with iwashere on to start tracking")
	}

	nameW, timeW, sessW := 8, 0, 0
	for _, r := range rep.Rows {
		nameW = max(nameW, lipgloss.Width(r.Project.Name))
		timeW = max(timeW, len(report.Human(r.Time)))
		sessW = max(sessW, len(sessions(r.Sessions)))
	}
	nameW = min(nameW, 24)
	timeW = max(timeW, len(report.Human(rep.Total)))
	barW := clamp(width-2-nameW-timeW-sessW-6, 6, 40)

	var b strings.Builder
	top := rep.Rows[0].Time
	for _, r := range rep.Rows {
		filled := clamp(int(math.Round(float64(r.Time)/float64(top)*float64(barW))), 1, barW)
		bar := th.Accent.Render(strings.Repeat("█", filled)) + th.Dim.Render(strings.Repeat("░", barW-filled))
		b.WriteString("  " + pad(truncate(r.Project.Name, nameW), nameW) + "  " + bar + "  " +
			padLeft(th.Strong.Render(report.Human(r.Time)), timeW) + "  " + th.Dim.Render(sessions(r.Sessions)) + "\n")
	}
	b.WriteString("\n  " + th.Dim.Render(pad("total", nameW)) + "  " + strings.Repeat(" ", barW) + "  " +
		padLeft(th.Accent.Render(report.Human(rep.Total)), timeW))
	return b.String()
}

func sessions(n int) string {
	if n == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

// statsCmd totals the current period in the background.
func (m *Model) statsCmd() tea.Cmd {
	projects, period, now := m.projects(), m.statsPeriod, m.deps.Now()
	return func() tea.Msg { return statsMsg(report.Build(projects, period, now)) }
}

func (m *Model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := 0
	switch msg.String() {
	case "l", "right", "tab":
		step = 1
	case "h", "left", "shift+tab":
		step = -1
	case "esc", "q", "s":
		m.screen = picker
		return m, nil
	}
	if step == 0 {
		return m, nil
	}
	n := len(report.Periods)
	m.statsPeriod = report.Periods[(int(m.statsPeriod)+step+n)%n]
	m.statsReady = false
	return m, m.statsCmd()
}

func (m *Model) viewStats() string {
	th := m.th
	tabs := make([]string, len(report.Periods))
	for i, p := range report.Periods {
		if p == m.statsPeriod {
			tabs[i] = th.Accent.Render(p.String())
		} else {
			tabs[i] = th.Dim.Render(p.String())
		}
	}
	width := max(m.width-2, 6)
	body := th.Dim.Render("adding it up…")
	if m.statsReady {
		body = RenderReport(th, m.stats, width-4)
	}
	box := m.frame("Time spent", strings.Join(tabs, "  "), body, width, max(m.height-1, 3), frameFocus)
	return indent(box, 1) + "\n" + m.help("h/l", "period", "esc", "back")
}
