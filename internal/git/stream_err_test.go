package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
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
	result, err := (Runner{Git: fakegit}).StreamErr(t.Context(), Call{Args: []string{"fetch"}}, func(line []byte) error {
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
	if !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
	if string(result.Stdout) != "unrelated stdout" {
		t.Errorf("Stdout = %q, want %q (untouched by stderr streaming)", result.Stdout, "unrelated stdout")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestFR_SYNC_07_StreamErrCancelKillsProcess(t *testing.T) {
	heartbeat := sleepWithHeartbeat(t)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := (Runner{Git: fakegit, StopGrace: testGrace}).StreamErr(ctx, Call{Args: []string{"fetch"}}, func([]byte) error { return nil })
	elapsed := time.Since(start)

	coretest.AssertCode(t, err, core.Cancelled)
	if elapsed > time.Second {
		t.Errorf("StreamErr() took %s, want well under 5s", elapsed)
	}
	assertStopped(t, heartbeat)
}

// A grandchild that inherited git's stderr and outlives the interrupt,
// e.g. a stalled ssh transport, keeps the pipe open after git itself is gone.
func TestFR_SYNC_07_StreamErrCancelReturnsWhileAGrandchildHoldsStderr(t *testing.T) {
	assertCancelReturnsDespiteGrandchild(t, "(trap '' TERM; exec sleep 6) & wait")
}

// The shape of a delayed transport: the grandchild holds git's stderr but
// not its stdout, so stdout ends as soon as git is gone.
func TestFR_SYNC_07_StreamErrCancelReturnsWhileAGrandchildHoldsOnlyStderr(t *testing.T) {
	assertCancelReturnsDespiteGrandchild(t, "(trap '' TERM; exec sleep 6 >/dev/null) & wait")
}

func assertCancelReturnsDespiteGrandchild(t *testing.T, script string) {
	t.Helper()
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := (Runner{Git: "sh", StopGrace: testGrace}).StreamErr(ctx, Call{Args: []string{"-c", script}}, func([]byte) error { return nil })
	elapsed := time.Since(start)

	coretest.AssertCode(t, err, core.Cancelled)
	if elapsed > time.Second {
		t.Errorf("StreamErr() took %s after cancel, want it back within the grace period", elapsed)
	}
}

func TestFR_SYNC_07_StreamErrKeepsStderrForAFailedCall(t *testing.T) {
	t.Setenv("FAKEGIT_STDERR", "fatal: could not read from remote\n")
	t.Setenv("FAKEGIT_EXIT", "128")

	result, err := (Runner{Git: fakegit}).StreamErr(t.Context(), Call{Args: []string{"fetch"}}, func([]byte) error { return nil })

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "could not read from remote") {
		t.Errorf("GitStderr = %q, want git's own message", ce.GitStderr)
	}
	if !strings.Contains(string(result.Stderr), "could not read from remote") {
		t.Errorf("Result.Stderr = %q, want the streamed text", result.Stderr)
	}
}
