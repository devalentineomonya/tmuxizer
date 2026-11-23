package stack

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture writes files (path -> content) into a temp dir and returns it.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		lang      Language
		manager   string
		framework string
		run       string // expected Run.Command, "" for none
	}{
		{
			name: "pnpm next typescript",
			files: map[string]string{
				"package.json":   `{"scripts":{"build":"next build","dev":"next dev"},"dependencies":{"next":"15"},"devDependencies":{"typescript":"5"}}`,
				"pnpm-lock.yaml": "",
			},
			lang: TypeScript, manager: "pnpm", framework: "Next.js", run: "pnpm dev",
		},
		{
			name: "npm javascript uses npm run",
			files: map[string]string{
				"package.json":      `{"scripts":{"dev":"node ."}}`,
				"package-lock.json": "{}",
			},
			lang: JavaScript, manager: "npm", run: "npm run dev",
		},
		{
			name: "npm start has no run keyword",
			files: map[string]string{
				"package.json": `{"scripts":{"start":"node ."}}`,
			},
			lang: JavaScript, manager: "npm", run: "npm start",
		},
		{
			name: "corepack field beats lockfile",
			files: map[string]string{
				"package.json":      `{"packageManager":"yarn@4.1.0","scripts":{"dev":"vite"}}`,
				"package-lock.json": "{}",
			},
			lang: JavaScript, manager: "yarn", run: "yarn dev",
		},
		{
			name: "go with cmd dir",
			files: map[string]string{
				"go.mod":          "module x\n",
				"cmd/app/main.go": "package main",
				"cmd/docs/readme": "not a main package",
				"internal/foo.go": "package foo",
			},
			lang: Go, manager: "go", run: "go run ./cmd/app",
		},
		{
			name:  "go root main",
			files: map[string]string{"go.mod": "module x\n", "main.go": "package main"},
			lang:  Go, manager: "go", run: "go run .",
		},
		{
			name:  "go with air",
			files: map[string]string{"go.mod": "module x\n", "main.go": "package main", ".air.toml": ""},
			lang:  Go, manager: "go", run: "air",
		},
		{
			name: "django with uv",
			files: map[string]string{
				"pyproject.toml": "[project]\nname='x'\n",
				"uv.lock":        "",
				"manage.py":      "",
			},
			lang: Python, manager: "uv", framework: "Django", run: "uv run python manage.py runserver",
		},
		{
			name: "fastapi",
			files: map[string]string{
				"requirements.txt": "fastapi\nuvicorn\n",
				"main.py":          "",
			},
			lang: Python, manager: "pip", framework: "FastAPI", run: "uvicorn main:app --reload",
		},
		{
			name: "laravel",
			files: map[string]string{
				"composer.json": `{"require":{"laravel/framework":"11"},"scripts":{"post-update-cmd":"x","test":"phpunit"}}`,
				"artisan":       "",
			},
			lang: PHP, manager: "composer", framework: "Laravel", run: "php artisan serve",
		},
		{
			name:  "docker only",
			files: map[string]string{"compose.yaml": "services: {}"},
			run:   "docker compose up",
		},
		{
			name: "language run beats compose",
			files: map[string]string{
				"package.json": `{"scripts":{"dev":"vite"}}`,
				"compose.yml":  "services: {}",
			},
			lang: JavaScript, manager: "npm", run: "npm run dev",
		},
		{name: "empty", files: map[string]string{"README.md": "hi"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Detect(fixture(t, tc.files))
			if got := s.Primary(); got != tc.lang {
				t.Errorf("language = %q, want %q", got, tc.lang)
			}
			if s.Manager != tc.manager {
				t.Errorf("manager = %q, want %q", s.Manager, tc.manager)
			}
			if s.Framework != tc.framework {
				t.Errorf("framework = %q, want %q", s.Framework, tc.framework)
			}
			got := ""
			if s.Run != nil {
				got = s.Run.Command
			}
			if got != tc.run {
				t.Errorf("run = %q, want %q", got, tc.run)
			}
		})
	}
}

func TestNodeScriptOrderFollowsFile(t *testing.T) {
	dir := fixture(t, map[string]string{
		"package.json": `{"scripts":{"zeta":"a","alpha":"b","dev":"c"}}`,
	})
	s := Detect(dir)
	var names []string
	for _, sc := range s.Scripts {
		names = append(names, sc.Name)
	}
	want := []string{"zeta", "alpha", "dev"}
	if len(names) != len(want) {
		t.Fatalf("scripts = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("scripts = %v, want %v", names, want)
		}
	}
}

func TestComposerSkipsLifecycleHooks(t *testing.T) {
	s := Detect(fixture(t, map[string]string{
		"composer.json": `{"scripts":{"post-install-cmd":"x","pre-commit":"y","lint":"z"}}`,
	}))
	if len(s.Scripts) != 1 || s.Scripts[0].Name != "lint" {
		t.Fatalf("scripts = %+v, want only lint", s.Scripts)
	}
}

func TestMakefileTargets(t *testing.T) {
	s := Detect(fixture(t, map[string]string{
		"Makefile": "CC := gcc\n.PHONY: build\nbuild:\n\tgo build\ntest: build\n\tgo test\nbuild:\n",
	}))
	var got []string
	for _, sc := range s.Scripts {
		got = append(got, sc.Command)
	}
	want := []string{"make build", "make test"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

func TestRecognised(t *testing.T) {
	if !Recognised(fixture(t, map[string]string{"go.mod": "module x"})) {
		t.Error("go.mod should be recognised")
	}
	if Recognised(fixture(t, map[string]string{"notes.txt": ""})) {
		t.Error("plain dir should not be recognised")
	}
}
