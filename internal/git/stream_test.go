// Requirements: QA-PERF-02

package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
)

func TestStreamCallsOnRecordAsRecordsArrive(t *testing.T) {
	streamSlowly(t, "A\x00\nB\x00\nC\x00\n")
	done := os.Getenv("FAKEGIT_STREAM_DONE_FILE")

	var records []string
	var finishedAtFirstRecord bool
	_, err := (Runner{Git: fakegit}).Stream(t.Context(), Call{Args: []string{"log"}}, func(record []byte) error {
		if len(records) == 0 {
			_, err := os.Stat(done)
			finishedAtFirstRecord = err == nil
		}
		records = append(records, string(record))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if finishedAtFirstRecord {
		t.Error("git had written its last record when the first arrived, want records delivered while git still runs")
	}
	if want := []string{"A", "B", "C"}; !slices.Equal(records, want) {
		t.Errorf("records = %q, want %q", records, want)
	}
}

func TestStreamOnRecordErrorKillsProcessAndReturnsIt(t *testing.T) {
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

func TestStreamCapturesStderrAndExitCode(t *testing.T) {
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
// 150ms apart and creates FAKEGIT_STREAM_DONE_FILE after the last.
func streamSlowly(t *testing.T, records string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(file, []byte(records), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "150")
	t.Setenv("FAKEGIT_STREAM_DONE_FILE", filepath.Join(t.TempDir(), "done"))
}
