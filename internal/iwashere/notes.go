package iwashere

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Note mirrors the fields of iwashere's note files that we read.
type Note struct {
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Branch    string    `json:"branch"`
}

// LastNote returns the most recently written note in dir. Only the newest
// file is parsed, so projects with many notes stay cheap.
func LastNote(dir string) (Note, bool) {
	files, err := filepath.Glob(filepath.Join(dir, dirName, "notes", "*.json"))
	if err != nil || len(files) == 0 {
		return Note{}, false
	}

	var newest string
	var newestTime time.Time
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest, newestTime = f, info.ModTime()
		}
	}
	raw, err := os.ReadFile(newest)
	if err != nil {
		return Note{}, false
	}
	var n Note
	if json.Unmarshal(raw, &n) != nil || n.Message == "" {
		return Note{}, false
	}
	return n, true
}
