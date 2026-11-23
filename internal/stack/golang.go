package stack

import (
	"os"
	"path/filepath"
	"sort"
)

func detectGo(dir string, s *Stack) {
	if !exists(filepath.Join(dir, "go.mod")) {
		return
	}
	s.addLanguage(Go)
	s.setManager("go")

	var mains []Script
	if exists(filepath.Join(dir, "main.go")) {
		mains = append(mains, Script{Name: "run", Command: "go run .", Source: "go.mod"})
	}
	for _, name := range cmdDirs(dir) {
		mains = append(mains, Script{Name: "run " + name, Command: "go run ./cmd/" + name, Source: "cmd/"})
	}

	// Live reload wins when the project is set up for it.
	if exists(filepath.Join(dir, ".air.toml")) {
		air := Script{Name: "air", Command: "air", Source: ".air.toml"}
		s.Scripts = append(s.Scripts, air)
		s.setRun(air)
	}
	s.Scripts = append(s.Scripts, mains...)
	if len(mains) > 0 {
		s.setRun(mains[0])
	}
	s.Scripts = append(s.Scripts,
		Script{Name: "test", Command: "go test ./...", Source: "go.mod"},
		Script{Name: "build", Command: "go build ./...", Source: "go.mod"},
		Script{Name: "vet", Command: "go vet ./...", Source: "go.mod"},
	)
}

// cmdDirs lists cmd/* subdirectories that hold a main.go.
func cmdDirs(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, "cmd"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && exists(filepath.Join(dir, "cmd", e.Name(), "main.go")) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
