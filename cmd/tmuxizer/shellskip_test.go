package main

import (
	"os"
	"testing"
	"time"
)

func TestShellSkipConsumedOnce(t *testing.T) {
	dir := t.TempDir()
	markShellSkip(dir, 42)
	now := time.Now()
	if !consumeShellSkip(dir, 42, now) {
		t.Fatal("first run in the new shell should be skipped")
	}
	if consumeShellSkip(dir, 42, now) {
		t.Error("a manual run afterwards must open the picker")
	}
}

func TestShellSkipOtherParentAndStale(t *testing.T) {
	dir := t.TempDir()
	markShellSkip(dir, 42)
	if consumeShellSkip(dir, 43, time.Now()) {
		t.Error("marker for another shell must not match")
	}
	if consumeShellSkip(dir, 42, time.Now().Add(2*skipTTL)) {
		t.Error("stale marker must not skip")
	}
	if _, err := os.Stat(skipMarker(dir, 42)); err == nil {
		t.Error("stale marker should still be cleaned up")
	}
}
