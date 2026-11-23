package stack

import (
	"path/filepath"
	"regexp"
	"strings"
)

var makeTargetRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]*)\s*:($|[^=])`)

func detectMakefile(dir string, s *Stack) {
	raw := readFile(filepath.Join(dir, "Makefile"))
	if raw == nil {
		return
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		m := makeTargetRe.FindStringSubmatch(line)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		s.Scripts = append(s.Scripts, Script{Name: m[1], Command: "make " + m[1], Source: "Makefile"})
	}
}
