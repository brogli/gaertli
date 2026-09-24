// Package editor builds and starts the external editor command. Pure Go:
// used identically in-process (local) and inside nebegit-core --stdio (a
// WSL distro).
package editor

import (
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
)

// Default is the built-in editor profile.
func Default() core.EditorProfile {
	return core.EditorProfile{Name: "the editor", FolderCmd: "editor {path}", FileCmd: "editor {path}", FileAtLineCmd: "editor {path}:{line}"}
}

// OrDefault returns profile if it names a folder command, else Default().
// An unconfigured or never-pushed profile has an empty FolderCmd.
func OrDefault(profile core.EditorProfile) core.EditorProfile {
	if profile.FolderCmd != "" {
		return profile
	}
	return Default()
}

// OpenFolder starts profile's folder command for path, detached.
func OpenFolder(profile core.EditorProfile, path string) error {
	argv, err := buildCommand(profile.FolderCmd, path, 0)
	if err != nil {
		return err
	}
	return start(argv)
}

// OpenFile starts profile's file command for path, at line if line > 0.
func OpenFile(profile core.EditorProfile, path string, line int) error {
	argv, err := buildCommand(fileTemplate(profile, line), path, line)
	if err != nil {
		return err
	}
	return start(argv)
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
	argv, err := splitCommand(template)
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
