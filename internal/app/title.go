package app

import (
	"net/url"
	"path/filepath"

	"github.com/brogli/nebegit/internal/core"
)

// windowTitle formats a window's title (FR-WT-09): the worktree's display
// name (its last path segment) and, once known, the branch. branch is ""
// until a later spec supplies one; path is "" for the no-argument
// placeholder window, kept as plain "nebegit" (spec 00's existing title).
func windowTitle(path, branch string) string {
	if path == "" {
		return "nebegit"
	}
	name := filepath.Base(path)
	if branch == "" {
		return name
	}
	return name + " [" + branch + "]"
}

// compareWindowTitle: the worktree's display name plus the two shortened
// SHAs — enough to tell two open diff windows for the same worktree apart.
func compareWindowTitle(path, a, b string) string {
	return filepath.Base(path) + ": compare " + shortSHA(a) + ".." + shortSHA(b)
}

// shortSHA bounds-checks rather than assuming len >= 7: this runs on
// whatever sha string the frontend already has, and a test fixture (or a
// future abbreviated hash) may be shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// compareDiffURL builds the compare diff window's URL (Design, "How the
// window is addressed", spec 23).
func compareDiffURL(wt core.WorktreeID, a, b, path, oldPath string) string {
	q := url.Values{"view": {"compare-diff"}, "wt": {string(wt)}, "from": {a}, "to": {b}, "file": {path}}
	if oldPath != "" {
		q.Set("oldFile", oldPath)
	}
	return "/?" + q.Encode()
}
