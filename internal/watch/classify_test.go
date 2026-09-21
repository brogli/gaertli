package watch

import (
	"path/filepath"
	"testing"
)

func TestFR_REPO_04_ClassifyMapsPathsToTheirScope(t *testing.T) {
	commonDir := "/repo/.git"
	cases := []struct {
		path      string
		wantScope Scope
		wantOK    bool
	}{
		{"HEAD", Refs, true},
		{"refs/heads/x", Refs, true},
		{"packed-refs", Refs, true},
		{"index", Index, true},
		{"MERGE_HEAD", State, true},
		{"rebase-merge", State, true},
		{"worktrees/name/HEAD", Refs, true},
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
