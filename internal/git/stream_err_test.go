package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

func TestFR_SYNC_07_StreamErrCallsOnLinePerRecordStdoutUntouched(t *testing.T) {
	file := filepath.Join(t.TempDir(), "stderr-records")
	data := "remote: Enumerating objects: 90, done.\n" +
		"remote: Counting objects:   1% (1/90)   \rremote: Counting objects:  50% (45/90)   \rremote: Counting objects: 100% (90/90), done.\n"
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STDERR_FILE", file)
	t.Setenv("FAKEGIT_STDERR_DELAY_MS", "50")
	t.Setenv("FAKEGIT_STDOUT", "unrelated stdout")

	var lines []string
	result, err := (Runner{Git: fakegit}).StreamErr(context.Background(), Call{Args: []string{"fetch"}}, func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"remote: Enumerating objects: 90, done.",
		"remote: Counting objects:   1% (1/90)   ",
		"remote: Counting objects:  50% (45/90)   ",
		"remote: Counting objects: 100% (90/90), done.",
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
	for i, l := range lines {
		if l != want[i] {
			t.Errorf("line %d = %q, want %q", i, l, want[i])
		}
	}
	if string(result.Stdout) != "unrelated stdout" {
		t.Errorf("Stdout = %q, want %q (untouched by stderr streaming)", result.Stdout, "unrelated stdout")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestFR_SYNC_07_StreamErrCancelKillsProcess(t *testing.T) {
	t.Setenv("FAKEGIT_SLEEP_MS", "5000")
	heartbeat := filepath.Join(t.TempDir(), "heartbeat")
	t.Setenv("FAKEGIT_HEARTBEAT", heartbeat)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := (Runner{Git: fakegit}).StreamErr(ctx, Call{Args: []string{"fetch"}}, func([]byte) error { return nil })
	elapsed := time.Since(start)

	assertCode(t, err, core.Cancelled)
	if elapsed > time.Second {
		t.Errorf("StreamErr() took %s, want well under 5s", elapsed)
	}

	stopped := mtime(t, heartbeat)
	time.Sleep(300 * time.Millisecond)
	if got := mtime(t, heartbeat); !got.Equal(stopped) {
		t.Errorf("heartbeat mtime advanced after cancel: %v -> %v (process not killed)", stopped, got)
	}
}

func TestFR_SYNC_07_StreamErrKeepsStderrForAFailedCall(t *testing.T) {
	t.Setenv("FAKEGIT_STDERR", "fatal: could not read from remote\n")
	t.Setenv("FAKEGIT_EXIT", "128")

	result, err := (Runner{Git: fakegit}).StreamErr(context.Background(), Call{Args: []string{"fetch"}}, func([]byte) error { return nil })

	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.GitFailed {
		t.Fatalf("err = %v, want GitFailed", err)
	}
	if !strings.Contains(ce.GitStderr, "could not read from remote") {
		t.Errorf("GitStderr = %q, want git's own message", ce.GitStderr)
	}
	if !strings.Contains(string(result.Stderr), "could not read from remote") {
		t.Errorf("Result.Stderr = %q, want the streamed text", result.Stderr)
	}
}
