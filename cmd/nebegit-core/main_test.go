package main_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// bin is nebegit-core; guiBin is a fake GUI standing in for the Windows app
// the launch hands off to.
var bin, guiBin string

func TestMain(m *testing.M) {
	testbin.Main(m,
		testbin.Bin{Path: &bin, Pkg: "."},
		testbin.Bin{Path: &guiBin, Pkg: "./testdata/fakegui"},
	)
}

// envWithout drops every existing entry for key from os.Environ(), so a
// caller can append its own override without relying on which duplicate a
// child process's libc resolves first (under WSL, WSL_DISTRO_NAME is
// already set).
func envWithout(key string) []string {
	prefix := key + "="
	var out []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

func TestTR_07_CoreBuildsWithoutCgo(t *testing.T) {
	if _, err := os.Stat(bin); err != nil {
		t.Fatal(err)
	}
}

func TestADR_3_StdioCoreAnswersVersionAndExitsWhenStdinCloses(t *testing.T) {
	cmd := exec.Command(bin, "--stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := remote.NewClient(stdout, stdin, nil).Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != (core.Version{App: "dev", Protocol: core.Protocol}) {
		t.Errorf("Version = %+v", v)
	}

	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("exit: %v, want 0", err)
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("core did not exit within 2s of stdin closing")
	}
}

func TestADR_3_VersionFlagPrintsAppVersion(t *testing.T) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(out), []byte("dev")) {
		t.Errorf("--version printed %q, want dev", out)
	}
}

// TestQA_INST_03_LdflagsStampAppVersion proves the release build's version
// stamping mechanism: -ldflags -X into buildinfo.App, surfaced by --version.
func TestQA_INST_03_LdflagsStampAppVersion(t *testing.T) {
	stamped := filepath.Join(t.TempDir(), "nebegit-core-stamped")
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/brogli/nebegit/internal/buildinfo.App=v9.9.9",
		"-o", stamped, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	out, err := exec.Command(stamped, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte("v9.9.9\n")) {
		t.Errorf("--version printed %q, want %q", out, "v9.9.9\n")
	}
}

// TestADR_3_UnknownArgumentExits2 uses a flag-shaped bad argument: a bare
// word is a legitimate path (launch mode), so "bogus" alone would not reach
// usage().
func TestADR_3_UnknownArgumentExits2(t *testing.T) {
	err := exec.Command(bin, "--bogus").Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Errorf("err = %v, want exit 2", err)
	}
}

func TestFR_REPO_06_LaunchHandsOffToTheGUIWithoutWaiting(t *testing.T) {
	guiPath := filepath.Join(filepath.Dir(bin), "gui-path")
	if err := os.WriteFile(guiPath, []byte(guiBin), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(guiPath) })

	cwd := t.TempDir()
	argvFile := filepath.Join(cwd, "argv.txt")

	cmd := exec.Command(bin)
	cmd.Env = append(envWithout("WSL_DISTRO_NAME"), "WSL_DISTRO_NAME=Ubuntu-22.04")
	cmd.Dir = cwd
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nebegit-core: %v\n%s", err, out)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("nebegit-core took %v to exit, want promptly (not waiting on the helper)", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	var content []byte
	for time.Now().Before(deadline) {
		content, err = os.ReadFile(argvFile)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("fake GUI helper never wrote its argv file")
	}

	want := remote.BuildWSLPath("Ubuntu-22.04", cwd)
	if got := strings.TrimSpace(string(content)); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
}
