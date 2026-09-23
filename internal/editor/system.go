package editor

import (
	"runtime"

	"github.com/brogli/nebegit/internal/core"
)

// SystemProfile is the OS default opener: explorer on Windows (opens a
// folder in Explorer, a file in its associated default app); xdg-open on
// Linux, including a WSL-hosted worktree, since this package runs
// unmodified inside nebegit-core --stdio. No FileAtLineCmd: a bare OS
// opener has no line-number convention.
func SystemProfile() core.EditorProfile {
	if runtime.GOOS == "windows" {
		return core.EditorProfile{Name: "System default", FolderCmd: "explorer {path}", FileCmd: "explorer {path}"}
	}
	return core.EditorProfile{Name: "System default", FolderCmd: "xdg-open {path}", FileCmd: "xdg-open {path}"}
}
