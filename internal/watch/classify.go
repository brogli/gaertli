package watch

import (
	"path/filepath"
	"strings"

	"github.com/brogli/gaertli/internal/core"
)

// baseScopes maps a state file's base name to the scope it belongs to.
var baseScopes = map[string]core.ChangeScope{
	"HEAD":             core.RefsChanged,
	"packed-refs":      core.RefsChanged,
	"config":           core.RefsChanged, // e.g. a remote added or a branch's upstream set
	"index":            core.IndexChanged,
	"MERGE_HEAD":       core.StateChanged,
	"CHERRY_PICK_HEAD": core.StateChanged,
	"REVERT_HEAD":      core.StateChanged,
	"rebase-merge":     core.StateChanged,
	"rebase-apply":     core.StateChanged,
	"sequencer":        core.StateChanged,
}

// classify maps path, somewhere under commonDir, to the scope it belongs
// to. false for a *.lock file (git's lockfile-then-rename pattern; the
// rename's target already classifies) and for anything outside baseScopes,
// including objects/, logs/, hooks/, info/.
func classify(commonDir, path string) (core.ChangeScope, bool) {
	rel, err := filepath.Rel(commonDir, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasSuffix(rel, ".lock") {
		return "", false
	}

	switch {
	case rel == "refs" || strings.HasPrefix(rel, "refs/"):
		return core.RefsChanged, true
	case rel == "worktrees":
		return core.WorktreesChanged, true
	case strings.HasPrefix(rel, "worktrees/"):
		return classifyWorktreeChild(rel)
	case strings.Contains(rel, "/"):
		return "", false // nested elsewhere (logs/, objects/, hooks/, info/): not watched
	default:
		scope, ok := baseScopes[rel]
		return scope, ok
	}
}

// classifyWorktreeChild classifies a path under worktrees/: the directory
// itself (WorktreesChanged) or one of its own state files, whose base names match
// the main worktree's.
func classifyWorktreeChild(rel string) (core.ChangeScope, bool) {
	parts := strings.Split(rel, "/") // "worktrees", "<name>"[, "<base>"]
	switch len(parts) {
	case 2:
		return core.WorktreesChanged, true
	case 3:
		scope, ok := baseScopes[parts[2]]
		return scope, ok
	default:
		return "", false
	}
}
