package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// writeHook installs dir/.git/hooks/name as an executable shell script.
func writeHook(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, ".git", "hooks", name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFR_CHG_05_CommitCommitsStagedChangesWithAMultilineMessageByteForByte(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	writeFile(t, repo.Dir, "a.txt", "content\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")

	c, id := openWorktree(t, repo.Dir)
	msg := "Subject\n\nBody 1\nBody 2"
	if err := c.Commit(t.Context(), id, msg, false); err != nil {
		t.Fatal(err)
	}

	if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%B", "HEAD"); got != msg {
		t.Errorf("HEAD message = %q, want %q", got, msg)
	}
	if got := gittest.Out(t, repo.Dir, "cat-file", "-p", "HEAD:a.txt"); got != "content" {
		t.Errorf("HEAD:a.txt = %q, want %q", got, "content")
	}
}

func TestFR_CHG_05_CommitFailsWhenNothingIsStaged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "content\n", "add a.txt")
	head := gittest.HeadSHA(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	err := c.Commit(t.Context(), id, "msg", false)

	coretest.AssertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != head {
		t.Errorf("HEAD = %q, want unchanged %q", got, head)
	}
}

func TestFR_CHG_05_CommitFailsOnAWhitespaceOnlyMessage(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "content\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "new\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	head := gittest.HeadSHA(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	err := c.Commit(t.Context(), id, "   \n  ", false)

	coretest.AssertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != head {
		t.Errorf("HEAD = %q, want unchanged %q", got, head)
	}
}

func TestFR_CHG_06_CommitAmendRewritesHeadInPlaceWithTheFurtherStagedChange(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "first message")
	writeFile(t, repo.Dir, "b.txt", "further\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")

	c, id := openWorktree(t, repo.Dir)
	if err := c.Commit(t.Context(), id, "new message", true); err != nil {
		t.Fatal(err)
	}

	if got := gittest.Out(t, repo.Dir, "rev-list", "--count", "HEAD"); got != "1" {
		t.Errorf("commit count = %q, want %q (amended in place, not a second commit)", got, "1")
	}
	if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%B", "HEAD"); got != "new message" {
		t.Errorf("HEAD message = %q, want %q", got, "new message")
	}
	if got := gittest.Out(t, repo.Dir, "cat-file", "-p", "HEAD:b.txt"); got != "further" {
		t.Errorf("HEAD:b.txt = %q, want %q", got, "further")
	}
}

func TestFR_CHG_06_CommitAmendSucceedsWithNothingStagedRewordingOnly(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "first message")
	treeBefore := gittest.Out(t, repo.Dir, "rev-parse", "HEAD^{tree}")

	c, id := openWorktree(t, repo.Dir)
	if err := c.Commit(t.Context(), id, "reworded", true); err != nil {
		t.Fatal(err)
	}

	if got := gittest.Out(t, repo.Dir, "log", "-1", "--format=%B", "HEAD"); got != "reworded" {
		t.Errorf("HEAD message = %q, want %q", got, "reworded")
	}
	if got := gittest.Out(t, repo.Dir, "rev-parse", "HEAD^{tree}"); got != treeBefore {
		t.Errorf("HEAD tree = %q, want unchanged %q", got, treeBefore)
	}
}

func TestFR_CHG_06_CommitAmendOnAnUnbornHeadFailsCleanly(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)

	c, id := openWorktree(t, repo.Dir)
	err := c.Commit(t.Context(), id, "msg", true)

	coretest.AssertCode(t, err, core.GitFailed)
}

func TestFR_CHG_07_AFailingPreCommitHooksOwnOutputOnEitherStreamReachesGitStderr(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	writeHook(t, repo.Dir, "pre-commit", "echo 'hook stdout line'\necho 'hook stderr line' >&2\nexit 1\n")
	head := gittest.HeadSHA(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	err := c.Commit(t.Context(), id, "msg", false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "hook stdout line") || !strings.Contains(ce.GitStderr, "hook stderr line") {
		t.Errorf("GitStderr = %q, want both of the hook's lines", ce.GitStderr)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != head {
		t.Errorf("HEAD = %q, want unchanged %q", got, head)
	}
}

func TestFR_CHG_07_AFailingCommitMsgHooksStdoutReachesGitStderr(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	writeHook(t, repo.Dir, "commit-msg", "echo 'commit-msg stdout line'\nexit 3\n")
	head := gittest.HeadSHA(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	err := c.Commit(t.Context(), id, "msg", false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "commit-msg stdout line") {
		t.Errorf("GitStderr = %q, want the hook's line", ce.GitStderr)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != head {
		t.Errorf("HEAD = %q, want unchanged %q", got, head)
	}
}

func TestFR_CHG_07_ASucceedingPreCommitHookRunsAsUsual(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	marker := filepath.Join(repo.Dir, "hook-ran")
	writeHook(t, repo.Dir, "pre-commit", "touch hook-ran\nexit 0\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.Commit(t.Context(), id, "msg", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("pre-commit hook did not run: %v", err)
	}
}

func TestFR_CHG_08_SetMessageDraftIsReadBackFromSummary(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)

	if err := c.SetMessageDraft(t.Context(), id, "draft text"); err != nil {
		t.Fatal(err)
	}

	summary, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MessageDraft != "draft text" {
		t.Errorf("MessageDraft = %q, want %q", summary.MessageDraft, "draft text")
	}
}

func TestFR_CHG_08_ASuccessfulCommitClearsTheDraft(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")

	c, id := openWorktree(t, repo.Dir)
	if err := c.SetMessageDraft(t.Context(), id, "draft"); err != nil {
		t.Fatal(err)
	}
	if err := c.Commit(t.Context(), id, "final message", false); err != nil {
		t.Fatal(err)
	}

	summary, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MessageDraft != "" {
		t.Errorf("MessageDraft = %q, want cleared", summary.MessageDraft)
	}
}

func TestFR_CHG_08_AFailedCommitLeavesTheDraftUntouched(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")

	c, id := openWorktree(t, repo.Dir)
	if err := c.SetMessageDraft(t.Context(), id, "draft"); err != nil {
		t.Fatal(err)
	}
	if err := c.Commit(t.Context(), id, "msg", false); err == nil {
		t.Fatal("Commit() error = nil, want GitFailed (nothing staged)")
	}

	summary, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MessageDraft != "draft" {
		t.Errorf("MessageDraft = %q, want unchanged %q", summary.MessageDraft, "draft")
	}
}
