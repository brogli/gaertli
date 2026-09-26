package services

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// line is a linear history on master: base, then B, C, D, E, each adding
// its own file (b.txt …) with message "<X> subject\n\n<X> body". B is
// authored by "Bea".
type line struct{ base, B, C, D, E string }

func lineFixture(t *testing.T, dir string) line {
	t.Helper()
	commit := func(name, author string) string {
		file, msg := strings.ToLower(name)+".txt", name+" subject\n\n"+name+" body"
		commitAs(t, dir, file, name+"\n", msg, author, author+"@nebegit.invalid")
		return gittest.HeadSHA(t, dir)
	}
	return line{
		base: commit("base", "Al"),
		B:    commit("B", "Bea"),
		C:    commit("C", "Al"),
		D:    commit("D", "Al"),
		E:    commit("E", "Al"),
	}
}

// history is dir's commits from HEAD down, one "subject: files" entry each.
func history(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for sha := range strings.Lines(gittest.Out(t, dir, "rev-list", "HEAD")) {
		sha = strings.TrimSpace(sha)
		files := strings.Fields(gittest.Out(t, dir, "show", "--root", "--name-only", "--format=", sha))
		out = append(out, gittest.Out(t, dir, "log", "-1", "--format=%s", sha)+": "+strings.Join(files, " "))
	}
	return out
}

func assertHistory(t *testing.T, dir string, want ...string) {
	t.Helper()
	if got := history(t, dir); !slices.Equal(got, want) {
		t.Errorf("history =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// rawMessage is rev's message as stored, without its final newline.
func rawMessage(t *testing.T, dir, rev string) string {
	t.Helper()
	_, msg, _ := strings.Cut(gittest.Out(t, dir, "cat-file", "commit", rev), "\n\n")
	return msg
}

func TestFR_LOG_14_SquashTodoPicksTheOldestFixesUpTheOthersAmendsThenReplaysTheRest(t *testing.T) {
	chain := []string{"b", "c", "d", "e"}
	selected := map[string]bool{"b": true, "d": true}

	want := "pick b\nfixup d\n" +
		"exec git commit --amend --no-verify --allow-empty --cleanup=verbatim --quiet -C m\n" +
		"pick c\npick e\n"
	if got := squashTodo(chain, selected, "m"); got != want {
		t.Errorf("todo = %q, want %q", got, want)
	}
}

func TestFR_LOG_14_AnyFailureThatLeavesTheRebaseIsAStop(t *testing.T) {
	timedOut := core.Errorf(core.Cancelled, "context deadline exceeded")

	coretest.AssertCode(t, asStop(timedOut, false), core.Stopped)
	coretest.AssertCode(t, asStop(core.Errorf(core.GitFailed, "could not apply"), true), core.Conflicts)
}

func TestFR_LOG_14_SquashFoldsAdjacentCommitsWithTheGivenMessageAndTheOldestsAuthor(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	result, err := c.Squash(t.Context(), wt, []string{l.C, l.B}, "Folded\n\n#123 stays\n")
	if err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "E subject: e.txt", "D subject: d.txt", "Folded: b.txt c.txt", "base subject: base.txt")
	if got := rawMessage(t, repo.Dir, "HEAD~2"); got != "Folded\n\n#123 stays" {
		t.Errorf("folded message = %q, want the given one verbatim", got)
	}
	if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%an", "HEAD~2"); got != "Bea" {
		t.Errorf("folded author = %q, want Bea (the oldest commit's)", got)
	}
	if got := rawMessage(t, repo.Dir, "HEAD"); got != "E subject\n\nE body" {
		t.Errorf("E's message = %q, want unchanged", got)
	}
	if result.PreviousHead != l.E {
		t.Errorf("PreviousHead = %s, want %s", result.PreviousHead, l.E)
	}
}

func TestFR_LOG_14_SquashMovesALaterCommitUpNextToTheOldest(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Squash(t.Context(), wt, []string{l.B, l.D}, "B and D"); err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "E subject: e.txt", "C subject: c.txt", "B and D: b.txt d.txt", "base subject: base.txt")
	if got := rawMessage(t, repo.Dir, "HEAD~1"); got != "C subject\n\nC body" {
		t.Errorf("C's message = %q, want unchanged", got)
	}
}

func TestFR_LOG_14_FixupKeepsTheOldestMessageWhateverOrderTheShasCome(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Fixup(t.Context(), wt, []string{l.E, l.C, l.D}); err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "C subject: c.txt d.txt e.txt", "B subject: b.txt", "base subject: base.txt")
	if got := rawMessage(t, repo.Dir, "HEAD"); got != "C subject\n\nC body" {
		t.Errorf("folded message = %q, want C's, body included", got)
	}
}

func TestFR_LOG_14_FixupIntoTheRootCommit(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Fixup(t.Context(), wt, []string{l.base, l.C}); err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "E subject: e.txt", "D subject: d.txt", "B subject: b.txt", "base subject: base.txt c.txt")
}

// A message git's cleanup would change: comment lines under both comment
// characters, a run of blank lines, trailing whitespace, a scissors line,
// and git's own autosquash marker as the subject.
const uncleanMessage = "amend! Subject\n\n# hash line\n; semicolon line\n\n\n\ntrailing  \t\n" +
	"# ------------------------ >8 ------------------------\nbelow the scissors"

func TestFR_LOG_14_TheMessageIsKeptExactlyWhateverTheCleanupConfig(t *testing.T) {
	for _, cleanup := range []string{"strip", "whitespace", "scissors", "verbatim"} {
		for _, commentChar := range []string{"#", ";"} {
			t.Run(cleanup+" "+commentChar, func(t *testing.T) {
				repo := gittest.New(t)
				l := lineFixture(t, repo.Dir)
				gittest.Run(t, repo.Dir, "config", "commit.cleanup", cleanup)
				gittest.Run(t, repo.Dir, "config", "core.commentChar", commentChar)
				c, wt := openWorktree(t, repo.Dir)

				if _, err := c.Squash(t.Context(), wt, []string{l.D, l.B}, uncleanMessage); err != nil {
					t.Fatal(err)
				}

				if got := rawMessage(t, repo.Dir, "HEAD~2"); got != uncleanMessage {
					t.Errorf("folded message = %q, want %q", got, uncleanMessage)
				}
				if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%an", "HEAD~2"); got != "Bea" {
					t.Errorf("folded author = %q, want Bea", got)
				}
			})
		}
	}
}

func TestFR_LOG_14_FixupKeepsTheOldestMessageExactly(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "base.txt", "base\n", "base")
	writeFile(t, repo.Dir, "b.txt", "b\n")
	gittest.Run(t, repo.Dir, "add", "b.txt")
	gittest.Run(t, repo.Dir, "commit", "--cleanup=verbatim", "-m", uncleanMessage)
	b := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "c.txt", "c\n", "C")
	gittest.Run(t, repo.Dir, "config", "commit.cleanup", "strip")
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Fixup(t.Context(), wt, []string{gittest.HeadSHA(t, repo.Dir), b}); err != nil {
		t.Fatal(err)
	}

	if got := rawMessage(t, repo.Dir, "HEAD"); got != uncleanMessage {
		t.Errorf("folded message = %q, want B's %q", got, uncleanMessage)
	}
}

// conflictLine is base, B (b.txt, message uncleanMessage), C and D, which
// both set f.txt: D no longer applies once moved up next to B.
func conflictLine(t *testing.T, dir string) (b, d string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "base\n", "base")
	writeFile(t, dir, "b.txt", "b\n")
	gittest.Run(t, dir, "add", "b.txt")
	gittest.Run(t, dir, "commit", "--cleanup=verbatim", "-m", uncleanMessage)
	b = gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "f.txt", "c\n", "C")
	gittest.Commit(t, dir, "f.txt", "d\n", "D")
	return b, gittest.HeadSHA(t, dir)
}

func TestFR_LOG_14_AConflictStopsAndContinueEndsWithTheExactMessage(t *testing.T) {
	for _, tc := range []struct {
		name string
		fold func(c *Core, wt core.WorktreeID, shas []string) error
		want string
	}{
		{"squash", func(c *Core, wt core.WorktreeID, shas []string) error {
			_, err := c.Squash(t.Context(), wt, shas, "Folded\n\n# kept\ntrailing  ")
			return err
		}, "Folded\n\n# kept\ntrailing"},
		{"fixup", func(c *Core, wt core.WorktreeID, shas []string) error {
			_, err := c.Fixup(t.Context(), wt, shas)
			return err
		}, uncleanMessage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gittest.New(t)
			b, d := conflictLine(t, repo.Dir)
			gittest.Run(t, repo.Dir, "config", "commit.cleanup", "strip")
			c, wt := openWorktree(t, repo.Dir)

			err := tc.fold(c, wt, []string{b, d})

			coretest.AssertCode(t, err, core.Conflicts)
			if operation(filepath.Join(repo.Dir, ".git")) != "rebase" {
				t.Fatal("no rebase in progress after the conflict")
			}
			writeFile(t, repo.Dir, "f.txt", "d\n")
			gittest.Run(t, repo.Dir, "add", "f.txt")
			err = c.ContinueOperation(t.Context(), wt) // C now conflicts with the folded D
			coretest.AssertCode(t, err, core.Conflicts)
			writeFile(t, repo.Dir, "f.txt", "c\n")
			gittest.Run(t, repo.Dir, "add", "f.txt")
			if err := c.ContinueOperation(t.Context(), wt); err != nil {
				t.Fatal(err)
			}
			if got := rawMessage(t, repo.Dir, "HEAD~1"); got != tc.want {
				t.Errorf("folded message = %q, want %q", got, tc.want)
			}
			if got := rawMessage(t, repo.Dir, "HEAD"); got != "C" {
				t.Errorf("C's message = %q, want unchanged", got)
			}
		})
	}
}

func TestFR_LOG_14_UncommittedChangesToTrackedFilesBlock(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	writeFile(t, repo.Dir, "untracked.txt", "fine")
	c, wt := openWorktree(t, repo.Dir)

	check, err := c.SquashCheck(t.Context(), wt, []string{l.D, l.E})
	if err != nil || check.Blocker != nil {
		t.Fatalf("with an untracked file only: blocker %v, err %v; want none", check.Blocker, err)
	}

	writeFile(t, repo.Dir, "b.txt", "edited")
	check, err = c.SquashCheck(t.Context(), wt, []string{l.D, l.E})
	if err != nil {
		t.Fatal(err)
	}
	assertBlocker(t, check.Blocker, core.DirtyWorktree, "squashUncommittedChanges")
	_, err = c.Fixup(t.Context(), wt, []string{l.D, l.E})
	coretest.AssertCode(t, err, core.DirtyWorktree)
	if got := gittest.HeadSHA(t, repo.Dir); got != l.E {
		t.Errorf("HEAD = %s, want unchanged %s", got, l.E)
	}
}

func TestFR_LOG_14_AMergeCommitInTheWayBlocksAndIsNamed(t *testing.T) {
	repo := gittest.New(t)
	x1, merge, x2 := mergeCommitFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	check, err := c.SquashCheck(t.Context(), wt, []string{x1, x2})
	if err != nil {
		t.Fatal(err)
	}

	args := assertBlocker(t, check.Blocker, core.GitFailed, "squashMergeInTheWay")
	if !strings.HasPrefix(merge, args["sha"]) || args["sha"] == "" {
		t.Errorf("sha = %q, want the merge %s abbreviated", args["sha"], merge)
	}
}

func TestFR_LOG_14_ACommitOffTheCurrentBranchBlocks(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "switch", "-c", "side", l.C)
	gittest.Commit(t, repo.Dir, "side.txt", "side", "side")
	side := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "switch", "master")
	const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	orphan := gittest.Out(t, repo.Dir, "commit-tree", "-m", "unrelated root", emptyTree)
	c, wt := openWorktree(t, repo.Dir)

	for name, shas := range map[string][]string{
		"on another branch":   {l.B, side},
		"no common ancestor":  {l.D, orphan},
		"only another branch": {side, orphan},
	} {
		check, err := c.SquashCheck(t.Context(), wt, shas)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertBlocker(t, check.Blocker, core.GitFailed, "squashNotOnBranch")
	}
}

func TestFR_LOG_14_ADetachedHeadOrARunningOperationBlocks(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	gittest.Run(t, repo.Dir, "switch", "--detach", l.E)
	check, err := c.SquashCheck(t.Context(), wt, []string{l.D, l.E})
	if err != nil {
		t.Fatal(err)
	}
	assertBlocker(t, check.Blocker, core.GitFailed, "squashDetached")

	gittest.Run(t, repo.Dir, "switch", "master")
	writeFile(t, repo.Dir, filepath.Join(".git", "CHERRY_PICK_HEAD"), l.B+"\n")
	check, err = c.SquashCheck(t.Context(), wt, []string{l.D, l.E})
	if err != nil {
		t.Fatal(err)
	}
	args := assertBlocker(t, check.Blocker, core.GitFailed, "squashOperationInProgress")
	if args["operation"] != "cherry-pick" {
		t.Errorf("operation = %q, want cherry-pick", args["operation"])
	}
}

func TestFR_LOG_14_CheckSaysWhetherTheOldestSelectedCommitIsPushed(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	check, err := c.SquashCheck(t.Context(), wt, []string{l.C, l.D})
	if err != nil || check.Pushed {
		t.Fatalf("without an upstream: Pushed %v, err %v; want false", check.Pushed, err)
	}

	bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "origin", l.C+":refs/heads/master")
	gittest.Run(t, repo.Dir, "branch", "--set-upstream-to=origin/master")
	for _, tc := range []struct {
		shas []string
		want bool
	}{
		{[]string{l.D, l.E}, false},
		{[]string{l.E, l.C}, true},
		{[]string{l.B, l.E}, true},
	} {
		check, err := c.SquashCheck(t.Context(), wt, tc.shas)
		if err != nil {
			t.Fatal(err)
		}
		if check.Pushed != tc.want {
			t.Errorf("Pushed = %v, want %v", check.Pushed, tc.want)
		}
	}
}

func TestFR_LOG_14_CheckProposesTheSelectedMessagesOldestFirst(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	check, err := c.SquashCheck(t.Context(), wt, []string{l.D, l.B})
	if err != nil {
		t.Fatal(err)
	}

	if check.Blocker != nil {
		t.Fatalf("Blocker = %v, want none", check.Blocker)
	}
	if want := "B subject\n\nB body\n\nD subject\n\nD body"; check.Message != want {
		t.Errorf("Message = %q, want %q", check.Message, want)
	}
}

func TestFR_LOG_14_SquashIgnoresTheUsersEditorsAutostashCleanupAndUpdateRefs(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "stacked", l.C)
	for _, kv := range [][2]string{
		{"sequence.editor", "false"},
		{"core.editor", "false"},
		{"rebase.autoStash", "true"},
		{"rebase.updateRefs", "true"},
		{"rebase.missingCommitsCheck", "error"},
		{"commit.cleanup", "strip"},
	} {
		gittest.Run(t, repo.Dir, "config", kv[0], kv[1])
	}
	unsetEditorEnv(t)
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Squash(t.Context(), wt, []string{l.B, l.D}, "#1 folded"); err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "E subject: e.txt", "C subject: c.txt", "#1 folded: b.txt d.txt", "base subject: base.txt")
	if got := branchTip(t, repo.Dir, "stacked"); got != l.C {
		t.Errorf("stacked = %s, want unchanged %s", got, l.C)
	}
}

func TestFR_LOG_14_FewerThanTwoCommitsOrABlankMessageIsABadRequest(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Fixup(t.Context(), wt, []string{l.E})
	coretest.AssertCode(t, err, core.Internal)
	_, err = c.Fixup(t.Context(), wt, []string{l.E, l.E})
	coretest.AssertCode(t, err, core.Internal)
	_, err = c.SquashCheck(t.Context(), wt, []string{l.D, l.D})
	coretest.AssertCode(t, err, core.Internal)
	_, err = c.Squash(t.Context(), wt, []string{l.D, l.E}, " \n")
	coretest.AssertCode(t, err, core.Internal)
	if got := gittest.HeadSHA(t, repo.Dir); got != l.E {
		t.Errorf("HEAD = %s, want unchanged %s", got, l.E)
	}
}

func TestFR_LOG_14_AStopThatIsNoConflictSaysGitsReasonAndAbortRestores(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture func(t *testing.T, dir string) (shas []string)
		reason  string
	}{
		{"an untracked file in the way of a later commit", func(t *testing.T, dir string) []string {
			t.Helper()
			gittest.Commit(t, dir, "base.txt", "base\n", "base")
			gittest.Commit(t, dir, "b.txt", "b\n", "B")
			b := gittest.HeadSHA(t, dir)
			gittest.Commit(t, dir, "z.txt", "z\n", "C adds z")
			gittest.Run(t, dir, "rm", "-q", "z.txt")
			gittest.CommitAll(t, dir, "D deletes z")
			d := gittest.HeadSHA(t, dir)
			writeFile(t, dir, "z.txt", "untracked, not to be lost\n")
			return []string{b, d}
		}, "untracked working tree files would be overwritten"},
		{"a commit folded with its own revert", func(t *testing.T, dir string) []string {
			t.Helper()
			gittest.Commit(t, dir, "base.txt", "base\n", "base")
			gittest.Commit(t, dir, "g.txt", "g\n", "B adds g")
			b := gittest.HeadSHA(t, dir)
			gittest.Run(t, dir, "rm", "-q", "g.txt")
			gittest.CommitAll(t, dir, "D removes g")
			return []string{b, gittest.HeadSHA(t, dir)}
		}, "Could not apply"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gittest.New(t)
			shas := tc.fixture(t, repo.Dir)
			head := gittest.HeadSHA(t, repo.Dir)
			c, wt := openWorktree(t, repo.Dir)

			_, err := c.Fixup(t.Context(), wt, shas)

			ce := coretest.AsError(t, err, core.Stopped)
			if !strings.Contains(ce.GitStderr+ce.Message, tc.reason) {
				t.Errorf("error = %q (details %q), want git's reason %q", ce.Message, ce.GitStderr, tc.reason)
			}
			if operation(filepath.Join(repo.Dir, ".git")) != "rebase" {
				t.Fatal("no rebase in progress after the stop")
			}
			if err := c.AbortOperation(t.Context(), wt); err != nil {
				t.Fatal(err)
			}
			if got := gittest.HeadSHA(t, repo.Dir); got != head {
				t.Errorf("HEAD after abort = %s, want %s", got, head)
			}
		})
	}
}

// refuseAmendHook is a prepare-commit-msg hook that fails for a commit that
// reuses another's message (-C), as the todo's amend does.
const refuseAmendHook = "[ \"$2\" = commit ] && { echo refusing >&2; exit 1; }\nexit 0\n"

func TestFR_LOG_14_AFailedAmendStopsAndContinueRunsItAgain(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	writeHook(t, repo.Dir, "prepare-commit-msg", refuseAmendHook)
	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Squash(t.Context(), wt, []string{l.B, l.D}, "Folded")

	coretest.AssertCode(t, err, core.Stopped)
	if err := c.ContinueOperation(t.Context(), wt); err == nil {
		t.Fatal("Continue with the amend still failing = nil, want the failure again")
	}
	if err := os.Remove(filepath.Join(repo.Dir, ".git", "hooks", "prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	if err := c.ContinueOperation(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	assertHistory(t, repo.Dir, "E subject: e.txt", "C subject: c.txt", "Folded: b.txt d.txt", "base subject: base.txt")
}

func TestFR_LOG_14_AFailedAmendStopsAndAbortRestores(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	writeHook(t, repo.Dir, "prepare-commit-msg", refuseAmendHook)
	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Squash(t.Context(), wt, []string{l.B, l.D}, "Folded")

	coretest.AssertCode(t, err, core.Stopped)
	if err := c.AbortOperation(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != l.E {
		t.Errorf("HEAD after abort = %s, want %s", got, l.E)
	}
}

func TestFR_LOG_14_ACommitMadeAfterTheCheckStopsTheRebaseInsteadOfBeingDropped(t *testing.T) {
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)
	session, err := c.sessions.Get(wt)
	if err != nil {
		t.Fatal(err)
	}
	sel, err := inspectSelection(t.Context(), session, []string{l.B, l.C})
	if err != nil || sel.blocker != nil {
		t.Fatal(err, sel.blocker)
	}
	gittest.Commit(t, repo.Dir, "late.txt", "late\n", "a commit made by another program")
	late := gittest.HeadSHA(t, repo.Dir)

	err = rebaseWithTodo(t.Context(), session, sel, l.B)

	coretest.AssertCode(t, err, core.Stopped)
	if err := c.AbortOperation(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != late {
		t.Errorf("HEAD after abort = %s, want the late commit %s", got, late)
	}
}

// assertBlocker checks blocker's code and key and returns its args.
func assertBlocker(t *testing.T, blocker *core.Error, code core.Code, key string) map[string]string {
	t.Helper()
	if blocker == nil || blocker.Code != code || blocker.Key != key {
		t.Fatalf("Blocker = %+v, want %s %s", blocker, code, key)
	}
	return blocker.Args
}
