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

// newApp lays out a temp copy of the install's bin/<script> structure and
// returns the app dir and the script's path inside it.
func newApp(t *testing.T, script string, perm os.FileMode) (dir, scriptPath string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath = filepath.Join(dir, "bin", script)
	copyFile(t, script, scriptPath, perm)
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

// scriptCmd builds the command for running dir's bin/nebegit script, with a
// clean environment: wslDistro sets WSL_DISTRO_NAME ("" leaves it unset),
// wslpathDir is prepended to PATH so a fake wslpath shadows any real one (""
// adds none).
func scriptCmd(dir, wslDistro, wslpathDir string, args ...string) *exec.Cmd {
	shArgs := append([]string{filepath.Join(dir, "bin", "nebegit")}, args...)
	cmd := exec.Command("sh", shArgs...)
	cmd.Dir = dir
	path := os.Getenv("PATH")
	if wslpathDir != "" {
		path = wslpathDir + string(os.PathListSeparator) + path
	}
	// os/exec keeps the last of duplicate keys, so appending overrides the
	// inherited PATH and WSL_DISTRO_NAME; an empty WSL_DISTRO_NAME counts as
	// unset for the script's `[ -n … ]`.
	cmd.Env = append(os.Environ(), "PATH="+path, "WSL_DISTRO_NAME="+wslDistro)
	return cmd
}

// runScript runs dir's bin/nebegit script and fails the test if it exits
// with an error.
func runScript(t *testing.T, dir, distro, wslpathDir string, args ...string) {
	t.Helper()
	if out, err := scriptCmd(dir, distro, wslpathDir, args...).CombinedOutput(); err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
}

// readArgvEventually polls for the fake app's argv file, which it writes
// only after a short sleep, failing the test if it never appears.
func readArgvEventually(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "argv.txt")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(content))
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the app never recorded its arguments")
	return ""
}

func TestFR_REPO_06_ScriptArgv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		distro  string
		args    []string
		wslpath bool
		symlink bool
		want    string
	}{
		{name: "converts in WSL", distro: "Ubuntu-22.04", args: []string{"repo"}, wslpath: true, want: "WIN:repo"},
		{name: "no args", distro: "Ubuntu-22.04", want: ""},
		{
			name: "through a symlink", distro: "Ubuntu-22.04", args: []string{"repo"}, wslpath: true,
			symlink: true, want: "WIN:repo",
		},
		{name: "unchanged outside WSL", args: []string{"some/path"}, wslpath: true, want: "some/path"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			skipOnWindows(t)

			dir, _ := newApp(t, "nebegit", 0o755)
			runDir := dir
			if tc.symlink {
				// The app is found beside the real script, not the symlink:
				// runDir holds only the symlink, so a hit proves the script
				// resolved it.
				runDir = t.TempDir()
				if err := os.Mkdir(filepath.Join(runDir, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "bin", "nebegit"), filepath.Join(runDir, "bin", "nebegit")); err != nil {
					t.Fatal(err)
				}
			}

			var wslpathDir string
			if tc.wslpath {
				wslpathDir = fakeWslpath(t, wslpathSuccessScript)
			}

			runScript(t, runDir, tc.distro, wslpathDir, tc.args...)

			if got := readArgvEventually(t, runDir); got != tc.want {
				t.Errorf("app argv = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFR_REPO_06_ScriptExitsWithoutWaitingForTheApp(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)
	dir, _ := newApp(t, "nebegit", 0o755)
	wslpathDir := fakeWslpath(t, wslpathSuccessScript)

	runScript(t, dir, "Ubuntu-22.04", wslpathDir, "repo")

	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); err == nil {
		t.Error("the app had already recorded its arguments before the script exited")
	}
	readArgvEventually(t, dir)
}

func TestFR_REPO_06_ScriptFailsWhenWslpathFails(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)
	dir, _ := newApp(t, "nebegit", 0o755)
	wslpathDir := fakeWslpath(t, wslpathFailScript)

	out, err := scriptCmd(dir, "Ubuntu-22.04", wslpathDir, "repo").CombinedOutput()
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
