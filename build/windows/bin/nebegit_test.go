// Package bin_test runs the nebegit and nebegit.cmd wrapper scripts under a
// fake nebegit.exe and a fake wslpath, without a real Windows or WSL setup.
package bin_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/testbin"
)

var fakeExePath string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &fakeExePath, Pkg: "./testdata/fakeexe"})
}

// wslpathSuccessScript stands in for wslpath -w: it echoes the argument back
// prefixed, so a test can tell a converted argument from an unconverted one.
const wslpathSuccessScript = "#!/bin/sh\nprintf 'WIN:%s\\n' \"$2\"\n"

const wslpathFailScript = "#!/bin/sh\necho 'boom: fake wslpath failure' >&2\nexit 3\n"

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the sh script needs a POSIX shell, which the Windows job does not provide")
	}
}

// newApp lays out a temp copy of the install's bin/nebegit.exe structure and
// returns the app dir and the script's path inside it.
func newApp(t *testing.T) (dir, scriptPath string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath = filepath.Join(dir, "bin", "nebegit")
	copyFile(t, "nebegit", scriptPath, 0o755)
	copyFile(t, fakeExePath, filepath.Join(dir, "nebegit.exe"), 0o755)
	return dir, scriptPath
}

func copyFile(t *testing.T, src, dst string, perm os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, perm); err != nil {
		t.Fatal(err)
	}
}

// fakeWslpath writes script as an executable "wslpath" in a fresh dir,
// returned for prepending to PATH.
func fakeWslpath(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wslpath"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// scriptEnv is a clean environment for running the script: wslDistro sets
// WSL_DISTRO_NAME ("" leaves it unset), wslpathDir is prepended to PATH so a
// fake wslpath shadows any real one ("" adds none).
func scriptEnv(wslDistro, wslpathDir string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "WSL_DISTRO_NAME=") || strings.HasPrefix(e, "PATH=") {
			continue
		}
		env = append(env, e)
	}
	path := os.Getenv("PATH")
	if wslpathDir != "" {
		path = wslpathDir + string(os.PathListSeparator) + path
	}
	env = append(env, "PATH="+path)
	if wslDistro != "" {
		env = append(env, "WSL_DISTRO_NAME="+wslDistro)
	}
	return env
}

// readArgvEventually polls for the fake app's argv file, which it writes
// only after a short sleep.
func readArgvEventually(t *testing.T, path string) (string, bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(content)), true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", false
}

func TestFR_REPO_06_ScriptConvertsArgumentsWithWslpathInWSL(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)
	wslpathDir := fakeWslpath(t, wslpathSuccessScript)

	cmd := exec.Command("sh", scriptPath, "repo")
	cmd.Dir = dir
	cmd.Env = scriptEnv("Ubuntu-22.04", wslpathDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}

	got, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt"))
	if !ok {
		t.Fatal("the app never recorded its arguments")
	}
	if want := "WIN:repo"; got != want {
		t.Errorf("app argv = %q, want %q", got, want)
	}
}

func TestFR_REPO_06_ScriptPassesNoArgumentsWhenNoneGiven(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)

	cmd := exec.Command("sh", scriptPath)
	cmd.Dir = dir
	cmd.Env = scriptEnv("Ubuntu-22.04", "")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}

	got, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt"))
	if !ok {
		t.Fatal("the app never recorded its arguments")
	}
	if got != "" {
		t.Errorf("app argv = %q, want none", got)
	}
}

func TestFR_REPO_06_ScriptFindsTheAppThroughASymlink(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)
	wslpathDir := fakeWslpath(t, wslpathSuccessScript)

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(scriptPath, link); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sh", link, "repo")
	cmd.Dir = dir
	cmd.Env = scriptEnv("Ubuntu-22.04", wslpathDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}

	got, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt"))
	if !ok {
		t.Fatal("the app beside the real script never started")
	}
	if want := "WIN:repo"; got != want {
		t.Errorf("app argv = %q, want %q", got, want)
	}
}

func TestFR_REPO_06_ScriptLeavesArgumentsUnchangedOutsideWSL(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)
	// present but must not be called: WSL_DISTRO_NAME is unset below
	wslpathDir := fakeWslpath(t, wslpathSuccessScript)

	cmd := exec.Command("sh", scriptPath, "some/path")
	cmd.Dir = dir
	cmd.Env = scriptEnv("", wslpathDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}

	got, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt"))
	if !ok {
		t.Fatal("the app never recorded its arguments")
	}
	if want := "some/path"; got != want {
		t.Errorf("app argv = %q, want %q unconverted (wslpath must not run)", got, want)
	}
}

func TestFR_REPO_06_ScriptExitsWithoutWaitingForTheApp(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)
	wslpathDir := fakeWslpath(t, wslpathSuccessScript)

	cmd := exec.Command("sh", scriptPath, "repo")
	cmd.Dir = dir
	cmd.Env = scriptEnv("Ubuntu-22.04", wslpathDir)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); err == nil {
		t.Error("the app had already recorded its arguments before the script exited")
	}

	if _, ok := readArgvEventually(t, filepath.Join(dir, "argv.txt")); !ok {
		t.Fatal("the app never recorded its arguments")
	}
}

func TestFR_REPO_06_ScriptFailsWhenWslpathFails(t *testing.T) {
	skipOnWindows(t)
	dir, scriptPath := newApp(t)
	wslpathDir := fakeWslpath(t, wslpathFailScript)

	cmd := exec.Command("sh", scriptPath, "repo")
	cmd.Dir = dir
	cmd.Env = scriptEnv("Ubuntu-22.04", wslpathDir)

	out, err := cmd.CombinedOutput()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("script exit = %v, want status 3", err)
	}
	if !strings.Contains(string(out), "boom: fake wslpath failure") {
		t.Errorf("script output = %q, want it to contain the wslpath message", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); err == nil {
		t.Error("the app started despite the wslpath failure")
	}
}
