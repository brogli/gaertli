package services

import (
	"context"
	"strings"

	"github.com/brogli/gaertli/internal/git"
)

// headBranch returns HEAD's branch name, or detached if HEAD is not a
// symbolic ref. The prefix is cut here: --short prints heads/<name> when a
// tag has the same name.
func headBranch(ctx context.Context, r git.Runner, dir string) (branch string, detached bool, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"symbolic-ref", "-q", "HEAD"},
	})
	if quietFailure(err, result) {
		return "", true, nil
	}
	return strings.TrimPrefix(strings.TrimSpace(string(result.Stdout)), "refs/heads/"), false, err
}

// literalPath is a pathspec that matches path only, never as a pattern.
func literalPath(path string) string { return ":(literal)" + path }

// verifyRev resolves rev to its object name, "" if rev does not exist (an
// unborn HEAD, no stash).
func verifyRev(ctx context.Context, r git.Runner, dir, rev string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"rev-parse", "-q", "--verify", "--end-of-options", rev},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// revParse resolves ref to its SHA, read-only.
func revParse(ctx context.Context, r git.Runner, dir, ref string) (string, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: []string{"rev-parse", "--verify", "--end-of-options", ref}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}
