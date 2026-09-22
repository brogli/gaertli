//go:build bench

package bench

import (
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// TestQA_PERF_04_FirstFilteredLogRowUnder2s proves the literal QA-PERF-04
// number against the real 100k-commit reference repo (spec 13's
// tools/mkrepo): a query narrowing to a small, known subset of commits
// ("merge feature-1", and substring-adjacent branch names like
// "feature-10".."feature-19" — still a small subset of 100k) still
// surfaces its first matching row within budget. The mechanism this
// depends on (a filtered row observable long before its stream finishes)
// is already proven at synthetic scale by internal/services's own
// QA-PERF-04 case; this is the literal number.
func TestQA_PERF_04_FirstFilteredLogRowUnder2s(t *testing.T) {
	c, id := openReference(t)
	start := time.Now()
	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Query: "merge feature-1"}); err != nil {
		t.Fatal(err)
	}
	for {
		rows, err := c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Rows) > 0 {
			elapsed := time.Since(start)
			t.Logf("first filtered row after %s", elapsed)
			if elapsed > 2*time.Second {
				t.Fatalf("QA-PERF-04: first filtered row after %s, want <= 2s", elapsed)
			}
			return
		}
		if rows.Done {
			t.Fatal(`filter finished with zero rows; query "merge feature-1" matched nothing in the reference repo`)
		}
	}
}
