// Command tmuxizer is a terminal workflow orchestrator: pick a project and
// get a tmux session with your app, editor and agents already running.
package main

import (
	"fmt"
	"os"
	"syscall"
)

// module is the import path `tmuxizer update` installs from.
const module = "github.com/devalentineomonya/tmuxizer"

// Set at build time with -ldflags "-X main.version=... -X main.source=...".
var (
	version = "dev"
	source  = "" // checkout the binary was built from, used by `update`
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tmuxizer:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return pick()
	}
	switch args[0] {
	case "init":
		return printInit(args[1:])
	case "setup":
		return runSetup(args[1:])
	case "doctor":
		return runDoctor()
	case "update":
		return runUpdate(args[1:])
	case "time":
		return runTime(args[1:])
	case "version", "--version", "-v":
		fmt.Println("tmuxizer", version)
		return nil
	case "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	case closedCommand:
		return sessionClosed(args[1:])
	}
	return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
}

const usage = `usage: tmuxizer [command]

  (none)             open the project picker
  setup [-y]         create a starter config, add the shell hook, install iwashere
  setup --remove     take the shell hook out again
  doctor             check tmux, iwashere, editors, config and the shell hook
  update [--check]   rebuild from your checkout (or go install) and replace this binary
  time [period]      time spent per project: today, week (default) or all
  init <shell>       print the shell hook for bash, zsh or fish
  version            print the version

keys in the picker:
  j/k move   enter open   / filter   s time   x close session   t shell here   q quit
`

// execShell replaces the process with the user's shell inside dir.
func execShell(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return syscall.Exec(shell, []string{shell}, os.Environ())
}
