// Requirements: FR-REPO-04

package watch

import (
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestClassifyMapsPathsToTheirScope(t *testing.T) {
	commonDir := "/repo/.git"
	cases := []struct {
		path      string
		wantScope core.ChangeScope
		wantOK    bool
	}{
		{"HEAD", core.RefsChanged, true},
		{"refs/heads/x", core.RefsChanged, true},
		{"packed-refs", core.RefsChanged, true},
		{"config", core.RefsChanged, true},
		{"index", core.IndexChanged, true},
		{"MERGE_HEAD", core.StateChanged, true},
		{"rebase-merge", core.StateChanged, true},
		{"worktrees/name/HEAD", core.RefsChanged, true},
		{"index.lock", "", false},
		{"logs/HEAD", "", false},
	}

	for _, c := range cases {
		scope, ok := classify(commonDir, filepath.Join(commonDir, c.path))
		if scope != c.wantScope || ok != c.wantOK {
			t.Errorf("classify(%q) = (%q, %v), want (%q, %v)", c.path, scope, ok, c.wantScope, c.wantOK)
		}
	}
}
