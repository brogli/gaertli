//go:build windows

// This file builds only on Windows: it runs nebegit.cmd through cmd.exe,
// which the other CI jobs do not have.
package bin_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFR_REPO_06_CmdScriptStartsTheAppDetached(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, "nebegit.cmd", filepath.Join(dir, "bin", "nebegit.cmd"), 0o644)
	copyFile(t, fakeExePath, filepath.Join(dir, "nebegit.exe"), 0o755)

	cmd := exec.Command("cmd", "/c", filepath.Join("bin", "nebegit.cmd"), `C:\x`)
	cmd.Dir = dir

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("nebegit.cmd: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); err == nil {
		t.Error("nebegit.cmd waited for the app")
	}

	got, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt"))
	if !ok {
		t.Fatal("the app never recorded its arguments")
	}
	if want := `C:\x`; got != want {
		t.Errorf("app argv = %q, want %q", got, want)
	}
}
