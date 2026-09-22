package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestFR_REPO_06_RunDispatchesBareArgsToLaunchAndKeepsFlagsWorking is a
// white-box, in-process test of run's dispatch: WSL_DISTRO_NAME is cleared
// so a routed-to-launch call fails deterministically (its own case, below)
// rather than depending on a real WSL/editor — proving routing without
// needing either. --stdio is not dispatched here: it blocks on stdin,
// already covered by the subprocess tests in main_test.go.
func TestFR_REPO_06_RunDispatchesBareArgsToLaunchAndKeepsFlagsWorking(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args launches the cwd", nil, 1},
		{"a bare path launches it", []string{"/some/path"}, 1},
		{"--version is unchanged", []string{"--version"}, 0},
		{"an unknown flag is a usage error", []string{"--bogus"}, 2},
		{"two arguments is a usage error", []string{"a", "b"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestFR_REPO_06_LaunchFailsClearlyWithoutWSLOrGuiPath(t *testing.T) {
	tests := []struct {
		name       string
		distro     string
		wantSubstr string
	}{
		{"WSL_DISTRO_NAME unset", "", "not running inside a WSL distro shell"},
		{"no gui-path file next to the binary", "Ubuntu-22.04", `Install nebegit command in this distro`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WSL_DISTRO_NAME", tt.distro)

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			orig := os.Stderr
			os.Stderr = w
			got := launch(t.TempDir())
			os.Stderr = orig
			_ = w.Close()
			stderr, _ := io.ReadAll(r)

			if got != 1 {
				t.Errorf("launch() = %d, want 1", got)
			}
			if !strings.Contains(string(stderr), tt.wantSubstr) {
				t.Errorf("stderr = %q, want containing %q", stderr, tt.wantSubstr)
			}
		})
	}
}
