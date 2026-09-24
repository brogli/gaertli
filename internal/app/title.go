package app

import "strings"

// windowTitle formats a window's title: the worktree's display name (its
// last path segment) and, once known, the branch. path is "" for the
// empty-selection window, titled plain "nebegit".
func windowTitle(path, branch string) string {
	if path == "" {
		return text(appName, nil)
	}
	name := displayName(path)
	if branch == "" {
		return name
	}
	return text(worktreeTitle, map[string]string{"name": name, "branch": branch})
}

// displayName is path's last segment, split on either separator: a WSL
// worktree's id is a UNC path, whatever the host OS.
func displayName(path string) string {
	return path[strings.LastIndexAny(path, `/\`)+1:]
}

// shortSHA is sha's first 7 characters, or all of a shorter sha.
func shortSHA(sha string) string {
	return sha[:min(len(sha), 7)]
}
