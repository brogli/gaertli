//go:build bench

// Requirements: QA-PERF-05

package bench

import (
	"strings"
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/services"
)

func TestDiffFileUnder1MBUnder500ms(t *testing.T) {
	repo := gittest.New(t)
	content := strings.Repeat("a line of committed text\n", 700*1024/25) // ~700 KB
	gittest.Commit(t, repo.Dir, "file.txt", content, "add a ~700KB file")
	sha := gittest.HeadSHA(t, repo.Dir)

	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseWorktree(t.Context(), wt.ID) })

	start := time.Now()
	diff, err := c.DiffFile(t.Context(), wt.ID, sha, "file.txt", "", core.DiffOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if diff.TooLarge || diff.NewContent != content {
		t.Fatal("DiffFile did not return the full content")
	}
	t.Logf("DiffFile took %s", elapsed)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("QA-PERF-05: DiffFile took %s, want <= 500ms", elapsed)
	}
}
