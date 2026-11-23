package main

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// skipTTL bounds how long a marker stays valid, so a stale one left by a
// shell without the hook cannot swallow a later launch after pid reuse.
const skipTTL = time.Minute

func skipMarker(dir string, pid int) string {
	return filepath.Join(dir, "shell-skip-"+strconv.Itoa(pid))
}

// markShellSkip tells the next tmuxizer started by this process's shell to
// stay quiet. exec keeps the pid, so the shell's startup file, which runs the
// picker hook, is a child of that pid.
func markShellSkip(dir string, pid int) {
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	_ = os.WriteFile(skipMarker(dir, pid), nil, 0o600)
}

// consumeShellSkip reports whether this run is the shell hook firing inside a
// shell that `t` just opened. It is true once; a later manual run proceeds.
func consumeShellSkip(dir string, ppid int, now time.Time) bool {
	path := skipMarker(dir, ppid)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if os.Remove(path) != nil {
		return false
	}
	return now.Sub(info.ModTime()) < skipTTL
}
