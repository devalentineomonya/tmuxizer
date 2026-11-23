package iwashere

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Duration reads iwashere's total_time, which older files store as
// nanoseconds and newer ones as a Go duration string such as "1h2m3s".
type Duration time.Duration

// UnmarshalJSON accepts both encodings.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case float64:
		*d = Duration(time.Duration(val))
	case string:
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		*d = Duration(parsed)
	case nil:
		*d = 0
	default:
		return fmt.Errorf("invalid duration %v", v)
	}
	return nil
}

// Elapsed is the time worked so far. iwashere adds a segment to TotalTime
// only when a session pauses or ends, so a running one adds the time since
// it last started.
func (s Session) Elapsed(now time.Time) time.Duration {
	d := time.Duration(s.TotalTime)
	if (s.State == ongoing || s.State == continued) && !s.StartTime.IsZero() {
		d += max(now.Sub(s.StartTime), 0)
	}
	return d
}

// Anchor is the moment a session counts towards for "today" and "this week":
// now while it runs, otherwise when it stopped.
func (s Session) Anchor(now time.Time) time.Time {
	switch {
	case s.State == ongoing || s.State == continued:
		return now
	case !s.EndTime.IsZero():
		return s.EndTime
	}
	return s.StartTime
}

// Sessions returns every session recorded in dir's iwashere store.
func Sessions(dir string) []Session {
	files, err := filepath.Glob(filepath.Join(dir, dirName, "sessions", "*.json"))
	if err != nil {
		return nil
	}
	var out []Session
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(raw, &s) != nil || s.ID == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
