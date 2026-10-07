package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/jobs"
)

func TestFR_SYNC_07_JobsPollsGrowingProgressThenCancelKillsProcess(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	c, wt := openWorktree(t, repo.Dir)
	sess, err := c.sessions.Get(wt)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "stderr-records")
	data := "remote: Counting objects:   1% (1/90)   \r" +
		"remote: Counting objects:  50% (45/90)   \r" +
		"remote: Counting objects: 100% (90/90), done.\n"
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	heartbeat := filepath.Join(t.TempDir(), "heartbeat")
	t.Setenv("FAKEGIT_STDERR_FILE", file)
	t.Setenv("FAKEGIT_STDERR_DELAY_MS", "50")
	t.Setenv("FAKEGIT_HEARTBEAT", heartbeat)

	sess.Repo.Jobs().Start(t.Context(), "fetch", func(ctx context.Context, progress jobs.Progress) (any, error) {
		_, err := (git.Runner{Git: fakegit}).StreamErr(ctx, git.Call{Args: []string{"fetch"}}, func(line []byte) error {
			if percent, message, ok := git.ParseProgressLine(line); ok {
				progress(percent, message)
			}
			return nil
		})
		return nil, err
	})

	job := waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.Progress > 0 })

	if err := c.CancelJob(t.Context(), wt, job.ID); err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.State == "cancelled" })

	stopped := mtime(t, heartbeat)
	time.Sleep(100 * time.Millisecond)
	if got := mtime(t, heartbeat); !got.Equal(stopped) {
		t.Errorf("heartbeat mtime advanced after CancelJob: %v -> %v (process not killed)", stopped, got)
	}
}

// waitForJob polls Jobs until it returns a job matching want, and fails the
// test if none arrives in time.
func waitForJob(t *testing.T, c *Core, id core.WorktreeID, want func(core.JobInfo) bool) core.JobInfo {
	t.Helper()
	var match core.JobInfo
	waitFor(t, func() bool {
		list, err := c.Jobs(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range list {
			if want(j) {
				match = j
				return true
			}
		}
		return false
	})
	return match
}

// waitForJobState waits for job id to reach state.
func waitForJobState(t *testing.T, c *Core, wt core.WorktreeID, id string, state core.JobState) core.JobInfo {
	t.Helper()
	return waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.ID == id && j.State == state })
}

// jobID returns a func taking a job start's results: the job's id, or the
// test fails on the start's error.
func jobID(t *testing.T) func(string, error) string {
	t.Helper()
	return func(id string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
}

func mtime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}
