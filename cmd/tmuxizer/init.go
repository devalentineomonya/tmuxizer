package main

import (
	"errors"
	"fmt"

	"github.com/devalentineomonya/tmuxizer/internal/setup"
)

// printInit prints the shell snippet for manual installation.
func printInit(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: tmuxizer init <bash|zsh|fish>")
	}
	snippet, ok := setup.Snippet(args[0])
	if !ok {
		return fmt.Errorf("unsupported shell %q (want bash, zsh or fish)", args[0])
	}
	fmt.Println(snippet)
	return nil
}
