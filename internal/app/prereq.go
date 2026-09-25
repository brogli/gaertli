package app

import (
	"context"
	"errors"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// checkGit runs runner's "git --version" and reports whether it is on PATH
// and at least git.MinVersion. When it is not, msg is the native message to
// show: gitOutdated with the found and minimum versions when git ran but is
// older, missingGit otherwise (not on PATH, or its version output could not
// be parsed — the latter never happens with a real git).
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
