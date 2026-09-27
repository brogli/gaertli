package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/testbin"
)

// fakegit is internal/git/testdata/fakegit, for the tests that inject a
// delayed stream.
var fakegit string

// argvWriterBin is internal/editor/testdata/argvwriter, the stand-in editor
// editor_test.go uses: it writes its argv to the file its first argument
// names.
var argvWriterBin string

func TestMain(m *testing.M) {
	gittest.Main(m,
		testbin.Bin{Path: &fakegit, Pkg: "../git/testdata/fakegit"},
		testbin.Bin{Path: &argvWriterBin, Pkg: "../editor/testdata/argvwriter"},
	)
}

func TestFR_LOG_01_ParseLogRecordParsesFieldsAndRefs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		record string
		want   []core.RefLabel
	}{
		{
			name:   "HEAD and a tag",
			record: "1111111111111111111111111111111111111111\x1f1111111\x1f\x1fAlice\x1f1700000000\x1fHEAD -> refs/heads/main, tag: refs/tags/v1.0\x1fInitial commit",
			want:   []core.RefLabel{{Name: "HEAD", Kind: "head"}, {Name: "main", Kind: "branch"}, {Name: "v1.0", Kind: "tag"}},
		},
		{
			name:   "a remote-tracking branch",
			record: "2222222222222222222222222222222222222222\x1f2222222\x1f1111111111111111111111111111111111111111\x1fBob\x1f1700000100\x1frefs/remotes/origin/feature\x1fFeature work",
			want:   []core.RefLabel{{Name: "origin/feature", Kind: "remote"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit, _, err := parseLogRecord([]byte(tt.record))
			if err != nil {
				t.Fatal(err)
			}
			if len(commit.Refs) != len(tt.want) {
				t.Fatalf("Refs = %+v, want %+v", commit.Refs, tt.want)
			}
			for i, ref := range commit.Refs {
				if ref != tt.want[i] {
					t.Errorf("Refs[%d] = %+v, want %+v", i, ref, tt.want[i])
				}
			}
		})
	}

	commit, parents, err := parseLogRecord([]byte(tests[1].record))
	if err != nil {
		t.Fatal(err)
	}
	if commit.SHA != "2222222222222222222222222222222222222222" || commit.ShortSHA != "2222222" {
		t.Errorf("SHA/ShortSHA = %q/%q, want the full/short hash", commit.SHA, commit.ShortSHA)
	}
	if len(parents) != 1 || parents[0] != "1111111111111111111111111111111111111111" {
		t.Errorf("parents = %v, want one parent", parents)
	}
	if commit.Author != "Bob" || commit.AuthorAt != 1700000100 || commit.Subject != "Feature work" {
		t.Errorf("commit = %+v, want Author Bob, AuthorAt 1700000100, Subject %q", commit, "Feature work")
	}
}

func TestFR_LOG_01_LoadLogIncludesAllBranchesExcludesStash(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	commitOrphan(t, repo.Dir, "main", "a.txt", "a")
	commitOrphan(t, repo.Dir, "feature", "b.txt", "b")
	commitOrphan(t, repo.Dir, "other", "c.txt", "c")
	writeFile(t, repo.Dir, "c.txt", "dirty")
	gittest.Run(t, repo.Dir, "stash")

	c, wt := openWorktree(t, repo.Dir)
	rows := loadLog(t, c, wt)

	subjects := map[string]bool{}
	for _, r := range rows {
		subjects[r.Subject] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !subjects[want] {
			t.Errorf("missing commit %q; got %+v", want, rows)
		}
	}
	if len(rows) != 3 {
		t.Errorf("rows = %+v, want exactly the 3 branch commits, no stash entries", rows)
	}
}

func TestFR_LOG_09_LogRowsWindowAheadOfLoadReturnsFewerRowsNotError(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 500)

	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	var res core.LogRowsResult
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		var err error
		res, err = c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if res.Loaded > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if res.Done {
		t.Fatal("Done became true before a partial-load observation; the test's timing assumption is off")
	}
	if res.Error != nil {
		t.Fatalf("Error = %v, want nil", res.Error)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %d, want exactly 1", len(res.Rows))
	}

	waitForLog(t, c, id) // drain the background stream so it doesn't leak noise into later tests
}

// The merge row has a line out to each parent's lane, the base row a line
// in from each branch's lane.
func TestFR_LOG_02_LogRowsJoinAMergeAndItsForkPointToBothLanes(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "base")
	gittest.Run(t, repo.Dir, "branch", "side")
	gittest.Commit(t, repo.Dir, "a.txt", "b", "on main")
	gittest.Run(t, repo.Dir, "switch", "side")
	gittest.Commit(t, repo.Dir, "b.txt", "c", "on side")
	gittest.Run(t, repo.Dir, "switch", "master")
	gittest.Run(t, repo.Dir, "merge", "--no-ff", "-m", "merge side", "side")

	c, wt := openWorktree(t, repo.Dir)
	rows := loadLog(t, c, wt)

	if len(rows) != 4 || rows[0].Subject != "merge side" || rows[3].Subject != "base" {
		t.Fatalf("rows = %+v, want merge side first and base last", rows)
	}
	lanesOf := func(g core.GraphRow, kind core.EdgeKind) []int {
		var lanes []int
		for _, e := range g.Edges {
			if e.Kind == kind {
				lanes = append(lanes, e.Lane)
			}
		}
		return lanes
	}
	if outs := lanesOf(rows[0].Graph, "out"); !slices.Equal(outs, []int{0, 1}) {
		t.Errorf("merge row outs = %v, want lanes [0 1] (%+v)", outs, rows[0].Graph)
	}
	if ins := lanesOf(rows[3].Graph, "in"); !slices.Equal(ins, []int{0, 1}) {
		t.Errorf("base row ins = %v, want lanes [0 1] (%+v)", ins, rows[3].Graph)
	}
}

func TestFR_LOG_10_LoadLogMarksCheckedOutWorktrees(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "base")
	older := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	gittest.Run(t, repo.Dir, "switch", "--detach", older)

	c, wt := openWorktree(t, repo.Dir)
	rows := loadLog(t, c, wt)

	byMsg := map[string]core.LogCommit{}
	for _, r := range rows {
		byMsg[r.Subject] = r
	}
	linkedPath := gitPath(t, linked)
	mainPath := gitPath(t, repo.Dir)
	if got := byMsg["second"].CheckedOutIn; len(got) != 1 || got[0] != linkedPath {
		t.Errorf("second commit CheckedOutIn = %v, want [%s]", got, linkedPath)
	}
	if got := byMsg["base"].CheckedOutIn; len(got) != 1 || got[0] != mainPath {
		t.Errorf("base commit CheckedOutIn = %v, want [%s]", got, mainPath)
	}
}

func TestFR_REPO_04_ReloadLogNoOpsWhenTipsUnchanged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	c, id := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, id)
	loadLog(t, c, id)
	beforeCalls := countCommands(rec, "git log ")
	beforeGen := logGeneration(t, c, id)

	if err := c.ReloadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	if got := countCommands(rec, "git log "); got != beforeCalls {
		t.Errorf("git log calls = %d, want unchanged from %d", got, beforeCalls)
	}
	if got := logGeneration(t, c, id); got != beforeGen {
		t.Errorf("Generation = %d, want unchanged (%d)", got, beforeGen)
	}
}

func TestFR_REPO_04_ReloadLogRestartsWhenARefMoved(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	c, id := openWorktree(t, repo.Dir)
	loadLog(t, c, id)
	beforeGen := logGeneration(t, c, id)

	gittest.Run(t, repo.Dir, "branch", "feature")

	if err := c.ReloadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, id)

	if got := logGeneration(t, c, id); got == beforeGen {
		t.Errorf("Generation = %d, want incremented from %d", got, beforeGen)
	}
	if !hasRefLabel(rows, "feature") {
		t.Errorf("rows after reload = %+v, want the new branch label", rows)
	}
}

func TestFR_REPO_04_ReloadLogRestartsWhenOnlyHEADsTargetChanged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "feature")

	c, id := openWorktree(t, repo.Dir)
	loadLog(t, c, id)
	beforeGen := logGeneration(t, c, id)

	gittest.Run(t, repo.Dir, "switch", "feature") // no new commit: only HEAD's target ref changes

	if err := c.ReloadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, id)

	if got := logGeneration(t, c, id); got == beforeGen {
		t.Errorf("Generation = %d, want incremented from %d", got, beforeGen)
	}
}

// A reload that keeps the commit set (a tag added on HEAD) still replaces
// the rows: its log:progress names the new generation, so a window knows
// to drop the rows it cached.
func TestFR_REPO_04_ReloadLogProgressNamesTheNewGeneration(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	progress := make(chan core.LogProgress, 100)
	c := New(func(ev core.Event) {
		var p core.LogProgress
		if ev.Name == "log:progress" && json.Unmarshal(ev.Payload, &p) == nil {
			progress <- p
		}
	})
	wt := openOn(t, c, repo.Dir)
	if err := c.LoadLog(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}
	loaded := nextDone(t, progress)

	gittest.Run(t, repo.Dir, "tag", "v1")
	if err := c.ReloadLog(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}
	reloaded := nextDone(t, progress)

	if reloaded.Generation == loaded.Generation {
		t.Errorf("reload's Generation = %d, want it to differ from the first load's", reloaded.Generation)
	}
	if got := logGeneration(t, c, wt.ID); reloaded.Generation != got {
		t.Errorf("reload's Generation = %d, want LogRows' %d", reloaded.Generation, got)
	}
}

// A window with a filter active keeps showing a commit the reset just made
// unreachable until ReloadLog also restarts its own filtered stream, not
// only the repo-wide one.
func TestFR_REPO_04_ReloadLogRestartsTheActiveFilterWhenARefMoved(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "keep this")
	gittest.Commit(t, repo.Dir, "a.txt", "a2", "drop this")

	c, id := openWorktree(t, repo.Dir)
	loadLog(t, c, id)
	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	before := waitForLogRows(t, c, id)
	if len(before.Rows) != 2 {
		t.Fatalf("filtered rows before the reset = %+v, want both commits", before.Rows)
	}

	gittest.Run(t, repo.Dir, "reset", "--hard", "HEAD~1")

	if err := c.ReloadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	after := waitForLogRows(t, c, id)

	if len(after.Rows) != 1 || after.Rows[0].Subject != "keep this" {
		t.Errorf("filtered rows after the reload = %+v, want just %q", after.Rows, "keep this")
	}
	if after.Generation == before.Generation {
		t.Errorf("filtered Generation = %d, want incremented from %d", after.Generation, before.Generation)
	}
}

// nextDone reads progress until a done event arrives.
func nextDone(t *testing.T, progress <-chan core.LogProgress) core.LogProgress {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case p := <-progress:
			if p.Done {
				return p
			}
		case <-timeout:
			t.Fatal("no done log:progress in time")
		}
	}
}

func TestFR_LOG_01_LogIndexOfFindsRowBySHA(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "third")

	c, wt := openWorktree(t, repo.Dir)
	rows := loadLog(t, c, wt)

	byMsg := map[string]core.LogCommit{}
	for _, r := range rows {
		byMsg[r.Subject] = r
	}

	got, err := c.LogIndexOf(t.Context(), wt, byMsg["second"].SHA)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 1 || !got.Found {
		t.Errorf("LogIndexOf(second) = %+v, want {Index:1, Found:true}", got)
	}

	missing, err := c.LogIndexOf(t.Context(), wt, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if missing.Found {
		t.Errorf("LogIndexOf(missing) = %+v, want Found false", missing)
	}
}

func TestQA_PERF_02_LargeLoadWritesCommitGraphOnce(t *testing.T) {
	prev := largeRepoRows.Load()
	largeRepoRows.Store(3)
	t.Cleanup(func() { largeRepoRows.Store(prev) })

	large := gittest.New(t)
	gittest.Commit(t, large.Dir, "a.txt", "1", "one")
	gittest.Commit(t, large.Dir, "a.txt", "2", "two")
	gittest.Commit(t, large.Dir, "a.txt", "3", "three")
	gittest.Commit(t, large.Dir, "a.txt", "4", "four")

	small := gittest.New(t)
	gittest.Commit(t, small.Dir, "a.txt", "1", "one")
	gittest.Commit(t, small.Dir, "a.txt", "2", "two")

	c := New(func(core.Event) {})

	largeInfo := openOn(t, c, large.Dir)
	largeRec := recordCommands(t, c, largeInfo.ID)
	loadLog(t, c, largeInfo.ID)

	smallInfo := openOn(t, c, small.Dir)
	smallRec := recordCommands(t, c, smallInfo.ID)
	loadLog(t, c, smallInfo.ID)

	// The write runs after the done signal, so wait for its recorded call.
	deadline := time.Now().Add(10 * time.Second)
	for countCommands(largeRec, "git commit-graph write") == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	largeGraph := filepath.Join(large.Dir, ".git", "objects", "info", "commit-graph")
	if _, err := os.Stat(largeGraph); err != nil {
		t.Errorf("large repo commit-graph missing: %v", err)
	}
	if got := countCommands(largeRec, "git commit-graph write"); got != 1 {
		t.Errorf("large repo recorded %d commit-graph writes, want 1", got)
	}

	smallGraph := filepath.Join(small.Dir, ".git", "objects", "info", "commit-graph")
	if _, err := os.Stat(smallGraph); err == nil {
		t.Error("small repo should not get a commit-graph file")
	}
	if got := countCommands(smallRec, "git commit-graph write"); got != 0 {
		t.Errorf("small repo recorded %d commit-graph writes, want 0", got)
	}

	loadLog(t, c, largeInfo.ID)
	if got := countCommands(largeRec, "git commit-graph write"); got != 1 {
		t.Errorf("recorded %d commit-graph writes after a third LoadLog, want still 1", got)
	}
}

// countCommands counts rec's recorded git calls whose command starts with
// prefix.
func countCommands(rec *commandRecorder, prefix string) int {
	n := 0
	for _, e := range rec.snapshot() {
		if strings.HasPrefix(e.Command, prefix) {
			n++
		}
	}
	return n
}

func logGeneration(t *testing.T, c *Core, id core.WorktreeID) int {
	t.Helper()
	res, err := c.LogRows(t.Context(), id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return res.Generation
}

func hasRefLabel(rows []core.LogCommit, name string) bool {
	for _, r := range rows {
		for _, ref := range r.Refs {
			if ref.Name == name {
				return true
			}
		}
	}
	return false
}

// streamFakeLog points id's runner at fakegit, whose log calls stream
// rows records, delayMS apart.
func streamFakeLog(t *testing.T, c *Core, id core.WorktreeID, rows, delayMS int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(file, buildLogFixture(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", strconv.Itoa(delayMS))
	wt, err := c.sessions.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	wt.Runner.Git = fakegit
}

func buildLogFixture(n int) []byte {
	var buf bytes.Buffer
	for i := range n {
		sha := fmt.Sprintf("%040d", i+1)
		fmt.Fprintf(&buf, "%s\x1f%s\x1f\x1fAuthor\x1f%d\x1f\x1fCommit %d\x00\n", sha, sha[:7], 1700000000+i, i)
	}
	return buf.Bytes()
}

// A window opened on a repo whose log another window already loaded
// learns the row count from log:progress like the first one did.
func TestFR_LOG_01_LoadLogOnALoadedRepoReemitsItsProgress(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	commitOrphan(t, repo.Dir, "main", "a.txt", "a")
	commitOrphan(t, repo.Dir, "feature", "b.txt", "b")
	var mu sync.Mutex
	var progress []core.LogProgress
	c := New(func(ev core.Event) {
		if ev.Name != "log:progress" {
			return
		}
		var p core.LogProgress
		_ = json.Unmarshal(ev.Payload, &p)
		mu.Lock()
		progress = append(progress, p)
		mu.Unlock()
	})
	wt := openOn(t, c, repo.Dir)
	loadLog(t, c, wt.ID)
	mu.Lock()
	progress = nil
	mu.Unlock()

	if err := c.LoadLog(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if want := (core.LogProgress{Loaded: 2, Done: true}); len(progress) != 1 || progress[0] != want {
		t.Errorf("progress after the second LoadLog = %+v, want [%+v]", progress, want)
	}
}

// A failed load ends with git's error in its done progress, also for a
// window that asks after it failed, and so does a failed filtered load.
func TestFR_LOG_09_AFailedLoadCarriesGitsErrorInItsDoneProgress(t *testing.T) {
	repo := gittest.New(t)
	progress := make(chan core.LogProgress, 100)
	c := New(func(ev core.Event) {
		var p core.LogProgress
		if ev.Name == "log:progress" && json.Unmarshal(ev.Payload, &p) == nil {
			progress <- p
		}
	})
	wt := openOn(t, c, repo.Dir)
	w, err := c.sessions.Get(wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "fatal: bad object HEAD\n")
	w.Runner.Git = fakegit

	assertFailed := func(what string, p core.LogProgress) {
		t.Helper()
		if p.Error == nil || p.Error.Code != core.GitFailed || !strings.Contains(p.Error.GitStderr, "bad object") {
			t.Errorf("%s progress = %+v, want git's failure in Error", what, p)
		}
	}
	if err := c.LoadLog(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}
	assertFailed("load", nextDone(t, progress))
	if err := c.LoadLog(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}
	assertFailed("second window's load", nextDone(t, progress))
	if err := c.SetLogFilter(t.Context(), wt.ID, core.LogFilter{Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	assertFailed("filtered load", nextDone(t, progress))
}

// The log is loaded once per repo; HEAD is per worktree, so each window's
// rows carry the HEAD label on its own worktree's HEAD commit.
func TestFR_LOG_01_LogRowsPlaceHEADOnTheAskingWorktreesHead(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "a")
	gittest.Run(t, repo.Dir, "branch", "side")
	gittest.Commit(t, repo.Dir, "a.txt", "a2", "a2")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "side")

	c := New(func(core.Event) {})
	mainWt := openOn(t, c, repo.Dir)
	linkedWt := openOn(t, c, linked)
	loadLog(t, c, mainWt.ID)

	for _, tc := range []struct {
		id   core.WorktreeID
		want string
	}{{mainWt.ID, "a2"}, {linkedWt.ID, "a"}} {
		res, err := c.LogRows(t.Context(), tc.id, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		var heads []string
		for _, row := range res.Rows {
			for _, ref := range row.Refs {
				if ref.Kind == "head" {
					heads = append(heads, row.Subject)
				}
			}
		}
		if len(heads) != 1 || heads[0] != tc.want {
			t.Errorf("worktree %s: HEAD labels on %v, want only on %q", tc.id, heads, tc.want)
		}
	}
}

// loadLog starts id's log load and returns the first 1000 rows once it
// finished without an error.
func loadLog(t *testing.T, c *Core, id core.WorktreeID) []core.LogCommit {
	t.Helper()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	return waitForLog(t, c, id)
}

// waitForLog returns the first 1000 rows of a load that finished without
// an error.
func waitForLog(t *testing.T, c *Core, id core.WorktreeID) []core.LogCommit {
	t.Helper()
	res := waitForLogRows(t, c, id)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	return res.Rows
}

// waitForLogRows polls LogRows until the window's load, filtered or not,
// finishes.
func waitForLogRows(t *testing.T, c *Core, id core.WorktreeID) core.LogRowsResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(t.Context(), id, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if res.Done {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("log did not finish loading in time")
	return core.LogRowsResult{}
}

func commitOrphan(t *testing.T, dir, branch, file, msg string) {
	t.Helper()
	gittest.Run(t, dir, "checkout", "--orphan", branch)
	_, _ = (git.Runner{}).Run(t.Context(), git.Call{Dir: dir, Args: []string{"rm", "-rf", "--cached", "."}})
	writeFile(t, dir, file, "x")
	gittest.Run(t, dir, "add", "--", file)
	gittest.CommitAll(t, dir, msg)
}

func TestFR_UX_02_AppDrivenReadsAreBackgroundCommands(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	loadLog(t, c, wt)
	if _, err := c.Summary(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Refs(t.Context(), wt); err != nil {
		t.Fatal(err)
	}

	for _, e := range rec.snapshot() {
		if !e.Background {
			t.Errorf("%q logged as a user command, want background", e.Command)
		}
	}
}
