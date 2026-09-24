package app

import (
	"log/slog"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// maxStderrLog bounds how much of a failed command's stderr reaches the app
// log.
const maxStderrLog = 4096 // 4 KiB

// logCommand writes one app-log record for a "command" event's payload:
// Info for a user-started command, Debug for a background one, Warn for a
// user-started command's failure.
func logCommand(logger *slog.Logger, worktree core.WorktreeID, e core.CommandRecord) {
	args := []any{"cmd", e.Command, "dir", e.Dir, "exit", e.ExitCode, "ms", e.DurationMS, "worktree", string(worktree)}
	if e.ErrorCode != "" {
		args = append(args, "stderr", maskAndTruncate(e.Stderr), "code", e.ErrorCode)
	}

	switch {
	case e.Background:
		logger.Debug("git", args...)
	case e.ErrorCode != "":
		logger.Warn("git", args...)
	default:
		logger.Info("git", args...)
	}
}

// maskAndTruncate masks an embedded credential and bounds the result to
// maxStderrLog.
func maskAndTruncate(stderr string) string {
	masked := git.MaskCredentials(stderr)
	return masked[:min(len(masked), maxStderrLog)]
}
