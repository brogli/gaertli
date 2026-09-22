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
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// bareRemote inits a bare repo at a fresh temp dir and adds it to repo as
// remote name.
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
	return remoteAppendCommitOnBranch(t, remoteDir, "", file, content, msg)
}

// remoteAppendCommitOnBranch is remoteAppendCommit on branch ("" for the
// remote's default branch).
func remoteAppendCommitOnBranch(t *testing.T, remoteDir, branch, file, content, msg string) string {
	t.Helper()
	return remoteAppendCommitFiles(t, remoteDir, branch, map[string]string{file: content}, msg)
}

// remoteAppendCommitFiles commits every file in files to branch ("" for the
// remote's default branch) in a single commit, so a merge/rebase step
// conflicts on all of them at once.
func remoteAppendCommitFiles(t *testing.T, remoteDir, branch string, files map[string]string, msg string) string {
	t.Helper()
	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	gittest.Run(t, parent, append(args, remoteDir, clone)...)
	for name, content := range files {
		writeFile(t, clone, name, content)
	}
	gittest.Run(t, clone, "add", "-A")
	gittest.CommitAll(t, clone, msg)
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

func TestFR_UPD_03_ClassifyDivergedFetchReclassifiesNonFastForward(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   core.Code
	}{
		{"non-fast-forward", " ! [rejected]        B          -> B  (non-fast-forward)\n", core.NotFastForward},
		{"unrelated refusal", "fatal: couldn't find remote ref B\n", core.GitFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &core.Error{Code: core.GitFailed}
			got := classifyDivergedFetch(err, git.Result{Stderr: []byte(tt.stderr)}, "B")
			var ce *core.Error
			if !errors.As(got, &ce) || ce.Code != tt.want {
				t.Fatalf("code = %v, want %v", ce, tt.want)
			}
			if tt.want == core.NotFastForward && !strings.Contains(ce.Message, `"B"`) {
				t.Errorf("message = %q, want it to name the branch", ce.Message)
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
	coretest.AssertCode(t, err, core.Internal)
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

// forcePushRewriteFixture sets up a repo that has fast-forwarded to
// origin's latest commit, then pushed and amended a further local commit.
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

// updateBranchFixture sets up a repo with an "origin" remote, this window
// on "master", and branch "B" tracking "origin/B", checked out nowhere.
func updateBranchFixture(t *testing.T) (repo *gittest.Repo, origin string) {
	t.Helper()
	repo = gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin = bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "master")
	gittest.Run(t, repo.Dir, "checkout", "-b", "B")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "B")
	gittest.Run(t, repo.Dir, "checkout", "master")
	return repo, origin
}

func updateBranchResult(t *testing.T, job core.JobInfo) core.UpdateBranchResult {
	t.Helper()
	result, ok := job.Result.(core.UpdateBranchResult)
	if !ok {
		t.Fatalf("job.Result = %#v (%T), want core.UpdateBranchResult", job.Result, job.Result)
	}
	return result
}

func TestFR_UPD_01_02_03_05_UpdateBranchFastForwardsABranchNotCheckedOutAnywhere(t *testing.T) {
	repo, origin := updateBranchFixture(t)
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")
	newTip := remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")
	mainBefore := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	result := updateBranchResult(t, job)
	if result.Outcome != "updated" || result.Commits <= 0 || result.To != newTip || result.From != beforeSHA {
		t.Errorf("Result = %+v, want Outcome updated, Commits > 0, From %q, To %q", result, beforeSHA, newTip)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != newTip {
		t.Errorf("refs/heads/B = %q, want %q", got, newTip)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != mainBefore {
		t.Errorf("HEAD (master) = %q, want unchanged %q", got, mainBefore)
	}
	if branch := gittest.Out(t, repo.Dir, "branch", "--show-current"); branch != "master" {
		t.Errorf("current branch = %q, want master (untouched)", branch)
	}
}

func TestFR_UPD_01_UpdateBranchFastForwardsABranchTrackingALocalBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "--track", "B", "master")
	gittest.Commit(t, repo.Dir, "next.txt", "y", "next")
	masterTip := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && (j.State == "done" || j.State == "failed") })
	if job.State != "done" {
		t.Fatalf("job = %+v, want done", job)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != masterTip {
		t.Errorf("refs/heads/B = %q, want master's tip %q", got, masterTip)
	}
}

func TestFR_UPD_05_UpdateBranchAlreadyAtUpstreamTipReportsUpToDate(t *testing.T) {
	repo, _ := updateBranchFixture(t)
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	result := updateBranchResult(t, job)
	if result.Outcome != "upToDate" || result.From != beforeSHA || result.To != beforeSHA || result.Commits != 0 {
		t.Errorf("Result = %+v, want Outcome upToDate, From == To == %q, Commits 0", result, beforeSHA)
	}
}

func TestFR_UPD_03_UpdateBranchDivergedFromUpstreamFailsNotFastForward(t *testing.T) {
	repo, origin := updateBranchFixture(t)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream-only.txt", "u", "upstream commit")
	gittest.Run(t, repo.Dir, "checkout", "B")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")
	localTip := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != localTip {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, localTip)
	}
}

func TestFR_UPD_05_UpdateBranchWithNoUpstreamFailsBeforeAnyGitCall(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "B")
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	// Refused before any job starts (a precondition, not a git failure):
	// UpdateBranch itself returns the error, with no job to poll.
	_, err = c.UpdateBranch(context.Background(), wt.ID, "B")
	coretest.AssertCode(t, err, core.NoUpstream)
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
}

// updateBranchElsewhereFixture is updateBranchFixture with "B" checked out
// in a second linked worktree (otherDir) rather than nowhere.
func updateBranchElsewhereFixture(t *testing.T) (repo *gittest.Repo, origin, otherDir string) {
	t.Helper()
	repo, origin = updateBranchFixture(t)
	otherDir = filepath.Join(t.TempDir(), "other")
	gittest.Run(t, repo.Dir, "worktree", "add", otherDir, "B")
	return repo, origin, otherDir
}

func TestFR_UPD_04_UpdateBranchCheckedOutElsewhereFastForwardsItsFilesToo(t *testing.T) {
	repo, origin, otherDir := updateBranchElsewhereFixture(t)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "done" })

	result := updateBranchResult(t, job)
	if result.Outcome != "updated" || result.Commits <= 0 {
		t.Errorf("Result = %+v, want Outcome updated, Commits > 0", result)
	}
	content, err := os.ReadFile(filepath.Join(otherDir, "upstream.txt"))
	if err != nil || string(content) != "u" {
		t.Errorf("other worktree's upstream.txt = %q, %v, want \"u\" (files followed the update)", content, err)
	}
}

func TestFR_UPD_04_UpdateBranchCheckedOutElsewhereRefusesWhenThatWorktreeIsDirty(t *testing.T) {
	repo, origin, otherDir := updateBranchElsewhereFixture(t)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")
	if err := os.WriteFile(filepath.Join(otherDir, "seed.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := c.UpdateBranch(context.Background(), wt.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, c, wt.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })

	if job.Error == nil || job.Error.Code != core.DirtyWorktree {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.DirtyWorktree)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
	content, err := os.ReadFile(filepath.Join(otherDir, "seed.txt"))
	if err != nil || string(content) != "dirty" {
		t.Errorf("other worktree's seed.txt = %q, %v, want unchanged \"dirty\"", content, err)
	}
}
