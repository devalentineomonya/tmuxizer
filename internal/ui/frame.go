package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// frameKind picks a box's colours: dim when idle, accent when it has focus,
// red when it holds a mistake.
type frameKind int

const (
	frameIdle frameKind = iota
	frameFocus
	frameBad
)

// frame draws a rounded box with title set into its top border and an
// optional right-aligned label, like lazygit's panes. width and height are
// outer sizes; height 0 fits the body. Colours are theme ANSI styles only.
func (m *Model) frame(title, right, body string, width, height int, kind frameKind) string {
	th := m.th
	width = max(width, 6)
	border, label := th.Border, th.Dim
	switch kind {
	case frameFocus:
		border, label = th.Focus, th.Accent
	case frameBad:
		border, label = th.Bad, th.Bad
	}
	inner := width - 4 // border and one cell of padding on each side

	lines := strings.Split(body, "\n")
	if height > 0 {
		want := max(height-2, 0)
		for len(lines) < want {
			lines = append(lines, "")
		}
		lines = lines[:want]
	}

	side := func(s string, st lipgloss.Style, limit int) string {
		if s == "" || limit < 3 {
			return ""
		}
		return " " + st.Render(truncate(s, limit-2)) + " "
	}
	t := side(title, label, width-6)
	r := side(right, th.Dim, width-6-lipgloss.Width(t))
	trail := 0
	if r != "" {
		trail = 1
	}
	fill := max(width-2-1-lipgloss.Width(t)-lipgloss.Width(r)-trail, 0)
	var b strings.Builder
	b.WriteString(border.Render("╭─") + t + border.Render(strings.Repeat("─", fill)) + r +
		border.Render(strings.Repeat("─", trail)+"╮") + "\n")
	for _, l := range lines {
		b.WriteString(border.Render("│") + " " + pad(truncate(l, inner), inner) + " " + border.Render("│") + "\n")
	}
	b.WriteString(border.Render("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

// overlay centres fg on bg. A terminal cannot blur, so bg is stripped of
// colour and faded, and the dialog gets a drop shadow to lift it off.
func (m *Model) overlay(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < m.height {
		bgLines = append(bgLines, "")
	}
	bgLines = bgLines[:m.height]
	fgLines := strings.Split(fg, "\n")
	fw := 0
	for _, l := range fgLines {
		fw = max(fw, lipgloss.Width(l))
	}
	x := max((m.width-fw)/2, 0)
	y := max((m.height-len(fgLines))/2, 0)

	fade := m.th.Dim.Faint(true)
	shadow := m.th.Border.Render
	splice := func(plain string, at int, ins string) string {
		return fade.Render(ansi.Truncate(plain, at, "")) + ins + fade.Render(ansi.TruncateLeft(plain, at+lipgloss.Width(ins), ""))
	}

	out := make([]string, len(bgLines))
	for i, l := range bgLines {
		plain := pad(ansi.Strip(l), m.width)
		switch k := i - y; {
		case k >= 0 && k < len(fgLines):
			row := pad(fgLines[k], fw)
			if k > 0 { // the shadow starts one row down
				row += shadow("░░")
			}
			out[i] = splice(plain, x, row)
		case k == len(fgLines):
			out[i] = splice(plain, x+2, shadow(strings.Repeat("░", fw)))
		default:
			out[i] = fade.Render(plain)
		}
	}
	for i := range out {
		out[i] = ansi.Truncate(out[i], m.width, "") // the shadow must not wrap
	}
	return strings.Join(out, "\n")
}

// window keeps lines[cursor] visible when only height lines fit.
func window(lines []string, cursor, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	start := clamp(cursor-height/2, 0, len(lines)-height)
	return lines[start : start+height]
}
