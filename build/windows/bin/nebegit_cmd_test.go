//go:build windows

// Requirements: FR-REPO-06

// This file builds only on Windows: it runs nebegit.cmd through cmd.exe,
// which the other CI jobs do not have.
package bin_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCmdScriptStartsTheAppDetached(t *testing.T) {
	t.Parallel()
	dir, scriptPath := newApp(t, "nebegit.cmd", 0o644)

	cmd := exec.Command("cmd", "/c", scriptPath, `C:\x`)
	cmd.Dir = dir

	// No output pipes: the started app inherits them, and reading them would
	// wait until the app exits.
	if err := cmd.Run(); err != nil {
		t.Fatalf("nebegit.cmd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); err == nil {
		t.Error("nebegit.cmd waited for the app")
	}

	if got, want := readArgvEventually(t, dir), `C:\x`; got != want {
		t.Errorf("app argv = %q, want %q", got, want)
	}
}
