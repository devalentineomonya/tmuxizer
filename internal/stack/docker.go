package stack

import (
	"path/filepath"
	"regexp"
)

var composeFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

func detectDocker(dir string, s *Stack) {
	hasCompose := false
	for _, f := range composeFiles {
		if exists(filepath.Join(dir, f)) {
			hasCompose = true
			break
		}
	}
	hasDockerfile := exists(filepath.Join(dir, "Dockerfile"))
	if !hasCompose && !hasDockerfile {
		return
	}
	s.Docker = true

	if hasCompose {
		up := Script{Name: "compose up", Command: "docker compose up", Source: "compose"}
		s.Scripts = append(s.Scripts,
			up,
			Script{Name: "compose up -d", Command: "docker compose up -d", Source: "compose"},
			Script{Name: "compose down", Command: "docker compose down", Source: "compose"},
			Script{Name: "compose logs", Command: "docker compose logs -f", Source: "compose"},
		)
		s.setRun(up) // only wins for projects with no language run command
		return
	}
	s.Scripts = append(s.Scripts, Script{
		Name:    "docker build",
		Command: "docker build -t " + imageName(dir) + " .",
		Source:  "Dockerfile",
	})
}

var nonImageChars = regexp.MustCompile(`[^a-z0-9_.-]+`)

func imageName(dir string) string {
	name := nonImageChars.ReplaceAllString(filepath.Base(dir), "-")
	if name == "" {
		return "app"
	}
	return name
}
