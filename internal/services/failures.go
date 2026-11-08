package services

import (
	"errors"
	"slices"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// quietFailure reports whether err is a -q call's "nothing to report": exit
// 1 with empty stdout.
func quietFailure(err error, result git.Result) bool {
	return gitExited(err, result, 1) && len(result.Stdout) == 0
}

// gitFailure returns err's *core.Error if it is git's own failure (Code
// GitFailed), else nil.
func gitFailure(err error) *core.Error {
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Code == core.GitFailed {
		return ce
	}
	return nil
}

// gitExited reports whether err is git's own failure with exit code code.
func gitExited(err error, result git.Result, code int) bool {
	return gitFailure(err) != nil && result.ExitCode == code
}

func containsAny(s string, subs []string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}
