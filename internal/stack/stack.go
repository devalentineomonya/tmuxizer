// Package stack detects a project's language, tooling and runnable scripts.
package stack

import (
	"os"
	"path/filepath"
)

// Language is a supported project language.
type Language string

// The supported languages.
const (
	Go         Language = "Go"
	TypeScript Language = "TypeScript"
	JavaScript Language = "JavaScript"
	Python     Language = "Python"
	PHP        Language = "PHP"
)

// Script is a command the project can run.
type Script struct {
	Name    string // short label, e.g. "dev"
	Command string // full shell command, e.g. "pnpm dev"
	Source  string // where it was found, e.g. "package.json"
}

// Stack describes what a directory contains.
type Stack struct {
	Languages []Language // primary language first
	Manager   string     // package manager or build tool, e.g. "pnpm"
	Framework string     // best-effort, e.g. "Next.js"
	Docker    bool       // Dockerfile or compose file present
	Scripts   []Script   // every runnable script found
	Run       *Script    // predicted "start the app" command, nil if unknown
}

// Primary returns the main language, or "" when none was detected.
func (s Stack) Primary() Language {
	if len(s.Languages) == 0 {
		return ""
	}
	return s.Languages[0]
}

// detector inspects dir and fills in s.
type detector func(dir string, s *Stack)

// detectors run in order; earlier ones win the primary language and Run.
var detectors = []detector{
	detectNode,
	detectGo,
	detectPython,
	detectPHP,
	detectDocker,
	detectMakefile,
}

// Detect inspects dir and returns what it found. It never fails: an
// unreadable or unrecognised directory yields an empty Stack.
func Detect(dir string) Stack {
	var s Stack
	for _, d := range detectors {
		d(dir, &s)
	}
	return s
}

// Recognised reports whether dir carries a marker for a supported language.
func Recognised(dir string) bool {
	for _, m := range []string{"package.json", "go.mod", "pyproject.toml", "requirements.txt", "composer.json"} {
		if exists(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}

func (s *Stack) addLanguage(l Language) {
	for _, have := range s.Languages {
		if have == l {
			return
		}
	}
	s.Languages = append(s.Languages, l)
}

// setRun records the predicted start command unless one is already set.
func (s *Stack) setRun(sc Script) {
	if s.Run == nil {
		s.Run = &sc
	}
}

func (s *Stack) setManager(m string) {
	if s.Manager == "" {
		s.Manager = m
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}
