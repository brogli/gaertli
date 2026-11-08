package app

import "strings"

// textID names a shell-native text: a window title, a native dialog's
// title or message, or the startup prerequisite message.
type textID string

const (
	appName            textID = "appName" // every dialog's title, and the empty-selection window's
	usage              textID = "usage"
	missingGit         textID = "missingGit"
	settingsPathErr    textID = "settingsPathErr"
	settingsLoadErr    textID = "settingsLoadErr"
	startupFailed      textID = "startupFailed" // a.Run failed after git was found: the web view is missing
	deployConsent      textID = "deployConsent"
	worktreeTitle      textID = "worktreeTitle"
	diffCommitTitle    textID = "diffCommitTitle"
	diffRangeTitle     textID = "diffRangeTitle"
	diffAgainstWorking textID = "diffAgainstWorkingTitle"
	diffWorkingTitle   textID = "diffWorkingTitle"
	diffConflictTitle  textID = "diffConflictTitle"
)

// texts holds the shell's own text, in English.
var texts = map[textID]string{
	appName:            "Gärtli",
	usage:              "usage: gaertli [path]",
	missingGit:         "Gärtli needs git. Install git and make sure it is on your PATH, then start Gärtli again.",
	settingsPathErr:    "could not determine the settings location: {error}",
	settingsLoadErr:    "could not load settings: {error}",
	startupFailed:      "Gärtli could not start its web view. On Windows, install the WebView2 runtime; on Linux, install the webkitgtk package. ({error})",
	deployConsent:      "Gärtli needs a small helper inside the WSL distro {distro} to work with repos there. Install it now?",
	worktreeTitle:      "{name} [{branch}]",
	diffCommitTitle:    "{name}: {sha}",
	diffRangeTitle:     "{name}: compare {from}..{to}",
	diffAgainstWorking: "{name}: compare {sha}..working tree",
	diffWorkingTitle:   "{name}: changes",
	diffConflictTitle:  "{name}: conflict in {file}",
}

// text renders id's English text, replacing each {key} placeholder with
// args[key].
func text(id textID, args map[string]string) string {
	s := texts[id]
	for k, v := range args {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
