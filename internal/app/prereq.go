package app

import (
	"context"
	"errors"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// checkGit reports whether runner's git runs and is at least
// git.MinVersion. When not, msg is the native message to show: gitOutdated
// with both versions when git ran but is older, else missingGit.
func checkGit(ctx context.Context, runner git.Runner) (ok bool, msg string) {
	_, err := runner.CheckVersion(ctx)
	if err == nil {
		return true, ""
	}
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Key == "gitTooOld" {
		return false, text(gitOutdated, ce.Args)
	}
	return false, text(missingGit, nil)
}
