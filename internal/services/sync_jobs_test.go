// Requirements: FR-SYNC-07

package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/jobs"
	"github.com/brogli/gaertli/internal/testwait"
)

func TestJobsPollsGrowingProgressThenCancelKillsProcess(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	c, wt := openWorktree(t, repo.Dir)
	sess, err := c.sessions.Get(wt)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "stderr-records")
	// Far more records than the test lives to see: only a kill ends fakegit.
	data := strings.Repeat("remote: Counting objects:  50% (45/90)   \r", 3000)
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STDERR_FILE", file)
	t.Setenv("FAKEGIT_STDERR_DELAY_MS", "200")

	returned := make(chan struct{})
	sess.Repo.Jobs().Start(t.Context(), "fetch", func(ctx context.Context, progress jobs.Progress) (any, error) {
		defer close(returned)
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
	waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.State == core.JobCancelled })

	<-returned // StreamErr returns only once the process is gone
}

// waitForJob polls Jobs until it returns a job matching want, and fails the
// test if none arrives in time.
func waitForJob(t *testing.T, c *Core, id core.WorktreeID, want func(core.JobInfo) bool) core.JobInfo {
	t.Helper()
	var match core.JobInfo
	testwait.Until(func() bool {
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

// waitForJobDone waits for job id to end and fails the test unless it ended
// done.
func waitForJobDone(t *testing.T, c *Core, wt core.WorktreeID, id string) {
	t.Helper()
	job := waitForJob(t, c, wt, func(j core.JobInfo) bool {
		return j.ID == id && (j.State == core.JobDone || j.State == core.JobFailed)
	})
	if job.State != core.JobDone {
		t.Fatalf("job failed: %+v, want done", job.Error)
	}
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
