package app

import (
	"net/url"
	"path/filepath"

	"github.com/brogli/nebegit/internal/core"
)

// windowTitle formats a window's title: the worktree's display name (its
// last path segment) and, once known, the branch. path is "" for the
// empty-selection window, titled plain "nebegit".
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

// shortSHA is sha's first 7 characters, or all of a shorter sha.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// compareDiffURL builds the compare diff window's URL.
func compareDiffURL(wt core.WorktreeID, a, b, path, oldPath string) string {
	q := url.Values{"view": {"compare-diff"}, "wt": {string(wt)}, "from": {a}, "to": {b}, "file": {path}}
	if oldPath != "" {
		q.Set("oldFile", oldPath)
	}
	return "/?" + q.Encode()
}
