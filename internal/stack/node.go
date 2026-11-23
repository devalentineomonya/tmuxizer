package stack

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type packageJSON struct {
	PackageManager  string            `json:"packageManager"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// startScripts are tried in order to predict the "run the app" script.
var startScripts = []string{"dev", "start:dev", "develop", "serve", "start"}

var nodeFrameworks = []struct{ dep, name string }{
	{"next", "Next.js"},
	{"nuxt", "Nuxt"},
	{"@nestjs/core", "NestJS"},
	{"@sveltejs/kit", "SvelteKit"},
	{"astro", "Astro"},
	{"@remix-run/react", "Remix"},
	{"vite", "Vite"},
	{"expo", "Expo"},
	{"express", "Express"},
	{"hono", "Hono"},
}

func detectNode(dir string, s *Stack) {
	raw := readFile(filepath.Join(dir, "package.json"))
	if raw == nil {
		return
	}
	var pkg packageJSON
	if err := json.Unmarshal(raw, &pkg); err != nil {
		// Still a node project, just without scripts.
		s.addLanguage(JavaScript)
		return
	}

	if exists(filepath.Join(dir, "tsconfig.json")) || hasDep(pkg, "typescript") {
		s.addLanguage(TypeScript)
	} else {
		s.addLanguage(JavaScript)
	}

	pm := nodeManager(dir, pkg)
	s.setManager(pm)
	s.Framework = firstFramework(pkg)

	for _, name := range sortedScriptNames(raw, pkg.Scripts) {
		s.Scripts = append(s.Scripts, Script{
			Name:    name,
			Command: runScript(pm, name),
			Source:  "package.json",
		})
	}
	for _, name := range startScripts {
		if _, ok := pkg.Scripts[name]; ok {
			s.setRun(Script{Name: name, Command: runScript(pm, name), Source: "package.json"})
			break
		}
	}
}

// nodeManager picks the package manager from the corepack field, then lockfiles.
func nodeManager(dir string, pkg packageJSON) string {
	if name, _, ok := strings.Cut(pkg.PackageManager, "@"); ok && name != "" {
		return name
	}
	lockfiles := []struct{ file, pm string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"bun.lock", "bun"},
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, l := range lockfiles {
		if exists(filepath.Join(dir, l.file)) {
			return l.pm
		}
	}
	return "npm"
}

// runScript renders the idiomatic invocation for each package manager.
func runScript(pm, name string) string {
	switch {
	case pm == "npm" && name != "start" && name != "test":
		return "npm run " + name
	case pm == "npm":
		return "npm " + name
	case pm == "bun":
		return "bun run " + name
	default: // pnpm and yarn run scripts directly
		return pm + " " + name
	}
}

func hasDep(pkg packageJSON, name string) bool {
	_, dev := pkg.DevDependencies[name]
	_, prod := pkg.Dependencies[name]
	return dev || prod
}

func firstFramework(pkg packageJSON) string {
	for _, f := range nodeFrameworks {
		if hasDep(pkg, f.dep) {
			return f.name
		}
	}
	return ""
}

// sortedScriptNames returns script names in the order they appear in the file,
// since encoding/json maps lose ordering.
func sortedScriptNames(raw []byte, scripts map[string]string) []string {
	var doc struct {
		Scripts json.RawMessage `json:"scripts"`
	}
	if json.Unmarshal(raw, &doc) != nil || len(doc.Scripts) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(doc.Scripts)))
	if _, err := dec.Token(); err != nil { // opening brace
		return nil
	}
	var names []string
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			break
		}
		name, _ := key.(string)
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			break
		}
		if _, ok := scripts[name]; ok {
			names = append(names, name)
		}
	}
	return names
}
