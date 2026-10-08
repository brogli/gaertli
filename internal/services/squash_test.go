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
	step := func(name, author string) gittest.Step {
		return gittest.Step{
			Message: name + " subject\n\n" + name + " body",
			Author:  author,
			Files:   map[string]string{strings.ToLower(name) + ".txt": name + "\n"},
		}
	}
	shas := gittest.History(t, dir, step("base", "Al"), step("B", "Bea"), step("C", "Al"), step("D", "Al"), step("E", "Al"))
	return line{base: shas[0], B: shas[1], C: shas[2], D: shas[3], E: shas[4]}
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
	t.Parallel()
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
	t.Parallel()
	timedOut := core.Errorf(core.Cancelled, "context deadline exceeded")

	coretest.AssertCode(t, asStop(timedOut, false), core.Stopped)
	coretest.AssertCode(t, asStop(core.Errorf(core.GitFailed, "could not apply"), true), core.Conflicts)
}

func TestFR_LOG_14_SquashFoldsSelectedCommitsNextToTheOldestWithTheGivenMessageAndTheOldestsAuthor(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)

	result, err := c.Squash(t.Context(), wt, []string{l.D, l.B}, "B and D\n\n#123 stays\n")
	if err != nil {
		t.Fatal(err)
	}

	assertHistory(t, repo.Dir, "E subject: e.txt", "C subject: c.txt", "B and D: b.txt d.txt", "base subject: base.txt")
	if got := rawMessage(t, repo.Dir, "HEAD~2"); got != "B and D\n\n#123 stays" {
		t.Errorf("folded message = %q, want the given one verbatim", got)
	}
	if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%an", "HEAD~2"); got != "Bea" {
		t.Errorf("folded author = %q, want Bea (the oldest commit's)", got)
	}
	if got := rawMessage(t, repo.Dir, "HEAD~1"); got != "C subject\n\nC body" {
		t.Errorf("C's message = %q, want unchanged", got)
	}
	if result.PreviousHead != l.E {
		t.Errorf("PreviousHead = %s, want %s", result.PreviousHead, l.E)
	}
}

func TestFR_LOG_14_FixupKeepsTheOldestMessageWhateverOrderTheShasCome(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	original := gittest.New(t)
	l := lineFixture(t, original.Dir)
	for _, cleanup := range []string{"strip", "whitespace", "scissors", "verbatim"} {
		for _, commentChar := range []string{"#", ";"} {
			t.Run(cleanup+" "+commentChar, func(t *testing.T) {
				repo := gittest.Copy(t, original.Dir)
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
	t.Parallel()
	repo := gittest.New(t)
	shas := gittest.History(t, repo.Dir,
		gittest.Step{Message: "base", Files: map[string]string{"base.txt": "base\n"}},
		gittest.Step{Message: uncleanMessage, Files: map[string]string{"b.txt": "b\n"}},
		gittest.Step{Message: "C", Files: map[string]string{"c.txt": "c\n"}},
	)
	gittest.Run(t, repo.Dir, "config", "commit.cleanup", "strip")
	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Fixup(t.Context(), wt, []string{shas[2], shas[1]}); err != nil {
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
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"f.txt": "base\n"}},
		gittest.Step{Message: uncleanMessage, Files: map[string]string{"b.txt": "b\n"}},
		gittest.Step{Message: "C", Files: map[string]string{"f.txt": "c\n"}},
		gittest.Step{Message: "D", Files: map[string]string{"f.txt": "d\n"}},
	)
	return shas[1], shas[3]
}

func TestFR_LOG_14_AConflictStopsAndContinueEndsWithTheExactMessage(t *testing.T) {
	t.Parallel()
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

func TestFR_LOG_14_SquashCheckBlocksAndReportsThePushedFlagAndMessage(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)
	check := func(t *testing.T, shas ...string) core.SquashCheck {
		t.Helper()
		res, err := c.SquashCheck(t.Context(), wt, shas)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	t.Run("uncommitted changes to tracked files block, untracked ones do not", func(t *testing.T) {
		writeFile(t, repo.Dir, "untracked.txt", "fine")
		if res := check(t, l.D, l.E); res.Blocker != nil {
			t.Fatalf("with an untracked file only: blocker %v; want none", res.Blocker)
		}

		writeFile(t, repo.Dir, "b.txt", "edited")
		assertBlocker(t, check(t, l.D, l.E).Blocker, core.DirtyWorktree, "squashUncommittedChanges")
		_, err := c.Fixup(t.Context(), wt, []string{l.D, l.E})
		coretest.AssertCode(t, err, core.DirtyWorktree)
		if got := gittest.HeadSHA(t, repo.Dir); got != l.E {
			t.Errorf("HEAD = %s, want unchanged %s", got, l.E)
		}
		gittest.Run(t, repo.Dir, "checkout", "--", "b.txt")
	})

	t.Run("a commit off the current branch blocks", func(t *testing.T) {
		gittest.Run(t, repo.Dir, "switch", "-c", "side", l.C)
		gittest.Commit(t, repo.Dir, "side.txt", "side", "side")
		side := gittest.HeadSHA(t, repo.Dir)
		gittest.Run(t, repo.Dir, "switch", "master")
		const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
		orphan := gittest.Out(t, repo.Dir, "commit-tree", "-m", "unrelated root", emptyTree)

		for name, shas := range map[string][]string{
			"on another branch":   {l.B, side},
			"no common ancestor":  {l.D, orphan},
			"only another branch": {side, orphan},
		} {
			t.Run(name, func(t *testing.T) {
				assertBlocker(t, check(t, shas...).Blocker, core.GitFailed, "squashNotOnBranch")
			})
		}
	})

	t.Run("a detached head or a running operation blocks", func(t *testing.T) {
		gittest.Run(t, repo.Dir, "switch", "--detach", l.E)
		assertBlocker(t, check(t, l.D, l.E).Blocker, core.GitFailed, "squashDetached")
		gittest.Run(t, repo.Dir, "switch", "master")

		cherryPickHead := filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")
		writeFile(t, repo.Dir, filepath.Join(".git", "CHERRY_PICK_HEAD"), l.B+"\n")
		args := assertBlocker(t, check(t, l.D, l.E).Blocker, core.GitFailed, "squashOperationInProgress")
		if args["operation"] != "cherry-pick" {
			t.Errorf("operation = %q, want cherry-pick", args["operation"])
		}
		if err := os.Remove(cherryPickHead); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("fewer than two commits or a blank message is a bad request", func(t *testing.T) {
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
	})

	t.Run("the proposed message is the selected messages, oldest first", func(t *testing.T) {
		res := check(t, l.D, l.B)
		if res.Blocker != nil {
			t.Fatalf("Blocker = %v, want none", res.Blocker)
		}
		if want := "B subject\n\nB body\n\nD subject\n\nD body"; res.Message != want {
			t.Errorf("Message = %q, want %q", res.Message, want)
		}
	})

	t.Run("pushed says whether the oldest selected commit is pushed", func(t *testing.T) {
		if res := check(t, l.C, l.D); res.Pushed {
			t.Fatal("without an upstream: Pushed = true, want false")
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
			if got := check(t, tc.shas...).Pushed; got != tc.want {
				t.Errorf("Pushed for %v = %v, want %v", tc.shas, got, tc.want)
			}
		}
	})

	t.Run("a merge commit in the way blocks and is named", func(t *testing.T) {
		gittest.Run(t, repo.Dir, "switch", "-c", "merged", l.base)
		gittest.Commit(t, repo.Dir, "m.txt", "m", "merged side")
		gittest.Run(t, repo.Dir, "switch", "master")
		gittest.Run(t, repo.Dir, "merge", "--no-ff", "-m", "Merge branch 'merged'", "merged")
		merge := gittest.HeadSHA(t, repo.Dir)
		gittest.Commit(t, repo.Dir, "x.txt", "x", "after the merge")

		args := assertBlocker(t, check(t, l.E, gittest.HeadSHA(t, repo.Dir)).Blocker, core.GitFailed, "squashMergeInTheWay")
		if !strings.HasPrefix(merge, args["sha"]) || args["sha"] == "" {
			t.Errorf("sha = %q, want the merge %s abbreviated", args["sha"], merge)
		}
	})
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

func TestFR_LOG_14_AStopThatIsNoConflictSaysGitsReasonAndAbortRestores(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestFR_LOG_14_ACommitMadeAfterTheCheckStopsTheRebaseInsteadOfBeingDropped(t *testing.T) {
	t.Parallel()
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
