package main_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// bin is nebegit-core.
var bin string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &bin, Pkg: "."})
}

func TestADR_3_StdioCoreAnswersVersionAndExitsWhenStdinCloses(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), bin, "--stdio")
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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
	out, err := exec.CommandContext(t.Context(), bin, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(out), []byte("dev")) {
		t.Errorf("--version printed %q, want dev", out)
	}
}

func TestDIST_03_LdflagsStampAppVersion(t *testing.T) {
	stamped := filepath.Join(t.TempDir(), testbin.Exe("nebegit-core-stamped"))
	build := exec.CommandContext(t.Context(), "go", "build",
		"-ldflags", "-X github.com/brogli/nebegit/internal/buildinfo.App=v9.9.9",
		"-o", stamped, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	out, err := exec.CommandContext(t.Context(), stamped, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte("v9.9.9\n")) {
		t.Errorf("--version printed %q, want %q", out, "v9.9.9\n")
	}
}

func TestADR_3_UnknownArgumentExits2(t *testing.T) {
	err := exec.CommandContext(t.Context(), bin, "--bogus").Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != 2 {
		t.Errorf("err = %v, want exit 2", err)
	}
}

func TestADR_3_ABarePathExits2WithTheUsageLine(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), bin, "/some/path").CombinedOutput()
	if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != 2 {
		t.Errorf("err = %v, want exit 2", err)
	}
	if !bytes.Contains(out, []byte("usage: nebegit-core")) {
		t.Errorf("output = %q, want the usage line", out)
	}
}
