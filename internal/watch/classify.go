package watch

import (
	"path/filepath"
	"strings"
)

// baseScopes maps a state file's base name to the Scope it belongs to
// (Design's table).
var baseScopes = map[string]Scope{
	"HEAD":             Refs,
	"packed-refs":      Refs,
	"index":            Index,
	"MERGE_HEAD":       State,
	"CHERRY_PICK_HEAD": State,
	"REVERT_HEAD":      State,
	"rebase-merge":     State,
	"rebase-apply":     State,
	"sequencer":        State,
}

// classify maps path, somewhere under commonDir, to the Scope it belongs
// to. false for a *.lock file (git's lockfile-then-rename pattern; the
// rename's target already classifies) and for anything not in the Design's
// table, including objects/, logs/, hooks/, info/.
func classify(commonDir, path string) (Scope, bool) {
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
		return Refs, true
	case rel == "worktrees":
		return Worktrees, true
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
// itself (Worktrees) or one of its own state files (same base names as the
// main worktree's, Design).
func classifyWorktreeChild(rel string) (Scope, bool) {
	parts := strings.Split(rel, "/") // "worktrees", "<name>"[, "<base>"]
	switch len(parts) {
	case 2:
		return Worktrees, true
	case 3:
		scope, ok := baseScopes[parts[2]]
		return scope, ok
	default:
		return "", false
	}
}
