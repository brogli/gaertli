package app

import (
	"net/url"

	"github.com/brogli/nebegit/internal/core"
)

// DiffKind is what a diff window lists and diffs.
type DiffKind string

const (
	CommitDiff         DiffKind = "commit"         // a commit's changes: Sha
	RangeDiff          DiffKind = "range"          // two commits compared: From, To
	AgainstWorkingDiff DiffKind = "againstWorking" // a commit compared with the working tree: Sha
	WorkingDiff        DiffKind = "working"        // the working tree's changes: Staged picks the side
)

// DiffTarget is what a diff window shows: its kind's files, starting at
// Path (OldPath for a rename).
type DiffTarget struct {
	Kind   DiffKind `json:"kind"`
	Sha    string   `json:"sha,omitempty"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
	Staged bool     `json:"staged,omitempty"`
	// Conflicted marks a conflicted file of the working tree: the window
	// shows its conflict, not a diff.
	Conflicted bool   `json:"conflicted,omitempty"`
	Path       string `json:"path"`
	OldPath    string `json:"oldPath,omitempty"`
}

// diffWindowKey names a diff window by its main window's id, or, for a
// call from no window, by its worktree.
type diffWindowKey struct {
	mainWindow uint
	wt         core.WorktreeID
}

// diffWindowTitle is the worktree's display name and what the window shows.
func diffWindowTitle(wt core.WorktreeID, t DiffTarget) string {
	name := displayName(string(wt))
	switch t.Kind {
	case CommitDiff:
		return text(diffCommitTitle, map[string]string{"name": name, "sha": shortSHA(t.Sha)})
	case RangeDiff:
		return text(diffRangeTitle, map[string]string{"name": name, "from": shortSHA(t.From), "to": shortSHA(t.To)})
	case AgainstWorkingDiff:
		return text(diffAgainstWorking, map[string]string{"name": name, "sha": shortSHA(t.Sha)})
	default:
		if t.Conflicted {
			return text(diffConflictTitle, map[string]string{"name": name, "file": t.Path})
		}
		return text(diffWorkingTitle, map[string]string{"name": name})
	}
}

// diffURL builds a diff window's URL; empty fields stay out of it.
func diffURL(wt core.WorktreeID, t DiffTarget) string {
	q := url.Values{"view": {"diff"}, "wt": {string(wt)}, "kind": {string(t.Kind)}, "file": {t.Path}}
	for name, value := range map[string]string{"sha": t.Sha, "from": t.From, "to": t.To, "oldFile": t.OldPath} {
		if value != "" {
			q.Set(name, value)
		}
	}
	if t.Staged {
		q.Set("staged", "true")
	}
	if t.Conflicted {
		q.Set("conflicted", "true")
	}
	return "/?" + q.Encode()
}
