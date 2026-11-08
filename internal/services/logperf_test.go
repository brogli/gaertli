// Requirements: FR-LOG-09, QA-PERF-02

package services

import (
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestFirstRowObservableWhileLoadingWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 200) // 5 records * 200ms = 1s unstreamed

	start := time.Now()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	observed := waitForFirstRow(t, c, id)

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("first row observable after %s, want well under 500ms (the full 1s unstreamed load)", elapsed)
	}

	if observed.Done {
		t.Error("Done = true with the first row, want a partial load")
	}
	if observed.Error != nil {
		t.Errorf("Error = %v, want nil", observed.Error)
	}
	if len(observed.Rows) != 1 {
		t.Errorf("Rows = %d, want exactly the 1 asked for", len(observed.Rows))
	}

	waitForLog(t, c, id) // drain the background stream so it doesn't leak noise into later tests
}
