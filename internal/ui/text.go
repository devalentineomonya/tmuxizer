package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// truncate shortens s to width cells, adding an ellipsis when it cuts.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// pad right-pads s with spaces to width cells.
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// padLeft left-pads s with spaces to width cells.
func padLeft(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// shortPath replaces the home directory prefix with "~".
func shortPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if p == home {
			return "~"
		}
		if rest, ok := strings.CutPrefix(p, home+"/"); ok {
			return "~/" + rest
		}
	}
	return p
}

// ago renders a time as a short relative phrase.
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case t.IsZero():
		return "never"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2 2006")
	}
}

// fuzzyMatch finds query in s as an in-order, case-insensitive subsequence
// and returns the matched rune positions. An empty query matches everything.
func fuzzyMatch(query, s string) ([]int, bool) {
	q := []rune(query)
	if len(q) == 0 {
		return nil, true
	}
	pos := make([]int, 0, len(q))
	i := 0
	for idx, r := range []rune(s) {
		if unicode.ToLower(r) == unicode.ToLower(q[i]) {
			pos = append(pos, idx)
			if i++; i == len(q) {
				return pos, true
			}
		}
	}
	return nil, false
}

// highlight renders s in base, with the runes at pos in hl.
func highlight(s string, pos []int, base, hl lipgloss.Style) string {
	if len(pos) == 0 {
		return base.Render(s)
	}
	marked := make(map[int]bool, len(pos))
	for _, p := range pos {
		marked[p] = true
	}
	var b strings.Builder
	runes := []rune(s)
	for start := 0; start < len(runes); {
		end := start
		for end < len(runes) && marked[end] == marked[start] {
			end++
		}
		style := base
		if marked[start] {
			style = hl
		}
		b.WriteString(style.Render(string(runes[start:end])))
		start = end
	}
	return b.String()
}

// clamp limits v to [lo, hi].
func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
