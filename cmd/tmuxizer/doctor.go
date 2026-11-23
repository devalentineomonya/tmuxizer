package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalentineomonya/tmuxizer/internal/config"
	"github.com/devalentineomonya/tmuxizer/internal/doctor"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

func runDoctor() error {
	home, _ := os.UserHomeDir()
	checks := doctor.Run(doctor.Env{
		Look:        tools.Default(),
		TmuxVersion: func() (string, error) { return execIn("", "tmux", "-V") },
		Home:        home,
		Shell:       filepath.Base(os.Getenv("SHELL")),
		ZDotDir:     os.Getenv("ZDOTDIR"),
		Path:        filepath.SplitList(os.Getenv("PATH")),
		Exe:         executable(),
		Source:      source,
		Config:      config.Load,
		ConfigPath:  config.Path(),
	})

	th := theme.New("")
	fmt.Println(th.Accent.Render("▌tmuxizer") + th.Dim.Render("  "+version))
	fmt.Println()
	for _, c := range checks {
		mark, style := "✓", th.Good
		switch c.Status {
		case doctor.Warn:
			mark, style = "!", th.Warn
		case doctor.Fail:
			mark, style = "✗", th.Bad
		}
		fmt.Printf("  %s %s %s\n", style.Render(mark), pad(c.Name, 16), c.Detail)
		if c.Hint != "" && c.Status != doctor.OK {
			fmt.Printf("    %s\n", th.Dim.Render(c.Hint))
		}
	}
	if doctor.Failed(checks) {
		return errors.New("some checks failed")
	}
	return nil
}

// executable resolves the running binary, following symlinks.
func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return "tmuxizer"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
