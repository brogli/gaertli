package services

import (
	"context"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// stashSubjectPrefix starts the subject of every stash entry the app makes around an integrate,
// so Summary's WaitingStash tells it apart from a stash made outside the app. git prefixes the
// branch ("On main: "); the verb (pull, merge or rebase) follows.
const stashSubjectPrefix = "Gärtli: local changes before "

// popStash pops the stash entry whose commit is sha.
func popStash(ctx context.Context, wt *session.Worktree, sha string) error {
	ref, err := resolveStashRef(ctx, wt.Runner, wt.Path, sha)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "pop", "--end-of-options", ref}})
	return err
}

// withStash runs integrate with local changes stashed around it, the entry's subject naming verb.
// git stash push exits 0 without an entry when there is nothing to save, so refs/stash is compared
// before and after the push. The new entry is popped by its commit, never by a bare "stash pop"
// that could take someone else's. A failed integrate that left the worktree untouched gives the
// entry back; any other failure keeps it. Caller holds wt's write lock.
func withStash(ctx context.Context, wt *session.Worktree, verb string, integrate func() error) error {
	before, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
	if err != nil {
		return err
	}
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "push", "-m", stashSubjectPrefix + verb}}); err != nil {
		return err
	}
	after, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
	if err != nil {
		return err
	}

	stashed := after != before
	var head string
	if stashed {
		if head, err = revParse(ctx, wt.Runner, wt.Path, "HEAD"); err != nil {
			return err
		}
	}
	if err := integrate(); err != nil {
		if stashed && untouched(ctx, wt, head) {
			_ = popStash(ctx, wt, after) // on failure the entry stays for the user
		}
		return err
	}
	if !stashed {
		return nil
	}
	return popStash(ctx, wt, after)
}

// PopStash implements core.API.
func (c *Core) PopStash(ctx context.Context, id core.WorktreeID, sha string) error {
	return c.stashVerb(ctx, id, "pop", sha)
}

// DropStash implements core.API.
func (c *Core) DropStash(ctx context.Context, id core.WorktreeID, sha string) error {
	return c.stashVerb(ctx, id, "drop", sha)
}

// stashVerb runs git stash verb on the entry whose commit is sha, resolved
// to its current stash@{n} under the same write lock the verb runs under.
func (c *Core) stashVerb(ctx context.Context, id core.WorktreeID, verb, sha string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	ref, err := resolveStashRef(ctx, wt.Runner, wt.Path, sha)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", verb, "--end-of-options", ref}})
	return err
}

// stashListFormat pairs each stash list entry's commit with its current
// stash@{n}, newest first.
const stashListFormat = "%H%x1f%gd"

// resolveStashRef finds sha's current stash@{n} in dir's stash list. Code
// Internal if sha names no current entry: a stale caller-supplied ID
// (already popped or dropped elsewhere), not a git failure.
func resolveStashRef(ctx context.Context, r git.Runner, dir, sha string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"stash", "list", "--format=" + stashListFormat, "-z"},
	})
	if err != nil {
		return "", err
	}
	for _, rec := range nulRecords(result.Stdout) {
		commit, ref, ok := strings.Cut(rec, "\x1f")
		if ok && commit == sha {
			return ref, nil
		}
	}
	return "", core.Errorf(core.Internal, "stash %s is no longer in the stash list", sha)
}

// waitingStash returns the commit of the newest stash entry the app made around an integrate, ""
// if none. git stash list is newest-first.
func waitingStash(ctx context.Context, r git.Runner, dir string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"stash", "list", "--format=%H%x1f%gs", "-z"},
	})
	if err != nil {
		return "", err
	}
	for _, rec := range nulRecords(result.Stdout) {
		sha, subject, ok := strings.Cut(rec, "\x1f")
		if ok && strings.Contains(subject, stashSubjectPrefix) {
			return sha, nil
		}
	}
	return "", nil
}
