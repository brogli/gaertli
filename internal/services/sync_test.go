package services

import (
	"context"
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

// remoteAppendCommit adds a commit on remoteDir's default branch via a
// throwaway clone, simulating a push from elsewhere without touching
// repo's own history. Returns the new commit's SHA.
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
			got := classifyPushRejection(err, git.Result{Stdout: []byte(tt.stdout)})
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

// TestFR_SYNC_09_ClassifyDirtyIntegrateReclassifiesRebaseAndMergeRefusals
// covers dirtyIntegrateReasons' three matched stderr shapes: rebase
// blocked by unstaged changes, rebase blocked by staged changes, and a
// fast-forward merge blocked by local changes to a file the incoming
// commit also touches.
func TestFR_SYNC_09_ClassifyDirtyIntegrateReclassifiesRebaseAndMergeRefusals(t *testing.T) {
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
			coretest.AssertCode(t, classifyDirtyIntegrate(err, git.Result{Stderr: []byte(tt.stderr)}), tt.want)
		})
	}
}

// TestFR_SYNC_09_PullReclassifiesAFastForwardBlockedByLocalChangesAsDirtyWorktree:
// a fast-forward pull succeeds despite an unstaged change to a file the
// incoming commits don't touch; only a change to a file they *also* touch
// blocks it. That is the fixture below.
func TestFR_SYNC_09_PullReclassifiesAFastForwardBlockedByLocalChangesAsDirtyWorktree(t *testing.T) {
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
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	upstreamTip := remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")
	// an unstaged change to a TRACKED file untouched by the incoming
	// commit — git stash push (no -u) only ever captures tracked content,
	// so the fixture's local change must be to an already-committed file.
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

// TestFR_SYNC_09_PullWithStashSkipsThePopWhenThereIsNothingToStash covers a
// worktree already clean when PullWithStash runs: `git stash push` finds
// nothing to save and creates no entry, so refs/stash is unchanged before
// and after — PullWithStash must not pop, or it would grab whatever stash
// a user already had on the stack.
func TestFR_SYNC_09_PullWithStashSkipsThePopWhenThereIsNothingToStash(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	upstreamTip := remoteAppendCommit(t, origin, "seed.txt", "upstream change\n", "upstream commit")

	writeFile(t, repo.Dir, "other.txt", "user's own stash\n")
	userSHA := stashPush(t, repo.Dir, "user's own work")
	// The worktree is clean again: PullWithStash's own `stash push` below
	// has nothing to save.

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

	// Summary.PullStash names the kept entry, by its stable commit SHA.
	summary, err := c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PullStash != stashSHA {
		t.Errorf("Summary.PullStash = %q, want %q", summary.PullStash, stashSHA)
	}

	// Resolving the conflict and committing, then PopStash brings the
	// unrelated local change back and clears PullStash.
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
	if summary.PullStash != "" {
		t.Errorf("Summary.PullStash = %q, want \"\" after popping", summary.PullStash)
	}
}

// TestFR_SYNC_09_PopStashOnAConflictKeepsTheEntryAndLeavesConflictMarkers
// covers a marker stash whose pop conflicts with HEAD: a conflicting
// `stash pop` leaves no state file, so it surfaces as plain GitFailed.
func TestFR_SYNC_09_PopStashOnAConflictKeepsTheEntryAndLeavesConflictMarkers(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "stashed change\n")
	stashSHA := stashPush(t, repo.Dir, stashMarker)
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

func TestFR_SYNC_09_DropStashDeletesTheEntry(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "changed\n")
	stashSHA := stashPush(t, repo.Dir, stashMarker)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DropStash(t.Context(), wt, stashSHA); err != nil {
		t.Fatal(err)
	}

	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty after drop", list)
	}
}

// TestFR_SYNC_09_DropStashDropsOnlyTheMarkerEntryWhenAUserStashIsOnTop: a
// second, unrelated stash pushed after the marker shifts stash@{n}
// indices. DropStash resolves the marker's own SHA to its current index
// at call time, so it drops the marker entry and leaves the user's stash
// untouched.
func TestFR_SYNC_09_DropStashDropsOnlyTheMarkerEntryWhenAUserStashIsOnTop(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "marker change\n")
	markerSHA := stashPush(t, repo.Dir, stashMarker)

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

// TestFR_SYNC_09_PopStashRestoresOnlyTheMarkerEntryWhenAUserStashIsOnTop is
// DropStash's above sibling for PopStash: it restores the marker entry's
// content and leaves the user's own stash in place.
func TestFR_SYNC_09_PopStashRestoresOnlyTheMarkerEntryWhenAUserStashIsOnTop(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	gittest.Commit(t, repo.Dir, "g.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "marker change\n")
	markerSHA := stashPush(t, repo.Dir, stashMarker)

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

// TestFR_SYNC_09_PopStashWithAnUnknownSHAReturnsAClearError covers a SHA
// that names no current stash entry (already popped/dropped elsewhere): a
// stale caller-supplied ID, Code Internal, not a git call.
func TestFR_SYNC_09_PopStashWithAnUnknownSHAReturnsAClearError(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")

	c, wt := openWorktree(t, repo.Dir)

	err := c.PopStash(t.Context(), wt, strings.Repeat("a", 40))

	coretest.AssertCode(t, err, core.Internal)
}

func TestFR_SYNC_02_QA_COMP_01_PullIntegrateArgsMapsModeAndPullFFToGitArgs(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := gittest.New(t)
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
	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "fatal: bad config line 1 in file .git/config")

	_, err := pullIntegrateArgs(t.Context(), git.Runner{Git: fakegit}, t.TempDir(), core.PullMerge)

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
	repo, origin := pullMergeFixture(t)
	gittest.Run(t, repo.Dir, "config", "pull.ff", "false")
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).Pull)

	if parents := headParents(t, repo.Dir); len(parents) != 2 || parents[1] != upstreamTip {
		t.Errorf("HEAD parents = %v, want a merge commit with %s second", parents, upstreamTip)
	}
}

func TestQA_COMP_01_PullMergeWithPullFFTrueFastForwardsDespiteMergeFFFalse(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	gittest.Run(t, repo.Dir, "config", "pull.ff", "true")
	gittest.Run(t, repo.Dir, "config", "merge.ff", "false")
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).Pull)

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
}

func TestQA_COMP_01_PullMergeWithPullFFOnlyStillMergesADivergedBranch(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	gittest.Run(t, repo.Dir, "config", "pull.ff", "only")
	upstreamTip := remoteAppendCommit(t, origin, "upstream-only.txt", "u", "upstream commit")
	gittest.Commit(t, repo.Dir, "local-only.txt", "l", "local commit")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).Pull)

	if parents := headParents(t, repo.Dir); len(parents) != 2 || parents[1] != upstreamTip {
		t.Errorf("HEAD parents = %v, want a merge commit with %s second", parents, upstreamTip)
	}
}

func TestQA_COMP_01_PullWithStashMergeWithPullFFFalseMakesAMergeCommitAndRestoresTheChange(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	gittest.Run(t, repo.Dir, "config", "pull.ff", "false")
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")
	writeFile(t, repo.Dir, "other.txt", "local uncommitted change\n")

	pullToDone(t, repo.Dir, core.PullMerge, (*Core).PullWithStash)

	if parents := headParents(t, repo.Dir); len(parents) != 2 || parents[1] != upstreamTip {
		t.Errorf("HEAD parents = %v, want a merge commit with %s second", parents, upstreamTip)
	}
	if content := readFile(t, repo.Dir, "other.txt"); content != "local uncommitted change\n" {
		t.Errorf("other.txt = %q, want the local change restored", content)
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
	repo, origin := pullMergeFixture(t)
	upstreamTip := remoteAppendCommit(t, origin, "upstream.txt", "u", "upstream commit")

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Pull(t.Context(), wt, core.PullMerge)), "done")

	if got := gittest.HeadSHA(t, repo.Dir); got != upstreamTip {
		t.Errorf("HEAD = %q, want fast-forwarded to upstream tip %q", got, upstreamTip)
	}
}

func TestFR_SYNC_02_PullMergeCreatesMergeCommitForNonConflictingDivergence(t *testing.T) {
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
	repo, origin := pullMergeFixture(t)
	gittest.Commit(t, repo.Dir, "local.txt", "x", "local commit")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "")), "done")

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != localTip {
		t.Errorf("origin's master = %q, want %q", got, localTip)
	}
	if cmd := lastCommand(t, rec, "push"); strings.Contains(cmd.Command, "--set-upstream") {
		t.Errorf("push command = %q, want no --set-upstream", cmd.Command)
	}
}

func TestFR_SYNC_03_PushSetUpstreamCreatesRemoteBranchAndTracks(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	origin := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "switch", "-c", "feature")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "origin")), "done")

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

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "")), "failed")

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

// TestFR_SYNC_04_PushDeclinedByAServerHookNamesTheReason covers the
// PushRejected branch: a rejection --porcelain reports with a reason other
// than the fast-forward ones, here a pre-receive hook that refuses every push.
func TestFR_SYNC_04_PushDeclinedByAServerHookNamesTheReason(t *testing.T) {
	repo, origin := pullMergeFixture(t)
	hookPath := filepath.Join(origin, "hooks", "pre-receive")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, repo.Dir, "local.txt", "x", "local commit")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "")), "failed")

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

	c, wt := openWorktree(t, repo.Dir)
	waitForJobState(t, c, wt, jobID(t)(c.ForcePush(t.Context(), wt)), "done")

	if got := gittest.Out(t, origin, "rev-parse", "master"); got != rewrittenTip {
		t.Errorf("origin's master = %q, want the rewritten local tip %q", got, rewrittenTip)
	}
}

func TestFR_SYNC_05_ForcePushRefusedWhenRemoteGainedAnUnfetchedCommit(t *testing.T) {
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
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "fatal: could not read Username for 'https://example.com/x.git': terminal prompts disabled")

	c, wt := openWorktree(t, repo.Dir)
	sess, err := c.sessions.Get(wt)
	if err != nil {
		t.Fatal(err)
	}
	sess.Runner.Git = fakegit

	job := waitForJobState(t, c, wt, jobID(t)(c.Fetch(t.Context(), wt, "")), "failed")

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
	if job.Result == nil {
		t.Fatal("job.Result = nil, want an UpdateBranchResult")
	}
	return *job.Result
}

func TestFR_UPD_01_02_03_05_UpdateBranchFastForwardsABranchNotCheckedOutAnywhere(t *testing.T) {
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
}

func TestFR_UPD_05_UpdateBranchReportsTheBranchNotATagOfTheSameName(t *testing.T) {
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

func TestFR_UPD_05_UpdateBranchAlreadyAtUpstreamTipReportsUpToDate(t *testing.T) {
	repo, _ := updateBranchFixture(t)
	beforeSHA, _ := refSHA(t, repo.Dir, "refs/heads/B")

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "done")

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

	c, wt := openWorktree(t, repo.Dir)
	job := waitForJobState(t, c, wt, jobID(t)(c.UpdateBranch(t.Context(), wt, "B")), "failed")

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

	c, wt := openWorktree(t, repo.Dir)

	// Refused before any job starts (a precondition, not a git failure):
	// UpdateBranch itself returns the error, with no job to poll.
	_, err := c.UpdateBranch(t.Context(), wt, "B")
	coretest.AssertCode(t, err, core.NoUpstream)
	if got, _ := refSHA(t, repo.Dir, "refs/heads/B"); got != beforeSHA {
		t.Errorf("refs/heads/B = %q, want unchanged %q", got, beforeSHA)
	}
}

func TestFR_UX_11_UpdateBranchWithNoUpstreamNamesTheBranchInArgs(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "B")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.UpdateBranch(t.Context(), wt, "B")

	ce := coretest.AsError(t, err, core.NoUpstream)
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
