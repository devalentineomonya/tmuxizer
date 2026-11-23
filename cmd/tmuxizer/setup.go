package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/devalentineomonya/tmuxizer/internal/config"
	"github.com/devalentineomonya/tmuxizer/internal/setup"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	yes := fs.Bool("yes", false, "")
	fs.BoolVar(yes, "y", false, "")
	remove := fs.Bool("remove", false, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("usage: tmuxizer setup [--yes] [--remove]")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return setup.Run(setup.Options{
		In:          os.Stdin,
		Out:         os.Stdout,
		Yes:         *yes,
		Remove:      *remove,
		Shell:       filepath.Base(os.Getenv("SHELL")),
		Home:        home,
		ZDotDir:     os.Getenv("ZDOTDIR"),
		Look:        tools.Default(),
		Run:         runIn,
		Roots:       config.DefaultRoots(),
		WriteConfig: config.Write,
		Template:    config.Template,
		InstallDir:  filepath.Join(home, ".local", "bin"),
	})
}
