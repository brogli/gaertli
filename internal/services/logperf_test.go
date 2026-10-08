package services

import (
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestQA_PERF_02_FR_LOG_09_FirstRowObservableWhileLoadingWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 200) // 5 records * 200ms = 1s unstreamed

	start := time.Now()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	var observed core.LogRowsResult
	waitFor(t, func() bool {
		var err error
		observed, err = c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		return len(observed.Rows) > 0
	})
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
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
