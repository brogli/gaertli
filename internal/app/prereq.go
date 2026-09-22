package app

import (
	"fmt"
	"os/exec"
)

// gitPresent reports whether a git executable is on PATH (QA-INST-01).
// Not a version check: TR-04's ≥ 2.40 minimum is enforced when a repo is
// opened.
func gitPresent() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// missingGitMessage is shown when gitPresent is false.
func missingGitMessage() string {
	return "nebegit needs git. Install git and make sure it is on your PATH, then start nebegit again."
}

// startupFailedMessage is shown when a.Run() fails after git was already
// confirmed present: the webview prerequisite (QA-INST-01).
func startupFailedMessage(err error) string {
	return fmt.Sprintf("nebegit could not start its web view. On Windows, install the WebView2 runtime; on Linux, install the webkitgtk package. (%v)", err)
}
