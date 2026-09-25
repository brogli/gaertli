package services

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_SYNC_07_ACancelledFetchLeavesNoLockAndTheNextFetchWorks(t *testing.T) {
	repo, origin := cancelFixture(t)
	tip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")
	hook := stallRefUpdate(t, repo.Dir, "refs/remotes/origin/master")
	c, wt := openWorktree(t, repo.Dir)

	hook.cancel(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "origin")))

	assertNoLock(t, repo.Dir)
	hook.release()
	waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "origin")), core.JobDone)
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/origin/master"); got != tip {
		t.Errorf("refs/remotes/origin/master = %q, want %q", got, tip)
	}
}

func TestFR_SYNC_07_ACancelledPullLeavesNoLockAndTheNextPullWorks(t *testing.T) {
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

func TestFR_SYNC_07_ACancelledPushLeavesNoLockAndTheNextPushWorks(t *testing.T) {
	repo, origin := cancelFixture(t)
	gittest.Commit(t, repo.Dir, "local.txt", "l", "local commit")
	local := gittest.HeadSHA(t, repo.Dir)
	hook := stallRefUpdate(t, repo.Dir, "refs/remotes/origin/master")
	c, wt := openWorktree(t, repo.Dir)

	hook.cancel(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "")))

	assertNoLock(t, repo.Dir)
	hook.release()
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "")), core.JobDone)
	if got, _ := refSHA(t, origin, "refs/heads/master"); got != local {
		t.Errorf("origin's master = %q, want %q", got, local)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/origin/master"); got != local {
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
	stalled string // created once git waits in the hook
	hold    string // the hook waits while it exists
}

// stallRefUpdate installs a stalledHook in dir for transactions touching
// ref. Cleanup releases it.
func stallRefUpdate(t *testing.T, dir, ref string) stalledHook {
	t.Helper()
	tmp := t.TempDir()
	h := stalledHook{stalled: filepath.Join(tmp, "stalled"), hold: filepath.Join(tmp, "hold")}
	writeFile(t, tmp, "hold", "")
	writeHook(t, dir, "reference-transaction", fmt.Sprintf(`[ "$1" = prepared ] || exit 0
refs=$(cat)
case "$refs" in *' %s'*) ;; *) exit 0 ;; esac
touch '%s'
while [ -e '%s' ]; do sleep 0.05; done
`, ref, h.stalled, h.hold))
	t.Cleanup(h.release)
	return h
}

func (h stalledHook) release() { _ = os.Remove(h.hold) }

// cancel waits until job's git stalls in the hook, cancels the job and
// waits for it to end cancelled.
func (h stalledHook) cancel(t *testing.T, c *Core, wt core.WorktreeID, job string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for _, err := os.Stat(h.stalled); err != nil; _, err = os.Stat(h.stalled) {
		if time.Now().After(deadline) {
			t.Fatal("git did not reach the stalling hook within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	start := time.Now()
	if err := c.CancelJob(t.Context(), wt, job); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, c, wt, job, core.JobCancelled)
	if took := time.Since(start); took > time.Second {
		t.Errorf("cancel took %s, want the hook stopped with git, not its pipe cut off after the grace period", took)
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
