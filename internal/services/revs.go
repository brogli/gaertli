package services

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/brogli/gaertli/internal/core"
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

// nulRecords splits a %00-terminated git stream into records, stripping the
// '\n' git appends after each one and dropping empties.
func nulRecords(out []byte) []string {
	var records []string
	for record := range bytes.SplitSeq(out, []byte{0}) {
		record = bytes.TrimPrefix(record, []byte("\n"))
		if len(record) > 0 {
			records = append(records, string(record))
		}
	}
	return records
}

// worktreeFile joins path onto dir. A path that resolves outside dir is
// Code Internal: path is caller-supplied and, unlike a revision or
// pathspec, never validated by git. A file deleted or moved away since the
// diff being viewed is Code FileNotFound.
func worktreeFile(dir, path string) (string, error) {
	full := filepath.Join(dir, path)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", core.Errorf(core.Internal, "path escapes the worktree: %s", path)
	}
	if !fileExists(full) {
		return "", core.ErrorfArgs(core.FileNotFound, "fileNotFound", map[string]string{"path": path}, "%s no longer exists in this worktree", path)
	}
	return full, nil
}
