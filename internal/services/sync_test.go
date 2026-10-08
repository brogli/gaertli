package services

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	result, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"rev-parse", "-q", "--verify", "--end-of-options", ref},
	})
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(result.Stdout)), true
}

// stashPush stashes dir's changes as msg and returns the new entry's commit.
func stashPush(t *testing.T, dir, msg string) string {
	t.Helper()
	gittest.Run(t, dir, "stash", "push", "-m", msg)
	return gittest.Out(t, dir, "rev-parse", "refs/stash")
}

// remoteAppendCommit pushes a commit to remoteDir's default branch from a
// throwaway clone and returns its SHA.
func remoteAppendCommit(t *testing.T, remoteDir, file, content, msg string) string {
	t.Helper()
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

// lastCommand returns the most recent git call rec recorded whose Command
// starts with "git <verb>".
func lastCommand(t *testing.T, rec *commandRecorder, verb string) git.Entry {
	t.Helper()
	list := rec.snapshot()
	for _, l := range slices.Backward(list) {
		if strings.HasPrefix(l.Command, "git "+verb) {
			return l
		}
	}
	t.Fatalf("no %q command logged", verb)
	return git.Entry{}
}

func TestFR_SYNC_04_ClassifyPushRejectionClassifiesByReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		stdout    string
		want      core.Code
		wantInMsg string // substring the message must name; "" skips the check
		wantKey   string // "" for the passthrough case: no remap happened
	}{
		{"fetch first", "!\trefs/heads/main:refs/heads/main\t[rejected] (fetch first)\n", core.NotFastForward, "fetch first", "pushFetchFirst"},
		{"non-fast-forward", "!\trefs/heads/main:refs/heads/main\t[rejected] (non-fast-forward)\n", core.NotFastForward, "non-fast-forward", "pushNotFastForward"},
		{"stale info", "!\trefs/heads/main:refs/heads/main\t[rejected] (stale info)\n", core.NotFastForward, "stale info", "pushStaleInfo"},
		{"remote ref updated since checkout", "!\trefs/heads/main:refs/heads/main\t[rejected] (remote ref updated since checkout)\n", core.NotFastForward, "remote ref updated since checkout", "pushRefUpdatedSinceCheckout"},
		{"hook declined", "!\trefs/heads/main:refs/heads/main\t[rejected] (hook declined)\n", core.PushRejected, "hook declined", "pushRejected"},
		{"no rejected-ref line", "To origin\nDone\n", core.GitFailed, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &core.Error{Code: core.GitFailed}
			got := classifyPushRejection(err, git.Result{Stdout: []byte(tt.stdout)}, false)
			ce := coretest.AsError(t, got, tt.want)
			if tt.wantInMsg != "" && !strings.Contains(ce.Message, tt.wantInMsg) {
				t.Errorf("message = %q, want it to name the reason %q", ce.Message, tt.wantInMsg)
			}
			if ce.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q", ce.Key, tt.wantKey)
			}
			if tt.want == core.PushRejected && ce.Args["reason"] != "hook declined" {
				t.Errorf("Args[reason] = %q, want %q", ce.Args["reason"], "hook declined")
			}
		})
	}
}

func TestFR_UPD_03_ClassifyDivergedFetchReclassifiesNonFastForward(t *testing.T) {
	t.Parallel()
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
			ce := coretest.AsError(t, got, tt.want)
			if tt.want == core.NotFastForward {
				if !strings.Contains(ce.Message, `"B"`) {
					t.Errorf("message = %q, want it to name the branch", ce.Message)
				}
				if ce.Key != "branchDiverged" || ce.Args["branch"] != "B" {
					t.Errorf("Key/Args = %q/%+v, want branchDiverged/branch=B", ce.Key, ce.Args)
				}
			} else if ce.Key != "" {
				t.Errorf("Key = %q, want empty (no remap happened)", ce.Key)
			}
		})
	}
}

func TestFR_SYNC_08_QA_SEC_02_RemapAuthFailedClassifiesVerifiedPrefixes(t *testing.T) {
	t.Parallel()
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
			ce := coretest.AsError(t, got, tt.want)
			if tt.want != core.AuthFailed {
				if ce.Key != "" {
					t.Errorf("Key = %q, want empty (no remap happened)", ce.Key)
				}
				return
			}
			if strings.Contains(ce.Message, url) || strings.Contains(ce.Message, "s3cret") {
				t.Errorf("message leaks the URL or credential text: %q", ce.Message)
			}
			for _, hint := range []string{"from a terminal", "never asks for, stores or sees credentials"} {
				if !strings.Contains(ce.Message, hint) {
					t.Errorf("message lacks %q: %q", hint, ce.Message)
				}
			}
			if ce.Key != "authFailed" {
				t.Errorf("Key = %q, want authFailed", ce.Key)
			}
		})
	}
}

func TestFR_SYNC_09_ClassifyDirtyIntegrateReclassifiesRebaseAndMergeRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stderr string
		want   core.Code
	}{
		{"rebase unstaged", "error: cannot rebase: You have unstaged changes.\nerror: Please commit or stash them.\n", core.DirtyWorktree},
		{"rebase staged", "error: cannot rebase: Your index contains uncommitted changes.\nerror: Please commit or stash them.\n", core.DirtyWorktree},
		{"merge overwritten", "error: Your local changes to the following files would be overwritten by merge:\n\tf.txt\nPlease commit your changes or stash them before you merge.\nAborting\n", core.DirtyWorktree},
		{"unrelated refusal", "fatal: no upstream configured for branch 'master'\n", core.GitFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &core.Error{Code: core.GitFailed}
			coretest.AssertCode(t, classifyDirtyIntegrate(err, git.Result{Stderr: []byte(tt.stderr)}, "pull"), tt.want)
		})
	}
}

// Only a local change to a file the incoming commits also touch blocks a
// fast-forward pull.
func TestFR_SYNC_09_PullReclassifiesAFastForwardBlockedByLocalChangesAsDirtyWorktree(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	writeFile(t, repo.Dir, "seed.txt", "local uncommitted change\n") // unstaged, not committed: still a pure fast-forward

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil || job.Error.Code != core.DirtyWorktree || job.Error.Key != "pullBlockedByChanges" {
		t.Errorf("job.Error = %+v, want Code %s, key pullBlockedByChanges", job.Error, core.DirtyWorktree)
	}
	if job.Error != nil && !strings.Contains(job.Error.GitStderr, "would be overwritten by merge") {
		t.Errorf("GitStderr = %q, want git's own refusal kept for the details", job.Error.GitStderr)
	}
	if content := readFile(t, repo.Dir, "seed.txt"); content != "local uncommitted change\n" {
		t.Errorf("seed.txt = %q, want the local change untouched", content)
	}
}

func TestFR_SYNC_09_PullWithStashStashesIntegratesAndPopsBackOnSuccess(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	upstreamTip := remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	// a change to a tracked file the incoming commit does not touch: git
	// stash push without -u stashes tracked files only
	writeFile(t, repo.Dir, "other.txt", "local uncommitted change\n")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.PullWithStash(t.Context(), wt, core.PullMerge)), "done")

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
	if content := readFile(t, repo.Dir, "other.txt"); content != "local uncommitted change\n" {
		t.Errorf("other.txt = %q, want the local change restored, unchanged", content)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
}

// On a clean worktree git stash push creates no entry; a pop would take the
// user's own stash.
func TestFR_SYNC_09_PullWithStashSkipsThePopWhenThereIsNothingToStash(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	upstreamTip := remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")

	writeFile(t, repo.Dir, "other.txt", "user's own stash\n")
	userSHA := stashPush(t, repo.Dir, "user's own work")
	// clean again: PullWithStash has nothing to stash

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.PullWithStash(t.Context(), wt, core.PullMerge)), "done")

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" || strings.Contains(list, "\n") {
		t.Errorf("stash list = %q, want the user's one entry, untouched", list)
	}
	if got, ok := refSHA(t, repo.Dir, "refs/stash"); !ok || got != userSHA {
		t.Errorf("refs/stash = %q, %v, want the user's own stash untouched at %q", got, ok, userSHA)
	}
}

// pullWithStashConflictFixture sets up a repo tracking "origin" where the
// remote and local branch each change the same line of seed.txt (a real
// merge conflict), plus an unrelated unstaged change to other.txt that
// PullWithStash stashes out of the way first.
func pullWithStashConflictFixture(t *testing.T) (repo *gittest.Repo, origin string) {
	t.Helper()
	repo, origin = pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	gittest.Commit(t, repo.Dir, "seed.txt", "local change\n", "local commit")
	writeFile(t, repo.Dir, "other.txt", "unrelated local change\n") // unstaged, tracked — stashable
	return repo, origin
}

func TestFR_SYNC_09_PullWithStashLeavesTheStashInPlaceOnAConflict(t *testing.T) {
	t.Parallel()
	repo, _ := pullWithStashConflictFixture(t)

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.PullWithStash(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil || job.Error.Code != core.Conflicts {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.Conflicts)
	}
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Error("MERGE_HEAD missing, want it left in place for the conflict flow")
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" || strings.Contains(list, "\n") {
		t.Errorf("stash list = %q, want exactly one entry (not popped)", list)
	}
	stashSHA, ok := refSHA(t, repo.Dir, "refs/stash")
	if !ok {
		t.Fatal("refs/stash missing, want the kept marker entry")
	}

	// Summary.WaitingStash names the kept entry, by its stable commit SHA.
	summary, err := c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.WaitingStash != stashSHA {
		t.Errorf("Summary.WaitingStash = %q, want %q", summary.WaitingStash, stashSHA)
	}

	// Resolving the conflict and committing, then PopStash brings the
	// unrelated local change back and clears WaitingStash.
	if err := c.StageFile(t.Context(), wt, "seed.txt"); err != nil {
		t.Fatal(err)
	}
	gittest.CommitAll(t, repo.Dir, "resolve conflict")

	if err := c.PopStash(t.Context(), wt, stashSHA); err != nil {
		t.Fatal(err)
	}
	if content := readFile(t, repo.Dir, "other.txt"); content != "unrelated local change\n" {
		t.Errorf("other.txt = %q, want the stashed change restored", content)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty after popping", list)
	}
	summary, err = c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.WaitingStash != "" {
		t.Errorf("Summary.WaitingStash = %q, want \"\" after popping", summary.WaitingStash)
	}
}

// A conflicting stash pop leaves no state file, so it is plain GitFailed.
func TestFR_SYNC_09_PopStashOnAConflictKeepsTheEntryAndLeavesConflictMarkers(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "stashed change\n")
	stashSHA := stashPush(t, repo.Dir, "nebegit: local changes before pull")
	gittest.Commit(t, repo.Dir, "f.txt", "conflicting head change\n", "head change")

	c, wt := openWorktree(t, repo.Dir)

	err := c.PopStash(t.Context(), wt, stashSHA)

	coretest.AssertCode(t, err, core.GitFailed)
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" {
		t.Error("stash list empty, want the conflicting entry kept")
	}
	status := statusOf(t, c, wt)
	if !slices.ContainsFunc(status.Conflicted, func(f core.ChangedFile) bool { return f.Path == "f.txt" }) {
		t.Errorf("Status.Conflicted = %+v, want f.txt listed", status.Conflicted)
	}
}

// A stash pushed after the marker shifts the stash@{n} indices.
func TestFR_SYNC_09_DropStashDropsOnlyTheMarkerEntryWhenAUserStashIsOnTop(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "marker change\n")
	markerSHA := stashPush(t, repo.Dir, "nebegit: local changes before pull")

	writeFile(t, repo.Dir, "f.txt", "user change\n")
	userSHA := stashPush(t, repo.Dir, "user's own work")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DropStash(t.Context(), wt, markerSHA); err != nil {
		t.Fatal(err)
	}

	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" || strings.Contains(list, "\n") {
		t.Fatalf("stash list = %q, want exactly the user's one entry left", list)
	}
	if got, ok := refSHA(t, repo.Dir, "refs/stash"); !ok || got != userSHA {
		t.Errorf("refs/stash = %q, %v, want the user's own stash untouched at %q", got, ok, userSHA)
	}
}

func TestFR_SYNC_09_PopStashRestoresOnlyTheMarkerEntryWhenAUserStashIsOnTop(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	gittest.Commit(t, repo.Dir, "g.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "marker change\n")
	markerSHA := stashPush(t, repo.Dir, "nebegit: local changes before pull")

	writeFile(t, repo.Dir, "g.txt", "user change\n")
	userSHA := stashPush(t, repo.Dir, "user's own work")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.PopStash(t.Context(), wt, markerSHA); err != nil {
		t.Fatal(err)
	}

	if content := readFile(t, repo.Dir, "f.txt"); content != "marker change\n" {
		t.Errorf("f.txt = %q, want the marker stash's change restored", content)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" || strings.Contains(list, "\n") {
		t.Fatalf("stash list = %q, want exactly the user's one entry left", list)
	}
	if got, ok := refSHA(t, repo.Dir, "refs/stash"); !ok || got != userSHA {
		t.Errorf("refs/stash = %q, %v, want the user's own stash untouched at %q", got, ok, userSHA)
	}
}

// A SHA already popped or dropped elsewhere is a stale ID: Code Internal.
func TestFR_SYNC_09_PopStashWithAnUnknownSHAReturnsAClearError(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")

	c, wt := openWorktree(t, repo.Dir)

	err := c.PopStash(t.Context(), wt, strings.Repeat("a", 40))

	coretest.AssertCode(t, err, core.Internal)
}

func TestFR_SYNC_02_QA_COMP_01_PullIntegrateArgsMapsModeAndPullFFToGitArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mode   core.PullMode
		pullFF string // "" leaves pull.ff unset
		want   []string
	}{
		{"merge, pull.ff unset", core.PullMerge, "", []string{"merge", "--no-edit", "@{upstream}"}},
		{"merge, pull.ff=false", core.PullMerge, "false", []string{"merge", "--no-edit", "--no-ff", "@{upstream}"}},
		{"merge, pull.ff=off", core.PullMerge, "off", []string{"merge", "--no-edit", "--no-ff", "@{upstream}"}},
		{"merge, pull.ff=true", core.PullMerge, "true", []string{"merge", "--no-edit", "--ff", "@{upstream}"}},
		{"merge, pull.ff=only", core.PullMerge, "only", []string{"merge", "--no-edit", "@{upstream}"}},
		{"rebase", core.PullRebase, "false", []string{"rebase", "--fork-point", "@{upstream}"}},
	}
	repo := gittest.New(t) // rows run in order on one repo; the unset row comes first
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pullFF != "" {
				gittest.Run(t, repo.Dir, "config", "pull.ff", tt.pullFF)
			}
			args, err := pullIntegrateArgs(t.Context(), git.Runner{}, repo.Dir, tt.mode)
			if err != nil || !slices.Equal(args, tt.want) {
				t.Errorf("args = %v, %v, want %v", args, err, tt.want)
			}
		})
	}

	_, err := pullIntegrateArgs(t.Context(), git.Runner{}, t.TempDir(), core.PullMode("bogus"))
	coretest.AssertCode(t, err, core.Internal)
}

func TestQA_COMP_01_PullIntegrateArgsFailsOnAConfigReadErrorOtherThanUnset(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	f, err := os.OpenFile(filepath.Join(repo.Dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[pull\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = pullIntegrateArgs(t.Context(), git.Runner{}, repo.Dir, core.PullMerge)

	coretest.AssertCode(t, err, core.GitFailed)
}

// pullToDone runs pull in mode on dir's worktree and fails the test unless
// its job ends done.
func pullToDone(t *testing.T, dir string, mode core.PullMode, pull func(*Core, context.Context, core.WorktreeID, core.PullMode) (string, error)) {
	t.Helper()
	c, wt := openWorktree(t, dir)
	id := jobID(t)(pull(c, t.Context(), wt, mode))
	job := waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.ID == id && (j.State == "done" || j.State == "failed") })
	if job.State != "done" {
		t.Fatalf("job = %+v, want done", job.Error)
	}
}

// headParents are HEAD's parent SHAs, first parent first.
func headParents(t *testing.T, dir string) []string {
	t.Helper()
	return strings.Fields(gittest.Out(t, dir, "log", "-1", "--format=%P", "HEAD"))
}

func TestQA_COMP_01_PullMergeWithPullFFFalseMakesAMergeCommitWhenBehind(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Run(t, repo.Dir, "config", "pull.ff", "false")
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).Pull)

	if parents := headParents(t, repo.Dir); len(parents) != 2 || parents[1] != upstreamTip {
		t.Errorf("HEAD parents = %v, want a merge commit with %s second", parents, upstreamTip)
	}
}

func TestQA_COMP_01_PullMergeWithPullFFTrueFastForwardsDespiteMergeFFFalse(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Run(t, repo.Dir, "config", "pull.ff", "true")
	gittest.Run(t, repo.Dir, "config", "merge.ff", "false")
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).Pull)

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
}

func TestFR_SYNC_01_FetchOneRemotePrunesGoneBranchesLeavesOthersUntouched(t *testing.T) {
	t.Parallel()
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

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "a")), "done")

	if _, ok := refSHA(t, repo.Dir, "refs/remotes/a/feature"); ok {
		t.Error("refs/remotes/a/feature still exists, want pruned")
	}
	if got, ok := refSHA(t, repo.Dir, "refs/remotes/b/master"); !ok || got != bMaster {
		t.Errorf("refs/remotes/b/master = %q, %v, want unchanged %q", got, ok, bMaster)
	}
}

func TestFR_SYNC_01_FetchAllUpdatesEveryRemote(t *testing.T) {
	t.Parallel()
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

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "")), "done")

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
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "done")

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
}

func TestFR_SYNC_02_PullMergeCreatesMergeCommitForNonConflictingDivergence(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "upstream-only.txt", "u", "upstream commit")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "done")

	parents := headParents(t, repo.Dir)
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
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	// both sides edit seed.txt differently: a real content conflict
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	gittest.Commit(t, repo.Dir, "seed.txt", "local change\n", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil || job.Error.Code != core.Conflicts {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.Conflicts)
	}
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Error("MERGE_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestFR_SYNC_02_PullMergeRefusesWhileDetached(t *testing.T) {
	t.Parallel()
	repo, _ := pullMergeFixture(t)
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	if err := c.CheckoutCommit(t.Context(), wt, sha); err != nil {
		t.Fatal(err)
	}

	job := waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil || job.Error.Code != core.GitFailed {
		t.Errorf("job.Error = %+v, want Code %s, no crash", job.Error, core.GitFailed)
	}
}

func TestFR_SYNC_02_PullMergeRefusesWithNoUpstreamConfigured(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "origin") // added, never pushed — no upstream tracking

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil || job.Error.Code != core.GitFailed {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.GitFailed)
	}
}

func TestFR_SYNC_02_PullRebaseLinearisesNonConflictingDivergence(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	upstreamTip := remoteAppendCommit(t, origin, "upstream-only.txt", "u", "upstream commit")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullRebase)), "done")

	parents := headParents(t, repo.Dir)
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
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	gittest.Commit(t, repo.Dir, "seed.txt", "local change\n", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullRebase)), "failed")

	if job.Error == nil || job.Error.Code != core.Conflicts {
		t.Errorf("job.Error = %+v, want Code %s", job.Error, core.Conflicts)
	}
	if !fileExists(filepath.Join(repo.Dir, ".git", "rebase-merge")) {
		t.Error("rebase-merge missing, want it left in place for the conflict flow")
	}
}

// remoteAmendTip amends remoteDir's default-branch tip with file set to
// content, through a throwaway clone, and force-pushes it. Returns the new
// tip's SHA.
func remoteAmendTip(t *testing.T, remoteDir, file, content string) string {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "clone")
	gittest.Run(t, filepath.Dir(clone), "clone", remoteDir, clone)
	writeFile(t, clone, file, content)
	gittest.Run(t, clone, "add", "--", file)
	gittest.Run(t, clone, "commit", "--amend", "--no-edit")
	gittest.Run(t, clone, "push", "--force", "origin", "HEAD")
	return gittest.HeadSHA(t, clone)
}

func TestQA_COMP_01_PullRebaseAfterAnUpstreamRewriteReplaysOnlyTheLocalCommit(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "upstream.txt", "first version\n", "upstream commit")
	gittest.Run(t, repo.Dir, "pull", "--ff-only")
	gittest.Commit(t, repo.Dir, "local.txt", "l", "local commit")
	rewrittenTip := remoteAmendTip(t, origin, "upstream.txt", "rewritten version\n")

	pullToDone(t, repo.Dir, core.PullRebase, (*Core).Pull)

	if parents := headParents(t, repo.Dir); len(parents) != 1 || parents[0] != rewrittenTip {
		t.Errorf("HEAD parents = %v, want the rewritten upstream tip %s", parents, rewrittenTip)
	}
	if subject := gittest.Out(t, repo.Dir, "log", "-1", "--format=%s", "HEAD"); subject != "local commit" {
		t.Errorf("HEAD subject = %q, want the local commit", subject)
	}
}

func TestFR_SYNC_03_PushPlainUpdatesTrackedUpstreamNoSetUpstream(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "local.txt", "x", "local commit")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "", "")), "done")

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != localTip {
		t.Errorf("origin's master = %q, want %q", got, localTip)
	}
	if cmd := lastCommand(t, rec, "push"); strings.Contains(cmd.Command, "--set-upstream") {
		t.Errorf("push command = %q, want no --set-upstream", cmd.Command)
	}
}

// TestFR_SYNC_03_PushSetUpstreamPushesTheNamedBranchEvenIfHEADMovedAway
// covers a caller that named a branch (PushTarget, or a dialog) before HEAD
// switched elsewhere: Push must still push and track that named branch, not
// whatever is current now.
func TestFR_SYNC_03_PushSetUpstreamPushesTheNamedBranchEvenIfHEADMovedAway(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "switch", "-c", "feature")
	gittest.Commit(t, repo.Dir, "f.txt", "f", "feature commit")
	featureTip := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "switch", "-c", "other")
	gittest.Commit(t, repo.Dir, "o.txt", "o", "other commit")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "origin", "feature")), "done")

	if got := gittest.Out(t, origin, "rev-parse", "feature"); got != featureTip {
		t.Errorf("origin's feature = %q, want %q", got, featureTip)
	}
	if _, ok := refSHA(t, origin, "refs/heads/other"); ok {
		t.Error("origin gained a heads/other ref, want only feature pushed")
	}
	if got := gittest.Out(t, repo.Dir, "rev-parse", "--abbrev-ref", "feature@{upstream}"); got != "origin/feature" {
		t.Errorf("feature's upstream = %q, want origin/feature", got)
	}
}

// TestFR_SYNC_03_PushTargetReadsAutoSetupRemoteUnsetTrueAndBadValue covers
// push.autoSetupRemote's three outcomes: exit 1 with nothing configured
// means false, a true spelling means true, and a bad value is git's own
// refusal (Code GitFailed, naming the bad value).
func TestFR_SYNC_03_PushTargetReadsAutoSetupRemoteUnsetTrueAndBadValue(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "origin")
	c, wt := openWorktree(t, repo.Dir)

	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.AutoSetUpstream {
		t.Fatalf("unset: PushTarget = %+v, %v, want AutoSetUpstream false", got, err)
	}

	gittest.Run(t, repo.Dir, "config", "push.autoSetupRemote", "yes")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || !got.AutoSetUpstream {
		t.Fatalf("yes: PushTarget = %+v, %v, want AutoSetUpstream true", got, err)
	}

	gittest.Run(t, repo.Dir, "config", "push.autoSetupRemote", "bogus")
	_, err := c.PushTarget(t.Context(), wt)
	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "bogus") {
		t.Errorf("GitStderr = %q, want it to name the bad value", ce.GitStderr)
	}
}

// TestFR_SYNC_03_PushTargetRemoteFollowsPushRemoteThenPushDefaultThenBranchRemote
// covers %(push:remotename)'s own three configured levels, in git's
// priority order, with three remotes so none of them is the sole-remote
// fallback.
func TestFR_SYNC_03_PushTargetRemoteFollowsPushRemoteThenPushDefaultThenBranchRemote(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "up")
	bareRemote(t, repo, "other")
	bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "switch", "-c", "b")
	c, wt := openWorktree(t, repo.Dir)

	gittest.Run(t, repo.Dir, "config", "branch.b.pushRemote", "up")
	gittest.Run(t, repo.Dir, "config", "remote.pushDefault", "other")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "up" {
		t.Errorf("branch.b.pushRemote set: Remote = %q, %v, want up", got.Remote, err)
	}

	gittest.Run(t, repo.Dir, "config", "--unset", "branch.b.pushRemote")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "other" {
		t.Errorf("only remote.pushDefault: Remote = %q, %v, want other", got.Remote, err)
	}

	gittest.Run(t, repo.Dir, "config", "--unset", "remote.pushDefault")
	gittest.Run(t, repo.Dir, "config", "branch.b.remote", "other")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "other" {
		t.Errorf("only branch.b.remote: Remote = %q, %v, want other", got.Remote, err)
	}

	gittest.Run(t, repo.Dir, "config", "--unset", "branch.b.remote")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "origin" {
		t.Errorf("none configured: Remote = %q, %v, want origin (the fallback)", got.Remote, err)
	}
}

// TestFR_SYNC_03_PushTargetFallsBackToTheSoleRemoteThenEmptyWithTwoUnconfigured
// covers the two fallbacks %(push:remotename) does not apply itself: the
// sole remote, and giving up (empty) once a second, unnamed remote exists.
func TestFR_SYNC_03_PushTargetFallsBackToTheSoleRemoteThenEmptyWithTwoUnconfigured(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "up")
	c, wt := openWorktree(t, repo.Dir)

	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "up" {
		t.Errorf("sole remote: Remote = %q, %v, want up", got.Remote, err)
	}

	bareRemote(t, repo, "other")
	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.Remote != "" {
		t.Errorf("two remotes, none origin, unconfigured: Remote = %q, %v, want empty", got.Remote, err)
	}
}

func TestFR_SYNC_03_PushTargetOnDetachedHeadFailsGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := gittest.HeadSHA(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)
	if err := c.CheckoutCommit(t.Context(), wt, sha); err != nil {
		t.Fatal(err)
	}

	_, err := c.PushTarget(t.Context(), wt)

	coretest.AssertCode(t, err, core.GitFailed)
}

// TestFR_SYNC_03_PushTargetReadsBranchAndHasUpstreamFreshAfterAConfigOnlyChange
// covers a `git branch -u` run outside the app, which only edits
// .git/config: PushTarget must reflect it immediately, unlike a cached refs
// store the UI would otherwise rely on.
func TestFR_SYNC_03_PushTargetReadsBranchAndHasUpstreamFreshAfterAConfigOnlyChange(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "origin", "HEAD") // no -u: no upstream set
	gittest.Run(t, repo.Dir, "fetch", "origin")
	c, wt := openWorktree(t, repo.Dir)

	if got, err := c.PushTarget(t.Context(), wt); err != nil || got.HasUpstream || got.Branch != "master" {
		t.Fatalf("before: PushTarget = %+v, %v, want HasUpstream false, Branch %q", got, err, "master")
	}

	gittest.Run(t, repo.Dir, "branch", "--set-upstream-to=origin/master", "master")

	got, err := c.PushTarget(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasUpstream || got.Branch != "master" {
		t.Errorf("after git branch -u: PushTarget = %+v, want HasUpstream true, Branch %q", got, "master")
	}
}

func TestFR_SYNC_04_PushBehindUpstreamIsRejectedNotFastForward(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	remoteAppendCommit(t, origin, "elsewhere.txt", "x", "pushed from elsewhere") // never fetched here
	gittest.Commit(t, repo.Dir, "local.txt", "y", "local commit")                // diverges, unaware

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "", "")), "failed")

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if job.Error.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
	if !strings.Contains(job.Error.Message, "fetch first") {
		t.Errorf("message = %q, want it to name the reason", job.Error.Message)
	}
}

// A rejection reason other than the fast-forward ones: PushRejected.
func TestFR_SYNC_04_PushDeclinedByAServerHookNamesTheReason(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	hookPath := filepath.Join(origin, "hooks", "pre-receive")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, repo.Dir, "local.txt", "x", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "", "")), "failed")

	if job.Error == nil || job.Error.Code != core.PushRejected {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.PushRejected)
	}
	if job.Error.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
	if !strings.Contains(job.Error.Message, "hook declined") {
		t.Errorf("message = %q, want it to name the reason", job.Error.Message)
	}
}

// TestFR_SYNC_04_PushWithARemoteGivenRejectedNotFastForwardNamesNoUpstreamAndOffersNothing
// covers a first push (a remote given, no upstream configured yet) whose
// remote already has a same-named branch with commits this branch lacks: it
// still classifies NotFastForward, but with its own message, naming neither
// pull nor force push, since neither applies without an upstream.
func TestFR_SYNC_04_PushWithARemoteGivenRejectedNotFastForwardNamesNoUpstreamAndOffersNothing(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "origin", "HEAD:refs/heads/feature") // remote has feature; no local upstream set
	remoteAppendCommitOnBranch(t, origin, "feature", "remote.txt", "r", "remote only")
	gittest.Run(t, repo.Dir, "switch", "--no-track", "-c", "feature")
	gittest.Commit(t, repo.Dir, "local.txt", "l", "local only") // diverges, unfetched

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "origin", "feature")), "failed")

	if job.Error == nil || job.Error.Code != core.NotFastForward || job.Error.Key != "pushFirstRejected" {
		t.Fatalf("job.Error = %+v, want Code %s, key pushFirstRejected", job.Error, core.NotFastForward)
	}
	if strings.Contains(job.Error.Message, "pull") || strings.Contains(job.Error.Message, "force") {
		t.Errorf("message = %q, want no pull or force push offer", job.Error.Message)
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
	t.Parallel()
	repo, origin := forcePushRewriteFixture(t)
	rewrittenTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.ForcePush(t.Context(), wt)), "done")

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != rewrittenTip {
		t.Errorf("origin's master = %q, want the rewritten local tip %q", got, rewrittenTip)
	}
}

func TestFR_SYNC_05_ForcePushRefusedWhenRemoteGainedAnUnfetchedCommit(t *testing.T) {
	t.Parallel()
	repo, origin := forcePushRewriteFixture(t)
	remoteAppendCommit(t, origin, "elsewhere.txt", "e", "elsewhere commit") // never fetched here
	remoteTip := gittest.Out(t, origin, "rev-parse", "master")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.ForcePush(t.Context(), wt)), "failed")

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if got := gittest.Out(t, origin, "rev-parse", "master"); got != remoteTip {
		t.Errorf("origin's master = %q, want unchanged %q", got, remoteTip)
	}
	if !strings.Contains(job.Error.Message, "stale info") {
		t.Errorf("message = %q, want it to name the reason", job.Error.Message)
	}
}

func TestQA_SEC_02_FR_SYNC_08_FetchJobClassifiesAFailedSignIn(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "remote", "add", "origin", server.URL+"/x.git")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "")), "failed")

	if job.Error == nil || job.Error.Code != core.AuthFailed {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.AuthFailed)
	}
	if strings.Contains(job.Error.Message, "127.0.0.1") || strings.Contains(job.Error.Message, "Username") {
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
	gittest.Run(t, repo.Dir, "branch", "B")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "master", "B")
	return repo, origin
}

func updateBranchResult(t *testing.T, job core.JobInfo) core.UpdateBranchResult {
	t.Helper()
	if job.Result == nil {
		t.Fatal("job.Result = nil, want an UpdateBranchResult")
	}
	return *job.Result
}

func TestFR_UPD_01_02_03_05_UpdateBranchFastForwardsABranchNotCheckedOutAnywhere(t *testing.T) {
	t.Parallel()
	repo, origin := updateBranchFixture(t)
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")
	newTip := remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")
	mainBefore := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "done")

	if job.Kind != "update-branch" || job.Branch != "B" {
		t.Errorf("job Kind, Branch = %q, %q, want update-branch, B", job.Kind, job.Branch)
	}
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
	if branch := currentBranch(t, repo.Dir); branch != "master" {
		t.Errorf("current branch = %q, want master (untouched)", branch)
	}

	job = waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "done")
	result = updateBranchResult(t, job)
	if result.Outcome != "upToDate" || result.From != newTip || result.To != newTip || result.Commits != 0 {
		t.Errorf("second Result = %+v, want Outcome upToDate, From == To == %q, Commits 0", result, newTip)
	}
}

func TestFR_UPD_05_UpdateBranchReportsTheBranchNotATagOfTheSameName(t *testing.T) {
	t.Parallel()
	repo, origin := updateBranchFixture(t)
	gittest.Commit(t, repo.Dir, "tagged.txt", "t", "tagged")
	gittest.Run(t, repo.Dir, "tag", "B")
	newTip := remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "done")

	if result := updateBranchResult(t, job); result.To != newTip || result.Commits != 1 {
		t.Errorf("Result = %+v, want To %q (the branch's new tip), Commits 1", result, newTip)
	}
}

func TestFR_UPD_01_UpdateBranchFastForwardsABranchTrackingALocalBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "--track", "B", "master")
	gittest.Commit(t, repo.Dir, "next.txt", "y", "next")
	masterTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	id := jobID(t)(c.UpdateBranch(t.Context(), wt, "B"))
	job := waitForJob(t, c, wt, func(j core.JobInfo) bool { return j.ID == id && (j.State == "done" || j.State == "failed") })
	if job.State != "done" {
		t.Fatalf("job = %+v, want done", job)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != masterTip {
		t.Errorf("refs/heads/B = %q, want master's tip %q", got, masterTip)
	}
}

func TestFR_UPD_03_UpdateBranchDivergedFromUpstreamFailsNotFastForward(t *testing.T) {
	t.Parallel()
	repo, origin := updateBranchFixture(t)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream-only.txt", "u", "upstream commit")
	gittest.Run(t, repo.Dir, "checkout", "B")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")
	localTip := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "failed")

	if job.Error == nil || job.Error.Code != core.NotFastForward {
		t.Fatalf("job.Error = %+v, want Code %s", job.Error, core.NotFastForward)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != localTip {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, localTip)
	}
}

func TestFR_UPD_05_FR_UX_11_UpdateBranchWithNoUpstreamFailsBeforeAnyJobNamingTheBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "B")
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c, wt := openWorktree(t, repo.Dir)

	// A precondition, not a git failure: UpdateBranch itself returns the
	// error, with no job to poll.
	_, err := c.UpdateBranch(t.Context(), wt, "B")

	ce := coretest.AsError(t, err, core.NoUpstream)
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
	if ce.Args["branch"] != "B" {
		t.Errorf("Args[branch] = %q, want %q", ce.Args["branch"], "B")
	}
	if ce.Key != "noUpstream" {
		t.Errorf("Key = %q, want noUpstream", ce.Key)
	}
}

// updateBranchElsewhereFixture is updateBranchFixture with "B" checked out
// in a second linked worktree (otherDir) rather than nowhere. The update
// refuses before opening a session on otherDir.
func updateBranchElsewhereFixture(t *testing.T) (repo *gittest.Repo, origin, otherDir string) {
	t.Helper()
	repo, origin = updateBranchFixture(t)
	otherDir = filepath.Join(t.TempDir(), "other")
	gittest.Run(t, repo.Dir, "worktree", "add", otherDir, "B")
	return repo, origin, otherDir
}

func TestFR_UPD_04_UpdateBranchCheckedOutElsewhereRefusesBeforeAnyGitCall(t *testing.T) {
	t.Parallel()
	repo, origin, otherDir := updateBranchElsewhereFixture(t)
	otherPath := gitPath(t, otherDir)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")
	beforeRemote, _ := refSHA(t, repo.Dir, "refs/remotes/origin/B")

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	_, err := c.UpdateBranch(t.Context(), wt, "B")

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Key != "branchCheckedOutElsewhere" {
		t.Errorf("Key = %q, want branchCheckedOutElsewhere", ce.Key)
	}
	if ce.Args["branch"] != "B" || ce.Args["path"] != otherPath {
		t.Errorf("Args = %+v, want branch=B path=%s", ce.Args, otherPath)
	}
	jobs, jobsErr := c.Jobs(t.Context(), wt)
	if jobsErr != nil {
		t.Fatal(jobsErr)
	}
	if len(jobs) != 0 {
		t.Errorf("Jobs = %+v, want none started", jobs)
	}
	for _, e := range rec.snapshot() {
		if strings.Contains(e.Command, "fetch") || strings.Contains(e.Command, "merge") {
			t.Errorf("recorded %q, want no fetch or merge before the refusal", e.Command)
		}
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/remotes/origin/B"); got != beforeRemote {
		t.Errorf("refs/remotes/origin/B = %q, want unchanged %q", got, beforeRemote)
	}
	if _, err := os.Stat(filepath.Join(otherDir, "upstream.txt")); !os.IsNotExist(err) {
		t.Errorf("otherDir has upstream.txt, want it untouched")
	}
}

func TestFR_UPD_01_UpdateBranchCheckedOutInThisWindowRefusesNamingItsOwnPath(t *testing.T) {
	t.Parallel()
	repo, origin := updateBranchFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "B")
	repoPath := gitPath(t, repo.Dir)
	remoteAppendCommitOnBranch(t, origin, "B", "upstream.txt", "u", "upstream commit")
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.UpdateBranch(t.Context(), wt, "B")

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Key != "branchCheckedOutElsewhere" {
		t.Errorf("Key = %q, want branchCheckedOutElsewhere", ce.Key)
	}
	if ce.Args["branch"] != "B" || ce.Args["path"] != repoPath {
		t.Errorf("Args = %+v, want branch=B path=%s", ce.Args, repoPath)
	}
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
}

// branchCheckedOutElsewhere is a repo on "master" with branch "B" checked out
// in a second linked worktree (wtDir). B tracks the local master, which is
// all the update-branch checks need of an upstream.
func branchCheckedOutElsewhere(t *testing.T, steps ...gittest.Step) (repo *gittest.Repo, wtDir string) {
	t.Helper()
	repo = gittest.New(t)
	gittest.History(t, repo.Dir, append([]gittest.Step{
		{Message: "seed", Files: map[string]string{"seed.txt": "x"}},
		{Branch: "B", From: "master", Message: "B"},
	}, steps...)...)
	gittest.Run(t, repo.Dir, "branch", "--set-upstream-to=master", "B")
	wtDir = filepath.Join(t.TempDir(), "wt")
	gittest.Run(t, repo.Dir, "worktree", "add", wtDir, "B")
	return repo, wtDir
}

// rebaseInProgressFixture is branchCheckedOutElsewhere with "B" stopped
// mid-rebase on a conflict: B and branch "upstream" both add conflict.txt
// with different content. applyBackend selects `rebase --apply` over the
// default merge backend.
func rebaseInProgressFixture(t *testing.T, applyBackend bool) (repo *gittest.Repo, wtDir string) {
	t.Helper()
	repo, wtDir = branchCheckedOutElsewhere(t,
		gittest.Step{Branch: "B", Message: "local change", Files: map[string]string{"conflict.txt": "local\n"}},
		gittest.Step{Branch: "upstream", From: "master", Message: "remote change", Files: map[string]string{"conflict.txt": "remote\n"}},
	)
	args := []string{"rebase"}
	if applyBackend {
		args = append(args, "--apply")
	}
	args = append(args, "upstream")
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: wtDir, Args: args}); err == nil {
		t.Fatal("rebase did not conflict as the fixture expects")
	}
	return repo, wtDir
}

// TestFR_UPD_04_UpdateBranchRefusesABranchBeingRebasedOrBisectedElsewhere runs
// one worktree through every in-progress state UpdateBranch refuses on; each
// refusal starts no job, runs no fetch and changes nothing.
func TestFR_UPD_04_UpdateBranchRefusesABranchBeingRebasedOrBisectedElsewhere(t *testing.T) {
	t.Parallel()
	repo, wtDir := rebaseInProgressFixture(t, false)
	wtPath := gitPath(t, wtDir)
	wtGitDir := gittest.Out(t, wtDir, "rev-parse", "--absolute-git-dir")
	dotGit := filepath.Join(wtDir, ".git")
	dotGitContent, err := os.ReadFile(dotGit)
	if err != nil {
		t.Fatal(err)
	}
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	refuses := func(t *testing.T, key string) {
		t.Helper()
		_, err := c.UpdateBranch(t.Context(), wt, "B")
		ce := coretest.AsError(t, err, core.GitFailed)
		if ce.Key != key {
			t.Errorf("Key = %q, want %s", ce.Key, key)
		}
		if ce.Args["branch"] != "B" || ce.Args["path"] != wtPath {
			t.Errorf("Args = %+v, want branch=B path=%s", ce.Args, wtPath)
		}
		jobs, jobsErr := c.Jobs(t.Context(), wt)
		if jobsErr != nil {
			t.Fatal(jobsErr)
		}
		if len(jobs) != 0 {
			t.Errorf("Jobs = %+v, want none started", jobs)
		}
		if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
			t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
		}
	}
	restoreDotGit := func() {
		if err := os.Remove(dotGit); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dotGit, dotGitContent, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("rebasing", func(t *testing.T) {
		refuses(t, "branchRebasingElsewhere")
		if _, err := os.Stat(filepath.Join(wtGitDir, "rebase-merge")); err != nil {
			t.Errorf("rebase-merge is gone, want the rebase still in progress: %v", err)
		}
	})
	t.Run("rebasing, relative gitdir", func(t *testing.T) {
		rewriteGitDirRelative(t, wtDir)
		refuses(t, "branchRebasingElsewhere")
	})
	// os.Lstat would see the symlink itself, not a directory.
	t.Run("rebasing, .git a symlink to the git dir", func(t *testing.T) {
		if err := os.Remove(dotGit); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(wtGitDir, dotGit); err != nil {
			t.Fatal(err)
		}
		refuses(t, "branchRebasingElsewhere")
		restoreDotGit()
	})
	t.Run("rebasing, apply backend", func(t *testing.T) {
		gittest.Run(t, wtDir, "rebase", "--abort")
		if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: wtDir, Args: []string{"rebase", "--apply", "upstream"}}); err == nil {
			t.Fatal("rebase did not conflict as the fixture expects")
		}
		refuses(t, "branchRebasingElsewhere")
	})
	t.Run("bisecting", func(t *testing.T) {
		gittest.Run(t, wtDir, "rebase", "--abort")
		gittest.Run(t, wtDir, "bisect", "start")
		gittest.Run(t, wtDir, "bisect", "bad", "HEAD")
		gittest.Run(t, wtDir, "bisect", "good", "master")
		refuses(t, "branchBisectingElsewhere")
	})

	for _, e := range rec.snapshot() {
		if strings.Contains(e.Command, "fetch") {
			t.Errorf("recorded %q, want no fetch before a refusal", e.Command)
		}
	}
}

// bisectInProgressFixture is branchCheckedOutElsewhere with "B" mid-bisect
// from a two-commit history.
func bisectInProgressFixture(t *testing.T) (repo *gittest.Repo, wtDir string) {
	t.Helper()
	repo, wtDir = branchCheckedOutElsewhere(t,
		gittest.Step{Branch: "B", Message: "c1", Files: map[string]string{"c1.txt": "1"}},
		gittest.Step{Branch: "B", Message: "c2", Files: map[string]string{"c2.txt": "2"}},
	)
	gittest.Run(t, wtDir, "bisect", "start")
	gittest.Run(t, wtDir, "bisect", "bad", "HEAD")
	gittest.Run(t, wtDir, "bisect", "good", "HEAD~2")
	return repo, wtDir
}

func TestFR_UPD_04_UpdateBranchRebasingInMainWorktreeRefusesNamingIt(t *testing.T) {
	t.Parallel()
	repo, origin := updateBranchFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "B")
	mainPath := gitPath(t, repo.Dir)
	otherDir := filepath.Join(t.TempDir(), "other")
	gittest.Run(t, repo.Dir, "worktree", "add", "-b", "other", otherDir, "master")
	gittest.Commit(t, repo.Dir, "conflict.txt", "local\n", "local change")
	remoteAppendCommitOnBranch(t, origin, "B", "conflict.txt", "remote\n", "remote change")
	gittest.Run(t, repo.Dir, "fetch", "origin")
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: repo.Dir, Args: []string{"rebase", "origin/B"}}); err == nil {
		t.Fatal("rebase did not conflict as the fixture expects")
	}

	c, wt := openWorktree(t, otherDir)

	_, err := c.UpdateBranch(t.Context(), wt, "B")

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Key != "branchRebasingElsewhere" {
		t.Errorf("Key = %q, want branchRebasingElsewhere", ce.Key)
	}
	if ce.Args["path"] != mainPath {
		t.Errorf("Args[path] = %q, want the main worktree %s", ce.Args["path"], mainPath)
	}
}

// rewriteGitDirRelative rewrites worktreeDir's .git file to a "gitdir:"
// target relative to worktreeDir, as worktree.useRelativePaths would.
func rewriteGitDirRelative(t *testing.T, worktreeDir string) {
	t.Helper()
	dotGit := filepath.Join(worktreeDir, ".git")
	content, err := os.ReadFile(dotGit)
	if err != nil {
		t.Fatal(err)
	}
	target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(content)), "gitdir:"))
	rel, err := filepath.Rel(worktreeDir, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotGit, []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFR_SYNC_09_PullWithStashGivesTheEntryBackWhenTheMergeNeverStarted(t *testing.T) {
	t.Parallel()
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	remoteAppendCommit(t, origin, "u", "upstream\n", "upstream adds u")
	writeFile(t, repo.Dir, "other.txt", "local uncommitted change\n")
	writeFile(t, repo.Dir, "u", "untracked\n")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.PullWithStash(t.Context(), wt, core.PullMerge)), "failed")

	if job.Error == nil {
		t.Fatal("job.Error is nil, want the untracked file to stop the pull")
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
	if content := readFile(t, repo.Dir, "other.txt"); content != "local uncommitted change\n" {
		t.Errorf("other.txt = %q, want the local change back", content)
	}
}
