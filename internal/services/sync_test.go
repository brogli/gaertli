package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// bareRemote inits a bare repo at a fresh temp dir and adds it to repo as
// remote name (gittest has no bare-remote helper of its own).
func bareRemote(t *testing.T, repo *gittest.Repo, name string) string {
	t.Helper()
	dir := t.TempDir()
	gittest.Run(t, dir, "init", "--bare")
	gittest.Run(t, repo.Dir, "remote", "add", name, dir)
	return dir
}

// refSHA resolves ref in dir; ok is false if it does not exist.
func refSHA(t *testing.T, dir, ref string) (sha string, ok bool) {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"rev-parse", "-q", "--verify", "--end-of-options", ref},
	})
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(result.Stdout)), true
}

// remoteAppendCommit adds a commit on remoteDir's default branch via a
// throwaway clone, simulating a push from elsewhere without touching
// repo's own history. Returns the new commit's SHA.
func remoteAppendCommit(t *testing.T, remoteDir, file, content, msg string) string {
	t.Helper()
	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	gittest.Run(t, parent, "clone", remoteDir, clone)
	gittest.Commit(t, clone, file, content, msg)
	gittest.Run(t, clone, "push", "origin", "HEAD")
	return gittest.HeadSHA(t, clone)
}

// lastCommand returns the most recent command-log entry for wt whose
// Command starts with "git <verb>".
func lastCommand(t *testing.T, c *Core, wt core.WorktreeID, verb string) core.CommandLogEntry {
	t.Helper()
	list, err := c.CommandLog(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(list) - 1; i >= 0; i-- {
		if strings.HasPrefix(list[i].Command, "git "+verb) {
			return list[i]
		}
	}
	t.Fatalf("no %q command logged", verb)
	return core.CommandLogEntry{}
}

func TestFR_SYNC_04_ClassifyPushRejectionClassifiesByReason(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   core.Code
	}{
		{"fetch first", "!\trefs/heads/main:refs/heads/main\t[rejected] (fetch first)\n", core.NotFastForward},
		{"stale info", "!\trefs/heads/main:refs/heads/main\t[rejected] (stale info)\n", core.NotFastForward},
		{"remote ref updated since checkout", "!\trefs/heads/main:refs/heads/main\t[rejected] (remote ref updated since checkout)\n", core.NotFastForward},
		{"hook declined", "!\trefs/heads/main:refs/heads/main\t[rejected] (hook declined)\n", core.PushRejected},
		{"no rejected-ref line", "To origin\nDone\n", core.GitFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &core.Error{Code: core.GitFailed}
			got := classifyPushRejection(err, git.Result{Stdout: []byte(tt.stdout)})
			var ce *core.Error
			if !errors.As(got, &ce) || ce.Code != tt.want {
				t.Errorf("code = %v, want %v", ce, tt.want)
			}
		})
	}
}

func TestFR_SYNC_08_QA_SEC_02_RemapAuthFailedClassifiesVerifiedPrefixes(t *testing.T) {
	const url = "https://user:s3cret@example.com/repo.git"
	tests := []struct {
		name   string
		stderr string
		want   core.Code
	}{
		{"could not read username", "fatal: could not read Username for '" + url + "': terminal prompts disabled\n", core.AuthFailed},
		{"authentication failed", "fatal: Authentication failed for '" + url + "'\n", core.AuthFailed},
		{"unrelated fatal", "fatal: repository '" + url + "' not found\n", core.GitFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &core.Error{Code: core.GitFailed}
			got := remapAuthFailed(err, git.Result{Stderr: []byte(tt.stderr)})
			var ce *core.Error
			if !errors.As(got, &ce) || ce.Code != tt.want {
				t.Fatalf("code = %v, want %v", ce, tt.want)
			}
			if tt.want != core.AuthFailed {
				return
			}
			if strings.Contains(ce.Message, url) || strings.Contains(ce.Message, "s3cret") {
				t.Errorf("message leaks the URL or credential text: %q", ce.Message)
			}
			if !strings.Contains(ce.Message, "credential") {
				t.Errorf("message does not mention credential setup: %q", ce.Message)
			}
		})
	}
}

func TestFR_SYNC_02_PullIntegrateArgsMapsModeToGitArgs(t *testing.T) {
	if args, err := pullIntegrateArgs(core.PullMerge); err != nil || !slices.Equal(args, []string{"merge", "--no-edit", "@{upstream}"}) {
		t.Errorf("merge args = %v, %v", args, err)
	}
	if args, err := pullIntegrateArgs(core.PullRebase); err != nil || !slices.Equal(args, []string{"rebase", "@{upstream}"}) {
		t.Errorf("rebase args = %v, %v", args, err)
	}

	_, err := pullIntegrateArgs(core.PullMode("bogus"))
	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.Internal {
		t.Errorf("bogus mode err = %v, want Code Internal", err)
	}
}

func TestFR_SYNC_01_FetchOneRemotePrunesGoneBranchesLeavesOthersUntouched(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "feature")
	a := bareRemote(t, repo, "a")
	bareRemote(t, repo, "b")
	gittest.Run(t, repo.Dir, "push", "a", "master", "feature")
	gittest.Run(t, repo.Dir, "push", "b", "master")
	gittest.Run(t, repo.Dir, "fetch", "--all") // seeds both remotes' tracking refs, including a/feature

	gittest.Run(t, a, "branch", "-D", "feature") // deleted on a's remote
	gittest.Commit(t, repo.Dir, "b-only.txt", "y", "b commit")
	gittest.Run(t, repo.Dir, "push", "b", "master") // a new commit, b only — also updates refs/remotes/b/master locally, git push's own side effect

	bMaster, ok := refSHA(t, repo.Dir, "refs/remotes/b/master")
	if !ok {
		t.Fatal("refs/remotes/b/master missing after pushing to b")
	}

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Fetch(context.Background(), wt.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if _, ok := refSHA(t, repo.Dir, "refs/remotes/a/feature"); ok {
		t.Error("refs/remotes/a/feature still exists, want pruned")
	}
	if got, ok := refSHA(t, repo.Dir, "refs/remotes/b/master"); !ok || got != bMaster {
		t.Errorf("refs/remotes/b/master = %q, %v, want unchanged %q", got, ok, bMaster)
	}
}

func TestFR_SYNC_01_FetchAllUpdatesEveryRemote(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "a")
	bareRemote(t, repo, "b")
	gittest.Run(t, repo.Dir, "push", "a", "master")
	gittest.Run(t, repo.Dir, "push", "b", "master")

	gittest.Commit(t, repo.Dir, "a-and-b.txt", "x", "a commit")
	gittest.Run(t, repo.Dir, "push", "a", "master")
	aTip := gittest.HeadSHA(t, repo.Dir)

	gittest.Commit(t, repo.Dir, "b-only.txt", "y", "b commit")
	gittest.Run(t, repo.Dir, "push", "b", "master")
	bTip := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Fetch(context.Background(), wt.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if got, _ := refSHA(t, repo.Dir, "refs/remotes/a/master"); got != aTip {
		t.Errorf("refs/remotes/a/master = %q, want %q", got, aTip)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/b/master"); got != bTip {
		t.Errorf("refs/remotes/b/master = %q, want %q", got, bTip)
	}
}

// pullMergeFixture sets up a repo with an "origin" upstream already
// tracked by "master" — the common base every merge-mode case below
// diverges from.
func pullMergeFixture(t *testing.T) (repo *gittest.Repo, origin string) {
	t.Helper()
	repo = gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin = bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "master")
	return repo, origin
}

func TestFR_SYNC_02_PullMergeFastForwardsWhenUpstreamIsAhead(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullMerge)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
}

func TestFR_SYNC_02_PullMergeCreatesMergeCommitForNonConflictingDivergence(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "upstream-only.txt", "u", "upstream commit")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullMerge)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	parents := strings.Fields(gittest.Out(t, repo.Dir, "log", "-1", "--format=%P", "HEAD"))
	if len(parents) != 2 {
		t.Fatalf("HEAD parents = %v, want 2 (a real merge commit)", parents)
	}
	for _, f := range []string{"local-only.txt", "upstream-only.txt"} {
		if _, err := os.Stat(filepath.Join(repo.Dir, f)); err != nil {
			t.Errorf("%s missing from the working tree after the merge: %v", f, err)
		}
	}
}

func TestFR_SYNC_02_PullMergeLeavesMergeHeadOnConflict(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	// both sides edit seed.txt differently: a real content conflict
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	gittest.Commit(t, repo.Dir, "seed.txt", "local change\n", "local commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullMerge)
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.Conflicts {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.Conflicts)
	}
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Error("MERGE_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestFR_SYNC_02_PullMergeRefusesWhileDetached(t *testing.T) {
	repo, _ := pullMergeFixture(t)
	sha := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CheckoutCommit(context.Background(), wt.ID, sha); err != nil {
		t.Fatal(err)
	}

	jobID, err := c.Pull(context.Background(), wt.ID, core.PullMerge)
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.GitFailed {
		t.Errorf("job.Error = %+v, want Code %s, no crash", job.Error, core.GitFailed)
	}
}

func TestFR_SYNC_02_PullMergeRefusesWithNoUpstreamConfigured(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "origin") // added, never pushed — no upstream tracking

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullMerge)
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.GitFailed {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.GitFailed)
	}
}

func TestFR_SYNC_02_PullRebaseLinearisesNonConflictingDivergence(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	upstreamTip := remoteAppendCommit(t, origin, "upstream-only.txt", "u", "upstream commit")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullRebase)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	parents := strings.Fields(gittest.Out(t, repo.Dir, "log", "-1", "--format=%P", "HEAD"))
	if len(parents) != 1 {
		t.Fatalf("HEAD parents = %v, want 1 (linear history, no merge commit)", parents)
	}
	if parents[0] != upstreamTip {
		t.Errorf("HEAD's parent = %q, want the rebased-onto upstream tip %q", parents[0], upstreamTip)
	}
	for _, f := range []string{"local-only.txt", "upstream-only.txt"} {
		if _, err := os.Stat(filepath.Join(repo.Dir, f)); err != nil {
			t.Errorf("%s missing from the working tree after the rebase: %v", f, err)
		}
	}
}

func TestFR_SYNC_02_PullRebaseLeavesRebaseDirOnConflict(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	gittest.Commit(t, repo.Dir, "seed.txt", "local change\n", "local commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Pull(context.Background(), wt.ID, core.PullRebase)
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.Conflicts {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.Conflicts)
	}
	if !fileExists(filepath.Join(repo.Dir, ".git", "rebase-merge")) {
		t.Error("rebase-merge missing, want it left in place for the conflict flow")
	}
}

func TestFR_SYNC_03_PushPlainUpdatesTrackedUpstreamNoSetUpstream(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "local.txt", "x", "local commit")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Push(context.Background(), wt.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != localTip {
		t.Errorf("origin's master = %q, want %q", got, localTip)
	}
	if cmd := lastCommand(t, c, wt.ID, "push"); strings.Contains(cmd.Command, "--set-upstream") {
		t.Errorf("push command = %q, want no --set-upstream", cmd.Command)
	}
}

func TestFR_SYNC_03_PushSetUpstreamCreatesRemoteBranchAndTracks(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "switch", "-c", "feature")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Push(context.Background(), wt.ID, "origin")
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if got := gittest.Out(t, origin, "rev-parse", "feature"); got != localTip {
		t.Errorf("origin's feature = %q, want %q", got, localTip)
	}
	if got := gittest.Out(t, repo.Dir, "rev-parse", "--abbrev-ref", "feature@{upstream}"); got != "origin/feature" {
		t.Errorf("feature's upstream = %q, want origin/feature", got)
	}
}

func TestFR_SYNC_04_PushBehindUpstreamIsRejectedNotFastForward(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "elsewhere.txt", "x", "pushed from elsewhere") // never fetched here
	gittest.Commit(t, repo.Dir, "local.txt", "y", "local commit")                // diverges, unaware

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.Push(context.Background(), wt.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if job.Error != nil && job.Error.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
}

// forcePushRewriteFixture sets up a repo that has fetched and properly
// incorporated (merge --ff-only) origin's latest commit, then pushed and
// amended a further local commit — the common base cases 16 and 17
// diverge from.
func forcePushRewriteFixture(t *testing.T) (repo *gittest.Repo, origin string) {
	t.Helper()
	repo, origin = pullMergeFixture(t)
	remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")
	gittest.Run(t, repo.Dir, "fetch", "origin")
	gittest.Run(t, repo.Dir, "merge", "--ff-only", "origin/master")

	gittest.Commit(t, repo.Dir, "local.txt", "x", "local change")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	gittest.Run(t, repo.Dir, "commit", "--amend", "-m", "amended local change")
	return repo, origin
}

func TestFR_SYNC_05_ForcePushSucceedsAfterProperlyIncorporatingUpstream(t *testing.T) {
	repo, origin := forcePushRewriteFixture(t)
	rewrittenTip := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.ForcePush(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != rewrittenTip {
		t.Errorf("origin's master = %q, want the rewritten local tip %q", got, rewrittenTip)
	}
}

func TestFR_SYNC_05_ForcePushRefusedWhenRemoteGainedAnUnfetchedCommit(t *testing.T) {
	repo, origin := forcePushRewriteFixture(t)
	remoteAppendCommit(t, origin, "elsewhere.txt", "e", "elsewhere commit") // never fetched here
	remoteTip := gittest.Out(t, origin, "rev-parse", "master")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.ForcePush(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if got := gittest.Out(t, origin, "rev-parse", "master"); got != remoteTip {
		t.Errorf("origin's master = %q, want unchanged %q", got, remoteTip)
	}
}

// Named QA_SEC_02 first (rather than FR_SYNC_08, already credited by
// TestFR_SYNC_08_QA_SEC_02_RemapAuthFailedClassifiesVerifiedPrefixes above):
// tools/trace only reads the one requirement ID at a Go test name's own
// start.
func TestQA_SEC_02_FR_SYNC_08_FetchJobClassifiesAFailedSignIn(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "fatal: could not read Username for 'https://example.com/x.git': terminal prompts disabled")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	sess, err := c.sessions.Get(wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	sess.Runner.Git = fakegit

	jobID, err := c.Fetch(context.Background(), wt.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.AuthFailed {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.AuthFailed)
	}
	if strings.Contains(job.Error.Message, "example.com") || strings.Contains(job.Error.Message, "Username") {
		t.Errorf("message leaks the URL or credential text: %q", job.Error.Message)
	}
	if !strings.Contains(job.Error.Message, "credential") {
		t.Errorf("message does not mention credential setup: %q", job.Error.Message)
	}
}
