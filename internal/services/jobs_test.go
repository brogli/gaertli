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

func TestFR_SYNC_07_QA_RESP_02_JobsPollsGrowingProgressThenCancelKillsProcess(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	sess, err := c.sessions.Get(wt.ID)
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
	t.Setenv("FAKEGIT_STDERR_DELAY_MS", "200")
	t.Setenv("FAKEGIT_HEARTBEAT", heartbeat)

	sess.Repo.Jobs().Start("fetch", func(ctx context.Context, progress jobs.Progress) error {
		_, err := (git.Runner{Git: fakegit}).StreamErr(ctx, git.Call{Args: []string{"fetch"}}, func(line []byte) error {
			if percent, message, ok := git.ParseProgressLine(line); ok {
				progress(percent, message)
			}
			return nil
		})
		return err
	})

	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.Progress > 0 })

	if err := c.CancelJob(context.Background(), wt.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.State == "cancelled" })

	stopped := mtime(t, heartbeat)
	time.Sleep(300 * time.Millisecond)
	if got := mtime(t, heartbeat); !got.Equal(stopped) {
		t.Errorf("heartbeat mtime advanced after CancelJob: %v -> %v (process not killed)", stopped, got)
	}
}

// waitForJob polls Jobs until it returns a job matching want, and fails the
// test if none arrives within 3s.
func waitForJob(t *testing.T, c *Core, id core.WorktreeID, want func(core.JobInfo) bool) core.JobInfo {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		list, err := c.Jobs(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range list {
			if want(j) {
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a matching job")
	return core.JobInfo{}
}

func mtime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}
