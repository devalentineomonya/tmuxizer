package stack

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

type composerJSON struct {
	Require map[string]string `json:"require"`
	Scripts map[string]any    `json:"scripts"`
}

func detectPHP(dir string, s *Stack) {
	raw := readFile(filepath.Join(dir, "composer.json"))
	if raw == nil {
		return
	}
	var c composerJSON
	_ = json.Unmarshal(raw, &c) // a broken composer.json is still a PHP project

	s.addLanguage(PHP)
	s.setManager("composer")

	switch {
	case exists(filepath.Join(dir, "artisan")):
		s.Framework = "Laravel"
		sc := Script{Name: "serve", Command: "php artisan serve", Source: "artisan"}
		s.Scripts = append(s.Scripts, sc)
		s.setRun(sc)
	case c.Require["symfony/framework-bundle"] != "":
		s.Framework = "Symfony"
	}

	names := make([]string, 0, len(c.Scripts))
	for name := range c.Scripts {
		// Composer lifecycle hooks are not meant to be run by hand.
		if !strings.HasPrefix(name, "pre-") && !strings.HasPrefix(name, "post-") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		s.Scripts = append(s.Scripts, Script{Name: name, Command: "composer run " + name, Source: "composer.json"})
	}
	for _, name := range startScripts {
		if _, ok := c.Scripts[name]; ok {
			s.setRun(Script{Name: name, Command: "composer run " + name, Source: "composer.json"})
			break
		}
	}

	// Plain PHP app with a web root: use the built-in server.
	if exists(filepath.Join(dir, "public", "index.php")) {
		sc := Script{Name: "serve", Command: "php -S localhost:8000 -t public", Source: "public/"}
		s.Scripts = append(s.Scripts, sc)
		s.setRun(sc)
	}
}
