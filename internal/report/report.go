// Package report totals the time recorded by iwashere sessions.
package report

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/devalentineomonya/tmuxizer/internal/iwashere"
	"github.com/devalentineomonya/tmuxizer/internal/project"
)

// Period is a reporting window.
type Period int

const (
	// Today is since local midnight.
	Today Period = iota
	// Week is since Monday, local time.
	Week
	// All is everything recorded.
	All
)

// Periods lists the windows in display order.
var Periods = []Period{Today, Week, All}

func (p Period) String() string {
	return [...]string{"today", "this week", "all time"}[p]
}

// Parse maps a command-line word to a period.
func Parse(s string) (Period, bool) {
	switch s {
	case "today", "day":
		return Today, true
	case "week", "":
		return Week, true
	case "all", "total":
		return All, true
	}
	return Week, false
}

// Since returns the start of the period; the zero time for All.
func (p Period) Since(now time.Time) time.Time {
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch p {
	case Today:
		return midnight
	case Week:
		back := (int(now.Weekday()) + 6) % 7 // Monday is day 0
		return midnight.AddDate(0, 0, -back)
	}
	return time.Time{}
}

// Summary is one project's time in each window.
type Summary struct {
	Today, Week, All time.Duration
	Sessions         int // sessions with recorded time
}

// In returns the time for a period.
func (s Summary) In(p Period) time.Duration {
	switch p {
	case Today:
		return s.Today
	case Week:
		return s.Week
	}
	return s.All
}

// Summarize totals a project's iwashere sessions. A session's whole time
// counts towards the day it stopped, or today while it is still running.
func Summarize(dir string, now time.Time) Summary {
	var sum Summary
	today, week := Today.Since(now), Week.Since(now)
	for _, s := range iwashere.Sessions(dir) {
		d := s.Elapsed(now)
		if d <= 0 {
			continue
		}
		sum.Sessions++
		sum.All += d
		if at := s.Anchor(now); !at.Before(week) {
			sum.Week += d
			if !at.Before(today) {
				sum.Today += d
			}
		}
	}
	return sum
}

// Row is one project in a report.
type Row struct {
	Project  project.Project
	Time     time.Duration
	Sessions int
}

// Report is the time per project for one period, longest first.
type Report struct {
	Period Period
	Rows   []Row
	Total  time.Duration
}

// Build totals the period for every project that has time in it.
func Build(projects []project.Project, p Period, now time.Time) Report {
	rep := Report{Period: p}
	for _, proj := range projects {
		sum := Summarize(proj.Path, now)
		if d := sum.In(p); d > 0 {
			rep.Rows = append(rep.Rows, Row{Project: proj, Time: d, Sessions: sum.Sessions})
			rep.Total += d
		}
	}
	slices.SortFunc(rep.Rows, func(a, b Row) int {
		if a.Time != b.Time {
			return int(b.Time - a.Time)
		}
		return strings.Compare(a.Project.Name, b.Project.Name)
	})
	return rep
}

// Human renders a duration as "2h 14m", "35m" or "<1m".
func Human(d time.Duration) string {
	d = d.Round(time.Minute)
	switch {
	case d <= 0:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
