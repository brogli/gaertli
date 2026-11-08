// Package editor builds and starts the external editor command. Pure Go:
// used identically in-process (local) and inside gaertli-core --stdio (a
// WSL distro).
package editor

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/brogli/gaertli/internal/core"
)

// OpenFolder starts profile's folder command for path, detached. argv is
// the resolved command, for the caller's own log record; nil if it could
// not be built (err is then EditorNotFound, and nothing started).
func OpenFolder(profile core.EditorProfile, path string) (argv []string, err error) {
	argv, err = buildCommand(profile.FolderCmd, path, 0)
	if err != nil {
		return nil, err
	}
	return argv, start(argv)
}

// OpenFile starts profile's file command for path, at line if line > 0.
// argv is the resolved command, for the caller's own log record; nil if it
// could not be built (err is then EditorNotFound, and nothing started).
func OpenFile(profile core.EditorProfile, path string, line int) (argv []string, err error) {
	argv, err = buildCommand(fileTemplate(profile, line), path, line)
	if err != nil {
		return nil, err
	}
	return argv, start(argv)
}

// fileTemplate picks OpenFile's command template: FileAtLineCmd when line >
// 0 and set, else FileCmd.
func fileTemplate(profile core.EditorProfile, line int) string {
	if line > 0 && profile.FileAtLineCmd != "" {
		return profile.FileAtLineCmd
	}
	return profile.FileCmd
}

// buildCommand splits template and fills {path}/{line}. An empty template
// is EditorNotFound: nothing configured.
func buildCommand(template, path string, line int) ([]string, error) {
	if template == "" {
		return nil, core.Errorf(core.EditorNotFound, "no editor command configured")
	}
	argv, err := splitCommand(template, runtime.GOOS == "windows")
	if err != nil {
		return nil, core.ErrorfArgs(core.EditorNotFound, "editorCommandInvalid", map[string]string{"command": template}, "editor command %q: %v", template, err)
	}
	return fillPlaceholders(argv, path, line), nil
}

// fillPlaceholders replaces {path} in every argv element, and {line} too
// when line > 0; otherwise {line} is left literal.
func fillPlaceholders(argv []string, path string, line int) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		a = strings.ReplaceAll(a, "{path}", path)
		if line > 0 {
			a = strings.ReplaceAll(a, "{line}", strconv.Itoa(line))
		}
		out[i] = a
	}
	return out
}
