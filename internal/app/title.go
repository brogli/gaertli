package app

import "path/filepath"

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
