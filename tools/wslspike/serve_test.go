package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bin is wslspike built once, as pure Go, for every test here.
var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wslspike")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "wslspike")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// TestADR_3_LocalDialDrivesPingEventAndRows is cases 2 and 3's CI-provable
// half: a local child process stands in for the wsl.exe hop (Design).
func TestADR_3_LocalDialDrivesPingEventAndRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var out strings.Builder
	report, err := runDial(ctx, dialConfig{local: bin}, &out)
	if err != nil {
		t.Fatalf("runDial: %v\n%s", err, out.String())
	}

	if string(report.eventPayload) != `{"seq":1}` {
		t.Errorf("event payload = %s, want {\"seq\":1}", report.eventPayload)
	}
	if report.eventAt > 5*time.Second {
		t.Errorf("event arrived after %s, want within 5s of connect (case 2)", report.eventAt)
	}
	if report.rowsDur > 2*time.Second {
		t.Errorf("rows(5000) took %s, want <= 2s sanity bound (case 3)", report.rowsDur)
	}
	if report.exitCode != 0 {
		t.Errorf("exit code = %d, want 0", report.exitCode)
	}

	for _, want := range []string{
		"connect: ", "warm ping (avg of 5): ", "event: received after",
		"rows(5000): ", "wsl.exe exit: 0 within",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report missing %q:\n%s", want, out.String())
		}
	}
}
