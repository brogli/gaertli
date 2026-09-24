//go:build bench

package bench

import (
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// TestQA_PERF_04_FirstFilteredLogRowUnder2s checks against the 100k-commit
// reference repo that a query narrowing to a small subset of commits
// ("merge feature-1", plus the substring-adjacent "feature-10".."feature-19")
// surfaces its first matching row within budget.
func TestQA_PERF_04_FirstFilteredLogRowUnder2s(t *testing.T) {
	c, id := openReference(t)
	start := time.Now()
	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Query: "merge feature-1"}); err != nil {
		t.Fatal(err)
	}
	elapsed := firstRowAfter(t, c, id, start)
	t.Logf("first filtered row after %s", elapsed)
	if elapsed > 2*time.Second {
		t.Fatalf("QA-PERF-04: first filtered row after %s, want <= 2s", elapsed)
	}
}
