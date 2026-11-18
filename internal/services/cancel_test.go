// Requirements: FR-SYNC-07

package services

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/testwait"
)

func TestACancelledFetchLeavesNoLockAndTheNextFetchWorks(t *testing.T) {
	t.Parallel()
	repo, origin := cancelFixture(t)
	tip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")
	hook := stallRefUpdate(t, repo.Dir, "refs/remotes/origin/master")
	c, wt := openWorktree(t, repo.Dir)

	hook.cancel(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "origin")))

	assertNoLock(t, repo.Dir)
	hook.release()
	waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "origin")), core.JobDone)
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/origin/master"); core.SHA(got) != tip {
		t.Errorf("refs/remotes/origin/master = %q, want %q", got, tip)
	}
}

func TestACancelledPullLeavesNoLockAndTheNextPullWorks(t *testing.T) {
	t.Parallel()
	repo, origin := cancelFixture(t)
	tip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")
	hook := stallRefUpdate(t, repo.Dir, "refs/heads/master")
	c, wt := openWorktree(t, repo.Dir)

	hook.cancel(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)))

	assertNoLock(t, repo.Dir)
	hook.release()
	waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), core.JobDone)
	if got := gittest.HeadSHA(t, repo.Dir); got != tip {
		t.Errorf("HEAD = %q, want fast-forwarded to %q", got, tip)
	}
}

func TestACancelledPushLeavesNoLockAndTheNextPushWorks(t *testing.T) {
	t.Parallel()
	repo, origin := cancelFixture(t)
	gittest.Commit(t, repo.Dir, "local.txt", "l", "local commit")
	local := gittest.HeadSHA(t, repo.Dir)
	hook := stallRefUpdate(t, repo.Dir, "refs/remotes/origin/master")
	c, wt := openWorktree(t, repo.Dir)

	hook.cancel(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "", "")))

	assertNoLock(t, repo.Dir)
	hook.release()
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "", "")), core.JobDone)
	if got, _ := refSHA(t, origin, "refs/heads/master"); core.SHA(got) != local {
		t.Errorf("origin's master = %q, want %q", got, local)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/origin/master"); core.SHA(got) != local {
		t.Errorf("refs/remotes/origin/master = %q, want %q", got, local)
	}
}

// cancelFixture is pullMergeFixture where the test binary can interrupt
// git: on Windows the break event needs a parent without a console.
func cancelFixture(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("a test binary started from a console cannot send git the break event")
	}
	return pullMergeFixture(t)
}

// stalledHook is a reference-transaction hook that holds git inside a
// prepared transaction, while git holds the transaction's lock files.
type stalledHook struct {
	stalled     string // created once git waits in the hook
	hold        string // the hook waits while it exists
	interrupted string // created when the hook receives SIGTERM
}

// stallRefUpdate installs a stalledHook in dir for transactions touching
// ref. Cleanup releases it.
func stallRefUpdate(t *testing.T, dir, ref string) stalledHook {
	t.Helper()
	tmp := t.TempDir()
	h := stalledHook{
		stalled:     filepath.Join(tmp, "stalled"),
		hold:        filepath.Join(tmp, "hold"),
		interrupted: filepath.Join(tmp, "interrupted"),
	}
	writeFile(t, tmp, "hold", "")
	writeHook(t, dir, "reference-transaction", fmt.Sprintf(`[ "$1" = prepared ] || exit 0
refs=$(cat)
case "$refs" in *' %s'*) ;; *) exit 0 ;; esac
trap "touch '%s'; exit 143" TERM
touch '%s'
while [ -e '%s' ]; do sleep 0.05; done
`, ref, h.interrupted, h.stalled, h.hold))
	t.Cleanup(h.release)
	return h
}

func (h stalledHook) release() { _ = os.Remove(h.hold) }

// cancel waits until job's git stalls in the hook, cancels the job, waits
// for it to end cancelled and checks the interrupt reached the hook, not only
// git.
func (h stalledHook) cancel(t *testing.T, c *Core, wt core.WorktreeID, job string) {
	t.Helper()
	testwait.Until(func() bool {
		_, err := os.Stat(h.stalled)
		return err == nil
	})
	if err := c.CancelJob(t.Context(), wt, job); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, c, wt, job, core.JobCancelled)
	if _, err := os.Stat(h.interrupted); err != nil {
		t.Errorf("the hook got no SIGTERM, want it interrupted with git, not killed after the grace period: %v", err)
	}
}

// assertNoLock fails for every lock file left under dir's .git.
func assertNoLock(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(filepath.Join(dir, ".git"), func(path string, _ fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, ".lock") {
			t.Errorf("lock left behind: %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
