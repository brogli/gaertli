package app

import (
	"log/slog"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// maxStderrLog bounds how much of a failed command's stderr reaches the app
// log.
const maxStderrLog = 4096

// logCommand writes one app-log record for a "command" event's payload:
// Warn for a failure, Debug for every success.
func logCommand(logger *slog.Logger, worktree core.WorktreeID, e core.CommandRecord) {
	args := []any{"cmd", e.Command, "dir", e.Dir, "exit", e.ExitCode, "ms", e.DurationMS, "worktree", string(worktree)}
	if e.ErrorCode != "" {
		args = append(args, "stderr", maskAndTruncate(e.Stderr), "code", e.ErrorCode)
		logger.Warn("git", args...)
		return
	}
	logger.Debug("git", args...)
}

// logEditor writes one app-log record for an "editor" event's payload:
// Debug for a start, Warn for one that failed.
func logEditor(logger *slog.Logger, worktree core.WorktreeID, e core.EditorRecord) {
	args := []any{"argv", e.Argv, "worktree", string(worktree)}
	if e.Error != "" {
		logger.Warn("editor", append(args, "err", e.Error)...)
		return
	}
	logger.Debug("editor", args...)
}

// maskAndTruncate masks an embedded credential and bounds the result to
// maxStderrLog.
func maskAndTruncate(stderr string) string {
	masked := git.MaskCredentials(stderr)
	return masked[:min(len(masked), maxStderrLog)]
}
