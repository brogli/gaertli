package main_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
)

// bin is nebegit-core built once, as pure Go, for every test here.
var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nebegit-core")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "nebegit-core")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "CGO_ENABLED=0 go build (TR-07): %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
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

func TestADR_3_UnknownArgumentExits2(t *testing.T) {
	err := exec.Command(bin, "bogus").Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Errorf("err = %v, want exit 2", err)
	}
}
