// Package theme defines tmuxizer's look.
//
// Colours are ANSI palette indexes (0-15), never fixed RGB, so the terminal's
// own theme (kitty, alacritty, ghostty, a tmux theme...) recolours the UI.
// The background is never painted; it stays whatever the terminal uses.
package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devalentineomonya/tmuxizer/internal/stack"
)

// Theme holds the styles shared by every screen.
type Theme struct {
	Accent  lipgloss.Style // selection bar, titles, key hints
	Dim     lipgloss.Style // secondary text
	Strong  lipgloss.Style // names and values
	Good    lipgloss.Style
	Info    lipgloss.Style
	Warn    lipgloss.Style
	Bad     lipgloss.Style
	Border  lipgloss.Style // panel borders
	Focus   lipgloss.Style // border of the focused pane or input
	accent  lipgloss.TerminalColor
	palette map[stack.Language]lipgloss.TerminalColor
}

// New builds a Theme. accent is an ANSI index ("0"-"15"), a colour name, or
// "#rrggbb"; empty selects the default.
func New(accent string) Theme {
	ac := parseColor(accent)
	fg := func(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return Theme{
		Accent: fg(ac).Bold(true),
		Dim:    fg(lipgloss.Color("8")),
		Strong: lipgloss.NewStyle().Bold(true),
		Good:   fg(lipgloss.Color("2")),
		Info:   fg(lipgloss.Color("6")),
		Warn:   fg(lipgloss.Color("3")),
		Bad:    fg(lipgloss.Color("1")),
		Border: fg(lipgloss.Color("8")),
		Focus:  fg(ac),
		accent: ac,
		palette: map[stack.Language]lipgloss.TerminalColor{
			stack.Go:         lipgloss.Color("6"),
			stack.TypeScript: lipgloss.Color("4"),
			stack.JavaScript: lipgloss.Color("3"),
			stack.Python:     lipgloss.Color("2"),
			stack.PHP:        lipgloss.Color("5"),
		},
	}
}

// Language styles a language badge in its own colour.
func (t Theme) Language(l stack.Language) lipgloss.Style {
	c, ok := t.palette[l]
	if !ok {
		return t.Dim
	}
	return lipgloss.NewStyle().Foreground(c)
}

// Panel wraps content in a rounded, dim border.
func (t Theme) Panel(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(width)
}

var ansiNames = map[string]string{
	"black": "0", "red": "1", "green": "2", "yellow": "3",
	"blue": "4", "magenta": "5", "cyan": "6", "white": "7",
}

func parseColor(s string) lipgloss.TerminalColor {
	s = strings.ToLower(strings.TrimSpace(s))
	if idx, ok := ansiNames[s]; ok {
		return lipgloss.Color(idx)
	}
	if s == "" {
		return lipgloss.Color("5")
	}
	return lipgloss.Color(s)
}
