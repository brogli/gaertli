package services

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brogli/nebegit/internal/git"
)

// branchInUse names where and how a worktree holds a branch: rebasing or
// bisecting it, the way a checked-out branch's own worktree already marks
// it. own is true when that worktree is the one the caller (Refs or
// UpdateBranch) is running for, not another one.
type branchInUse struct {
	path, kind string // kind: "rebase" or "bisect"
	own        bool
}

// branchesInUse maps each branch some worktree of dir's repo is rebasing or
// bisecting to where and which. No git output names such a branch mid-
// rebase or mid-bisect (a detached worktree's %(worktreepath) is empty, and
// a worktree that later checked out another branch there is not detached at
// all), so every non-bare, non-missing worktree's own git dir is read for
// git's rebase and bisect state files; a worktree can be doing both at
// once, on different branches (e.g. a bisect left unfinished, then a
// checkout and rebase of another branch), so both are recorded. A worktree
// whose .git cannot be read is skipped: git's own commands still refuse it
// with their own text. Read-only; background selects an app-driven read
// over a user command, as worktreeList's own.
func branchesInUse(ctx context.Context, r git.Runner, dir string, background bool) (map[string]branchInUse, error) {
	worktrees, err := worktreeList(ctx, r, dir, background)
	if err != nil {
		return nil, err
	}
	dirInfo, _ := os.Stat(dir) // nil is fine: then no entry matches it below
	uses := map[string]branchInUse{}
	for _, wt := range worktrees {
		if wt.Bare || wt.Missing {
			continue
		}
		gitDir, ok := worktreeGitDir(wt.Path)
		if !ok {
			continue
		}
		own := sameWorktree(dirInfo, wt.Path)
		if branch, ok := rebasingBranch(gitDir); ok {
			uses[branch] = branchInUse{path: wt.Path, kind: "rebase", own: own}
		}
		if branch, ok := bisectingBranch(gitDir); ok {
			uses[branch] = branchInUse{path: wt.Path, kind: "bisect", own: own}
		}
	}
	return uses, nil
}

// sameWorktree reports whether path is the worktree dirInfo was Stat'd
// from: file identity, not string equality, since git's listed path and the
// caller's own need not be the same string for the same directory (e.g.
// worktree.useRelativePaths, a symlink).
func sameWorktree(dirInfo os.FileInfo, path string) bool {
	info, err := os.Stat(path)
	return dirInfo != nil && err == nil && os.SameFile(dirInfo, info)
}

// worktreeGitDir resolves a worktree's own git dir from its top-level
// folder's .git: a directory (or a symlink to one) for the main worktree, a
// "gitdir: <path>" file for a linked one, the path possibly relative to the
// worktree's own folder (worktree.useRelativePaths). ok is false if .git
// cannot be read.
func worktreeGitDir(worktreePath string) (dir string, ok bool) {
	dotGit := filepath.Join(worktreePath, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return dotGit, true
	}
	content, err := os.ReadFile(dotGit)
	if err != nil {
		return "", false
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir:")
	if !ok {
		return "", false
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(worktreePath, target)
	}
	return target, true
}

// rebasingBranch reads gitDir's rebase state (rebase-merge, the default and
// rebase -i, else rebase-apply): the branch its head-name names, "" if
// neither is running or the rebase was started on a detached HEAD
// (head-name then holds "detached HEAD", not a ref).
func rebasingBranch(gitDir string) (branch string, ok bool) {
	for _, sub := range []string{"rebase-merge", "rebase-apply"} {
		content, err := os.ReadFile(filepath.Join(gitDir, sub, "head-name"))
		if err != nil {
			continue
		}
		if b, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "refs/heads/"); ok {
			return b, true
		}
	}
	return "", false
}

// gitObjectID matches a full SHA-1 or SHA-256 object id: BISECT_START holds
// one when a bisect started on a detached HEAD, the branch's short name
// otherwise.
var gitObjectID = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)

// bisectingBranch reads gitDir's BISECT_START: the branch it names, "" if
// no bisect is running or it started on a detached HEAD.
func bisectingBranch(gitDir string) (branch string, ok bool) {
	content, err := os.ReadFile(filepath.Join(gitDir, "BISECT_START"))
	if err != nil {
		return "", false
	}
	name := strings.TrimSpace(string(content))
	if name == "" || gitObjectID.MatchString(name) {
		return "", false
	}
	return name, true
}
