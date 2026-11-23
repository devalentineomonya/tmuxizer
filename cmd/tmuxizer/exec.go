package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// execIn runs a command in dir (or the current directory when empty) and
// returns its combined output; failures carry that output.
func execIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return text, nil
}

// runIn is execIn for callers that only care whether it worked.
func runIn(dir, name string, args ...string) error {
	_, err := execIn(dir, name, args...)
	return err
}
