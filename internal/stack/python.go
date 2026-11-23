package stack

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	scriptTableRe = regexp.MustCompile(`(?m)^\[(?:project\.scripts|tool\.poetry\.scripts)\]\s*$`)
	tomlKeyRe     = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*=`)
)

func detectPython(dir string, s *Stack) {
	pyproject := readFile(filepath.Join(dir, "pyproject.toml"))
	hasReqs := exists(filepath.Join(dir, "requirements.txt"))
	if pyproject == nil && !hasReqs && !exists(filepath.Join(dir, "Pipfile")) {
		return
	}
	s.addLanguage(Python)

	runner := pythonRunner(dir, string(pyproject))
	s.setManager(runner.manager)
	deps := strings.ToLower(string(pyproject) + string(readFile(filepath.Join(dir, "requirements.txt"))))

	switch {
	case exists(filepath.Join(dir, "manage.py")):
		s.Framework = "Django"
		sc := Script{Name: "runserver", Command: runner.prefix + "python manage.py runserver", Source: "manage.py"}
		s.Scripts = append(s.Scripts, sc)
		s.setRun(sc)
	case strings.Contains(deps, "fastapi"):
		s.Framework = "FastAPI"
		if target := asgiTarget(dir); target != "" {
			sc := Script{Name: "uvicorn", Command: runner.prefix + "uvicorn " + target + " --reload", Source: "fastapi"}
			s.Scripts = append(s.Scripts, sc)
			s.setRun(sc)
		}
	case strings.Contains(deps, "flask"):
		s.Framework = "Flask"
		sc := Script{Name: "flask", Command: runner.prefix + "flask run --debug", Source: "flask"}
		s.Scripts = append(s.Scripts, sc)
		s.setRun(sc)
	}

	for _, name := range tomlScripts(string(pyproject)) {
		s.Scripts = append(s.Scripts, Script{Name: name, Command: runner.prefix + name, Source: "pyproject.toml"})
	}
	for _, entry := range []string{"main.py", "app.py", "__main__.py"} {
		if exists(filepath.Join(dir, entry)) {
			sc := Script{Name: entry, Command: runner.prefix + "python " + entry, Source: entry}
			s.Scripts = append(s.Scripts, sc)
			s.setRun(sc)
			break
		}
	}
	if strings.Contains(deps, "pytest") {
		s.Scripts = append(s.Scripts, Script{Name: "test", Command: runner.prefix + "pytest", Source: "pytest"})
	}
}

type pyRunner struct {
	manager string
	prefix  string // command prefix that activates the environment
}

func pythonRunner(dir, pyproject string) pyRunner {
	switch {
	case exists(filepath.Join(dir, "uv.lock")):
		return pyRunner{"uv", "uv run "}
	case exists(filepath.Join(dir, "poetry.lock")) || strings.Contains(pyproject, "[tool.poetry]"):
		return pyRunner{"poetry", "poetry run "}
	case exists(filepath.Join(dir, "pdm.lock")):
		return pyRunner{"pdm", "pdm run "}
	case exists(filepath.Join(dir, "Pipfile")):
		return pyRunner{"pipenv", "pipenv run "}
	default:
		return pyRunner{"pip", ""}
	}
}

// asgiTarget finds the "module:app" string for uvicorn.
func asgiTarget(dir string) string {
	candidates := []struct{ file, target string }{
		{"main.py", "main:app"},
		{"app.py", "app:app"},
		{"app/main.py", "app.main:app"},
		{"src/main.py", "src.main:app"},
	}
	for _, c := range candidates {
		if exists(filepath.Join(dir, c.file)) {
			return c.target
		}
	}
	return ""
}

// tomlScripts reads keys from [project.scripts] / [tool.poetry.scripts].
func tomlScripts(pyproject string) []string {
	loc := scriptTableRe.FindStringIndex(pyproject)
	if loc == nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(pyproject[loc[1]:], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			break
		}
		if m := tomlKeyRe.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}
