package services

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/jobs"
	"github.com/brogli/nebegit/internal/session"
)

// remoteTimeout is git.Call's own safety-net Timeout for a remote call,
// generous on purpose: the job's own cancel through ctx is the real limit.
const remoteTimeout = time.Hour

// runWithProgress runs a --progress git.Call live, reporting every parsed
// line. A line with no percent keeps the last known percent and updates
// only the message, so a status line between two percent phases never
// blanks the job's progress bar.
func runWithProgress(ctx context.Context, r git.Runner, dir string, args []string, progress jobs.Progress) (git.Result, error) {
	percent := -1
	return r.StreamErr(ctx, git.Call{Dir: dir, Args: args, Timeout: remoteTimeout}, func(line []byte) error {
		if p, message, ok := git.ParseProgressLine(line); ok {
			percent = p
			progress(percent, message)
		} else {
			progress(percent, string(line))
		}
		return nil
	})
}

// Fetch implements core.API.
func (c *Core) Fetch(ctx context.Context, id core.WorktreeID, remote string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	args := []string{"fetch", "--prune", "--progress"}
	if remote == "" {
		args = append(args, "--all")
	} else {
		args = append(args, remote)
	}
	jobID := wt.Repo.Jobs().Start("fetch", func(ctx context.Context, progress jobs.Progress) error {
		result, err := runWithProgress(ctx, wt.Runner, wt.Path, args, progress)
		return remapAuthFailed(err, result)
	})
	return string(jobID), nil
}

// Pull implements core.API.
func (c *Core) Pull(ctx context.Context, id core.WorktreeID, mode core.PullMode) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	integrateArgs, err := pullIntegrateArgs(mode)
	if err != nil {
		return "", err
	}
	jobID := wt.Repo.Jobs().Start("pull", func(ctx context.Context, progress jobs.Progress) error {
		fetchResult, err := runWithProgress(ctx, wt.Runner, wt.Path, []string{"fetch", "--prune", "--progress"}, progress)
		if err := remapAuthFailed(err, fetchResult); err != nil {
			return err
		}

		wt.Lock() // only the local integration step writes this worktree's index/files
		defer wt.Unlock()
		_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: integrateArgs})
		return remapPullConflict(err, wt.GitDir)
	})
	return string(jobID), nil
}

// Push implements core.API.
func (c *Core) Push(ctx context.Context, id core.WorktreeID, remote string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	args := []string{"push", "--porcelain", "--progress"}
	if remote != "" {
		args = append(args, "--set-upstream", remote, "HEAD") // HEAD as the source sets up tracking against a same-named branch on the far side, with no extra call to resolve the current branch's own name
	}
	return startPushJob(wt, "push", args), nil
}

// ForcePush implements core.API.
func (c *Core) ForcePush(ctx context.Context, id core.WorktreeID) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	// Bare --force-with-lease, never =<ref>:<expect>, which would silently
	// disable --force-if-includes. Plain --force is never offered.
	args := []string{"push", "--porcelain", "--progress", "--force-with-lease", "--force-if-includes"}
	return startPushJob(wt, "force-push", args), nil
}

// startPushJob starts kind ("push" or "force-push") running args as a job,
// sharing Push's and ForcePush's classification: a rejection read from
// --porcelain's own machine-readable summary line on stdout, and a failed
// sign-in read from stderr.
func startPushJob(wt *session.Worktree, kind string, args []string) string {
	jobID := wt.Repo.Jobs().Start(kind, func(ctx context.Context, progress jobs.Progress) error {
		result, err := runWithProgress(ctx, wt.Runner, wt.Path, args, progress)
		return classifyPushRejection(remapAuthFailed(err, result), result)
	})
	return string(jobID)
}

// pullIntegrateArgs maps mode to its git call. mode is a closed UI choice
// (a two-option picker), never free text — an unrecognised value is this
// app's own bug, Code Internal, same convention as branch.go's resetFlag.
func pullIntegrateArgs(mode core.PullMode) ([]string, error) {
	switch mode {
	case core.PullMerge:
		return []string{"merge", "--no-edit", "@{upstream}"}, nil
	case core.PullRebase:
		return []string{"rebase", "@{upstream}"}, nil
	default:
		return nil, core.Errorf(core.Internal, "unknown pull mode %q", mode)
	}
}

// remapPullConflict classifies a failed merge/rebase step: a state file
// left behind (MERGE_HEAD or rebase-merge/rebase-apply) means it stopped
// mid-way, left in place for the resolve flow.
func remapPullConflict(err error, gitDir string) error {
	return remapIfPending(err, operation(gitDir) != "")
}

// pushRejectPrefix is --porcelain's own rejected-ref line prefix, always on
// stdout.
const pushRejectPrefix = "!\t"

// ffReasons are the porcelain rejection reasons meaning the remote has
// commits not yet fetched and integrated locally — classified
// NotFastForward. Every other explained rejection is PushRejected.
var ffReasons = []string{"fetch first", "non-fast-forward", "stale info", "remote ref updated since checkout"}

// classifyPushRejection reads Push/ForcePush's own git.Result after
// remapAuthFailed has already had a chance at it (an auth failure never
// reaches the per-ref summary line at all, so the two never compete).
func classifyPushRejection(err error, result git.Result) error {
	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.GitFailed {
		return err
	}
	reason, found := rejectReason(result.Stdout)
	if !found {
		return err // refused before any per-ref line printed (e.g. no upstream at all) — plain GitFailed
	}
	ce.Code = core.PushRejected
	for _, r := range ffReasons {
		if strings.Contains(reason, r) {
			ce.Code = core.NotFastForward
			break
		}
	}
	return err
}

// rejectReason extracts <reason> from --porcelain's own
// "!\t<from>:<to>\t[rejected] (<reason>)" line.
func rejectReason(stdout []byte) (string, bool) {
	for _, line := range strings.Split(string(stdout), "\n") {
		if !strings.HasPrefix(line, pushRejectPrefix) {
			continue
		}
		open, close := strings.LastIndex(line, "("), strings.LastIndex(line, ")")
		if open >= 0 && close > open {
			return line[open+1 : close], true
		}
	}
	return "", false
}

// authFailurePrefixes are the two known git stderr shapes for a failed
// sign-in — exit 128, shared with every unrelated fatal, so message-prefix
// matching is unavoidable.
var authFailurePrefixes = []string{"fatal: could not read Username for", "fatal: Authentication failed for"}

// remapAuthFailed classifies a failed sign-in. The message points at the
// user's own git credential setup and names no URL or credential text —
// git's own stderr, already carrying whatever it printed, stays available
// unchanged in GitStderr for "show details"; this replaces only the short
// summary shown up front.
func remapAuthFailed(err error, result git.Result) error {
	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.GitFailed {
		return err
	}
	for _, p := range authFailurePrefixes {
		if bytes.Contains(result.Stderr, []byte(p)) {
			ce.Code = core.AuthFailed
			ce.Message = "Could not sign in to the remote. Check your git credential setup (credential helper, SSH key, or cached login) outside the app — nebegit never asks for, stores, or sees credentials."
			break
		}
	}
	return err
}
