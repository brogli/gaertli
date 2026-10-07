package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
)

func TestADR_5_StreamCallsOnRecordAsRecordsArrive(t *testing.T) {
	streamSlowly(t, "A\x00\nB\x00\nC\x00\n")

	start := time.Now()
	var records []string
	var firstAt time.Duration
	_, err := (Runner{Git: fakegit}).Stream(t.Context(), Call{Args: []string{"log"}}, func(record []byte) error {
		if len(records) == 0 {
			firstAt = time.Since(start)
		}
		records = append(records, string(record))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if firstAt > 75*time.Millisecond {
		t.Errorf("first record arrived after %s, want well under the 150ms record delay (proves streaming, not buffer-then-split)", firstAt)
	}
	if want := []string{"A", "B", "C"}; !slices.Equal(records, want) {
		t.Errorf("records = %q, want %q", records, want)
	}
}

func TestADR_5_StreamOnRecordErrorKillsProcessAndReturnsIt(t *testing.T) {
	streamSlowly(t, "A\x00\nB\x00\nC\x00\n")

	calls := 0
	_, err := (Runner{Git: fakegit}).Stream(t.Context(), Call{Args: []string{"log"}}, func([]byte) error {
		calls++
		return errors.New("stop here")
	})

	coretest.AssertCode(t, err, core.Internal)
	if !strings.Contains(err.Error(), "stop here") {
		t.Errorf("err = %v, want it to mention %q", err, "stop here")
	}
	if calls != 1 {
		t.Errorf("onRecord called %d times, want 1 (stopped after the first error)", calls)
	}
}

func TestADR_5_StreamCapturesStderrAndExitCode(t *testing.T) {
	t.Setenv("FAKEGIT_STDERR", "warning: x")
	t.Setenv("FAKEGIT_EXIT", "1")

	result, err := (Runner{Git: fakegit}).Stream(t.Context(), Call{Args: []string{"log"}}, func([]byte) error { return nil })

	coretest.AssertCode(t, err, core.GitFailed)
	if string(result.Stderr) != "warning: x" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "warning: x")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

// streamSlowly makes fakegit's log call print the NUL-terminated records
// 150ms apart.
func streamSlowly(t *testing.T, records string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(file, []byte(records), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "150")
}
