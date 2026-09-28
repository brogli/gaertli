package services

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brogli/nebegit/internal/git"
)

// branchInUse names where and how another worktree holds a branch: rebasing
// or bisecting it, the way a checked-out branch's own worktree already
// marks it.
type branchInUse struct {
	path, kind string // kind: "rebase" or "bisect"
}

// branchesInUse maps each branch some worktree of dir's repo is rebasing or
// bisecting to where and which. No git output names such a branch (worktree
// list shows only "detached"), so each detached worktree's own git dir is
// read for git's rebase and bisect state files. Rebase wins when both are
// present. A worktree whose .git cannot be read is skipped: git's own
// commands still refuse it with their own text. Read-only.
func branchesInUse(ctx context.Context, r git.Runner, dir string) (map[string]branchInUse, error) {
	worktrees, err := worktreeList(ctx, r, dir, false)
	if err != nil {
		return nil, err
	}
	uses := map[string]branchInUse{}
	for _, wt := range worktrees {
		if wt.Branch != "" || wt.Bare || wt.Missing {
			continue // not detached, or no working git dir to read
		}
		gitDir, ok := worktreeGitDir(wt.Path)
		if !ok {
			continue
		}
		if branch, ok := rebasingBranch(gitDir); ok {
			uses[branch] = branchInUse{path: wt.Path, kind: "rebase"}
		} else if branch, ok := bisectingBranch(gitDir); ok {
			uses[branch] = branchInUse{path: wt.Path, kind: "bisect"}
		}
	}
	return uses, nil
}

// worktreeGitDir resolves a worktree's own git dir from its top-level
// folder's .git: a directory for the main worktree, a "gitdir: <path>" file
// for a linked one, the path possibly relative to the worktree's own folder
// (worktree.useRelativePaths). ok is false if .git cannot be read.
func worktreeGitDir(worktreePath string) (dir string, ok bool) {
	dotGit := filepath.Join(worktreePath, ".git")
	info, err := os.Lstat(dotGit)
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
