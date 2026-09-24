package app

import "os/exec"

// gitPresent reports whether a git executable is on PATH. The minimum
// version is checked when a repo is opened.
func gitPresent() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
