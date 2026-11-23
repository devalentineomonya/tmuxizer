package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devalentineomonya/tmuxizer/internal/update"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("usage: tmuxizer update [--check]")
	}

	u := update.Updater{
		Exe:     executable(),
		Source:  source,
		Module:  module,
		Version: version,
		Out:     os.Stdout,
		Run:     execIn,
	}
	if !*check {
		return u.Update()
	}
	behind, err := u.Check()
	switch {
	case err != nil:
		return err
	case behind == 0:
		fmt.Println("up to date")
	default:
		fmt.Printf("%d new commit(s) available; run `tmuxizer update`\n", behind)
	}
	return nil
}
