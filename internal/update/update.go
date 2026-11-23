// Package update rebuilds or reinstalls tmuxizer.
package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Updater replaces the running binary with a newer build.
type Updater struct {
	Exe     string // path of the running binary
	Source  string // checkout it was built from, "" when installed another way
	Module  string // module path, for `go install` when there is no checkout
	Version string // version being replaced
	Out     io.Writer

	// Run executes a command and returns its output. dir may be empty.
	Run func(dir, name string, args ...string) (string, error)
	// Rename swaps files; os.Rename unless a test replaces it.
	Rename func(oldPath, newPath string) error
}

// Ldflags are the linker flags that stamp a build with its version and the
// checkout it came from; the Makefile and install.sh use the same ones.
func Ldflags(version, source string) string {
	return fmt.Sprintf("-s -w -X main.version=%s -X main.source=%s", version, source)
}

// Update pulls the checkout (when it tracks a remote) and rebuilds over the
// running binary; without a checkout it runs `go install`.
func (u Updater) Update() error {
	if u.Source == "" {
		return u.goInstall()
	}
	if _, err := os.Stat(filepath.Join(u.Source, "go.mod")); err != nil {
		return fmt.Errorf("the source checkout %s is gone; reinstall with `make install`", u.Source)
	}
	say := func(format string, a ...any) { fmt.Fprintf(u.Out, format+"\n", a...) }

	say("updating from %s", u.Source)
	if _, err := u.Run(u.Source, "git", "rev-parse", "--abbrev-ref", "@{u}"); err == nil {
		out, err := u.Run(u.Source, "git", "pull", "--ff-only")
		if err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
		say("  %s", lastLine(out))
	} else {
		say("  no remote branch is tracked, rebuilding the local checkout")
	}

	version, err := u.Run(u.Source, "git", "describe", "--tags", "--always", "--dirty")
	if err != nil || strings.TrimSpace(version) == "" {
		version = "dev"
	}
	version = strings.TrimSpace(version)

	next := u.Exe + ".new"
	defer func() { _ = os.Remove(next) }() // gone after a successful rename
	if _, err := u.Run(u.Source, "go", "build", "-ldflags", Ldflags(version, u.Source), "-o", next, "./cmd/tmuxizer"); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	rename := u.Rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(next, u.Exe); err != nil {
		return fmt.Errorf("replace %s: %w", u.Exe, err)
	}
	if version == u.Version {
		say("✓ rebuilt %s (already the latest)", version)
	} else {
		say("✓ updated %s → %s", u.Version, version)
	}
	return nil
}

// Check fetches the remote and reports how many commits the checkout is behind.
func (u Updater) Check() (behind int, err error) {
	if u.Source == "" {
		return 0, errors.New("not built from a checkout; run `tmuxizer update` to install the latest release")
	}
	if _, err := u.Run(u.Source, "git", "fetch", "--quiet"); err != nil {
		return 0, fmt.Errorf("git fetch: %w", err)
	}
	out, err := u.Run(u.Source, "git", "rev-list", "--count", "HEAD..@{u}")
	if err != nil {
		return 0, errors.New("the checkout does not track a remote branch")
	}
	_, err = fmt.Sscanf(strings.TrimSpace(out), "%d", &behind)
	return behind, err
}

func (u Updater) goInstall() error {
	fmt.Fprintf(u.Out, "installing %s@latest\n", u.Module)
	if _, err := u.Run("", "go", "install", u.Module+"/cmd/tmuxizer@latest"); err != nil {
		return fmt.Errorf("go install: %w", err)
	}
	fmt.Fprintln(u.Out, "✓ installed to $(go env GOBIN) or $(go env GOPATH)/bin; make sure that folder is on your PATH")
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
