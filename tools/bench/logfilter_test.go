//go:build bench

// Requirements: QA-PERF-04

package bench

import (
	"testing"
	"time"
)

// TestSearchLogUnder2s checks against the 100k-commit reference
// repo that a query matching a small subset of commits ("merge feature-1",
// plus the substring-adjacent "feature-10".."feature-19") resolves within
// budget.
func TestSearchLogUnder2s(t *testing.T) {
	c, id := openReference(t)
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	waitLogDone(t, c, id)

	start := time.Now()
	if _, err := c.SearchLog(t.Context(), id, "merge feature-1"); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	t.Logf("SearchLog after %s", elapsed)
	if elapsed > 2*time.Second {
		t.Fatalf("QA-PERF-04: SearchLog after %s, want <= 2s", elapsed)
	}
}
