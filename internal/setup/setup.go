package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

const iwashereRepo = "https://github.com/Murchoid/iwashere"

// Options drive Run. Everything with side effects is injectable for tests.
type Options struct {
	In  io.Reader
	Out io.Writer

	Yes    bool // answer yes to every question
	Remove bool // undo the shell hook instead of installing

	Shell   string // base name of $SHELL
	Home    string
	ZDotDir string

	Look        tools.LookPath
	Run         func(dir, name string, args ...string) error
	Roots       []string // folders for the starter config
	WriteConfig func(content string) (bool, error)
	Template    func(roots []string) string
	InstallDir  string // where iwashere is installed
}

// Run sets tmuxizer up: starter config, shell hook, and iwashere. Each step
// that changes something outside tmuxizer's own folders asks first.
func Run(o Options) error {
	out := func(format string, a ...any) { fmt.Fprintf(o.Out, format+"\n", a...) }
	in := bufio.NewReader(o.In)

	if o.Remove {
		return removeHook(o, out)
	}

	out("tmuxizer setup\n")
	if _, err := o.Look("tmux"); err != nil {
		out("✗ tmux is not installed. Install it with your package manager, then run setup again.")
		return errors.New("tmux is required")
	}
	out("✓ tmux found")

	if err := configStep(o, out); err != nil {
		return err
	}
	if err := hookStep(o, in, out); err != nil {
		return err
	}
	iwashereStep(o, in, out)

	out("\nDone. Run `tmuxizer doctor` to check the result.")
	return nil
}

func configStep(o Options, out func(string, ...any)) error {
	created, err := o.WriteConfig(o.Template(o.Roots))
	switch {
	case err != nil:
		return err
	case created:
		out("✓ wrote a starter config (edit it to change the folders that are scanned)")
	default:
		out("• config already exists, left as is")
	}
	return nil
}

func hookStep(o Options, in *bufio.Reader, out func(string, ...any)) error {
	snippet, ok := Snippet(o.Shell)
	rc, _ := RCPath(o.Shell, o.Home, o.ZDotDir)
	if !ok {
		out("• %q is not supported for the shell hook; run `tmuxizer init <bash|zsh|fish>` and add it yourself", o.Shell)
		return nil
	}
	if HasHook(rc) {
		out("• shell hook already in %s", tilde(rc, o.Home))
		return nil
	}
	if !ask(in, o, fmt.Sprintf("Open the picker in new terminals by adding a block to %s?", tilde(rc, o.Home))) {
		out("• skipped the shell hook (add it later with `tmuxizer setup`)")
		return nil
	}
	if _, err := InstallHook(rc, snippet); err != nil {
		return err
	}
	out("✓ added the hook to %s (open a new terminal, or run `exec %s`)", tilde(rc, o.Home), o.Shell)
	return nil
}

func removeHook(o Options, out func(string, ...any)) error {
	rc, ok := RCPath(o.Shell, o.Home, o.ZDotDir)
	if !ok {
		out("• %q is not supported; remove the tmuxizer block from your shell file by hand", o.Shell)
		return nil
	}
	changed, err := RemoveHook(rc)
	if err != nil {
		return err
	}
	if changed {
		out("✓ removed the hook from %s", tilde(rc, o.Home))
	} else {
		out("• no tmuxizer block in %s", tilde(rc, o.Home))
	}
	return nil
}

func iwashereStep(o Options, in *bufio.Reader, out func(string, ...any)) {
	if _, err := o.Look("iwashere"); err == nil {
		out("✓ iwashere found (session tracking is on)")
		return
	}
	out("• iwashere is not installed; it records the time you spend per project")
	if !ask(in, o, fmt.Sprintf("Build it from %s into %s?", iwashereRepo, tilde(o.InstallDir, o.Home))) {
		out("  skipped; install it later with `tmuxizer setup`")
		return
	}
	if err := installIwashere(o); err != nil {
		out("✗ could not install iwashere: %v", err)
		out("  manual steps: git clone %s && cd iwashere && go build -o iwashere ./cmd", iwashereRepo)
		return
	}
	out("✓ installed iwashere to %s", tilde(filepath.Join(o.InstallDir, "iwashere"), o.Home))
	if !onPath(o.InstallDir) {
		out("  %s is not on your PATH; add it so tmuxizer can find iwashere", tilde(o.InstallDir, o.Home))
	}
}

// installIwashere clones the repo and builds its main package.
func installIwashere(o Options) error {
	for _, bin := range []string{"git", "go"} {
		if _, err := o.Look(bin); err != nil {
			return fmt.Errorf("%s is required to build it", bin)
		}
	}
	src, err := os.MkdirTemp("", "iwashere-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(src) }()

	if err := o.Run("", "git", "clone", "--depth", "1", iwashereRepo, src); err != nil {
		return err
	}
	pkg, err := mainPackage(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.InstallDir, 0o755); err != nil {
		return err
	}
	return o.Run(src, "go", "build", "-o", filepath.Join(o.InstallDir, "iwashere"), pkg)
}

// mainPackage finds the main package; iwashere's layout changed over time.
func mainPackage(dir string) (string, error) {
	for _, pkg := range []string{"./cmd/iwashere", "./cmd", "."} {
		if _, err := os.Stat(filepath.Join(dir, pkg, "main.go")); err == nil {
			return pkg, nil
		}
	}
	return "", errors.New("no main package found in the repository")
}

func onPath(dir string) bool {
	return slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir)
}

// ask prints a question and reads y/n. With Yes it answers for the user; at
// end of input (no terminal) it answers no.
func ask(in *bufio.Reader, o Options, question string) bool {
	if o.Yes {
		fmt.Fprintf(o.Out, "%s [y/N] y\n", question)
		return true
	}
	fmt.Fprintf(o.Out, "%s [y/N] ", question)
	line, err := in.ReadString('\n')
	if err != nil {
		fmt.Fprintln(o.Out) // no terminal: finish the prompt line
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func tilde(p, home string) string {
	if home != "" {
		if rest, ok := strings.CutPrefix(p, home+"/"); ok {
			return "~/" + rest
		}
	}
	return p
}
