// Package mkrepo builds a deterministic reference git repository via git
// fast-import, for perf benchmarking (spec 13). Its Build/Config/Params are
// imported directly by tools/bench (same module); tools/mkrepo/cmd/mkrepo is
// its CLI entry point.
package mkrepo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/brogli/nebegit/internal/git"
)

// Config is the reference repo's shape.
type Config struct {
	Commits         int
	FeatureBranches int
	TagCount        int
	MergeEvery      int // advance-and-merge a feature branch back this often
	Worktrees       int
	Seed            int64
}

// Params is docs/requirements/quality.md's reference repo: "100k commits".
var Params = Config{Commits: 100_000, FeatureBranches: 40, TagCount: 50, MergeEvery: 500, Worktrees: 3, Seed: 42}

const generatorVersion = "v1" // bumped by hand whenever stream's own logic changes

// Fingerprint is Build's cache key: generatorVersion plus every Config
// field, one short deterministic string.
func (cfg Config) Fingerprint() string {
	return fmt.Sprintf("%s-%d-%d-%d-%d-%d-%d", generatorVersion,
		cfg.Commits, cfg.FeatureBranches, cfg.TagCount, cfg.MergeEvery, cfg.Worktrees, cfg.Seed)
}

const fingerprintFile = ".mkrepo-fingerprint"

// runner executes every git call Build makes. A package var, not a field on
// Config or a Build parameter, so tests can wrap its Log to count calls
// (e.g. proving a cache hit runs no git process at all) without widening
// Build's signature.
var runner = git.Runner{}

// Build makes dir a checkout matching cfg: a cache hit if dir already holds
// a ".mkrepo-fingerprint" file equal to cfg.Fingerprint() (no git process
// runs); otherwise wipes dir and cfg.Worktrees sibling directories and
// rebuilds it via git fast-import.
func Build(ctx context.Context, dir string, cfg Config) error {
	fp := cfg.Fingerprint()
	if current, err := os.ReadFile(filepath.Join(dir, fingerprintFile)); err == nil && string(current) == fp {
		return nil
	}

	restore, err := isolateConfig()
	if err != nil {
		return err
	}
	defer restore()

	if err := wipe(dir, cfg.Worktrees); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, git.Call{Dir: dir, Args: []string{"init", "--quiet", "--initial-branch=main"}}); err != nil {
		return err
	}

	var buf bytes.Buffer
	stream(&buf, cfg)
	if _, err := runner.Run(ctx, git.Call{Dir: dir, Args: []string{"fast-import", "--quiet"}, Stdin: buf.Bytes()}); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, git.Call{Dir: dir, Args: []string{"checkout", "--quiet", "main"}}); err != nil {
		return err
	}
	if err := addWorktrees(ctx, dir, cfg); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, fingerprintFile), []byte(fp), 0o644)
}

// wipe removes dir and its worktree sibling directories, so a rebuild never
// mixes stale state with the new history.
func wipe(dir string, worktrees int) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for i := 1; i <= worktrees; i++ {
		if err := os.RemoveAll(worktreeDir(dir, i)); err != nil {
			return err
		}
	}
	return nil
}

func worktreeDir(dir string, n int) string {
	return fmt.Sprintf("%s-wt%d", dir, n)
}

// addWorktrees adds cfg.Worktrees sibling worktrees: one on an existing
// feature branch, the rest detached at an older commit on main — varied on
// purpose, FR-LOG-10/FR-BR-05-shaped data for any later spec benchmarking
// against this repo.
func addWorktrees(ctx context.Context, dir string, cfg Config) error {
	for i := 1; i <= cfg.Worktrees; i++ {
		wt := worktreeDir(dir, i)
		args := []string{"worktree", "add", "--quiet"}
		if i == 1 && cfg.FeatureBranches > 0 {
			args = append(args, wt, featureBranchName(1))
		} else {
			args = append(args, "--detach", wt, fmt.Sprintf("main~%d", i))
		}
		if _, err := runner.Run(ctx, git.Call{Dir: dir, Args: args}); err != nil {
			return err
		}
	}
	return nil
}

// isolateConfig points HOME and git's global config at a fresh temp
// directory and disables the system config, so Build never reads the
// developer's own git configuration (gittest.isolateConfig's own approach,
// spec 02, duplicated rather than shared: gittest.New takes a *testing.T
// and cannot be called from a non-test main).
func isolateConfig() (restore func(), err error) {
	home, err := os.MkdirTemp("", "mkrepo-home")
	if err != nil {
		return nil, err
	}
	prevHome, hadHome := os.LookupEnv("HOME")
	prevGlobal, hadGlobal := os.LookupEnv("GIT_CONFIG_GLOBAL")
	prevNoSystem, hadNoSystem := os.LookupEnv("GIT_CONFIG_NOSYSTEM")

	_ = os.Setenv("HOME", home)
	_ = os.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	_ = os.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	return func() {
		restoreEnv("HOME", prevHome, hadHome)
		restoreEnv("GIT_CONFIG_GLOBAL", prevGlobal, hadGlobal)
		restoreEnv("GIT_CONFIG_NOSYSTEM", prevNoSystem, hadNoSystem)
		_ = os.RemoveAll(home)
	}, nil
}

func restoreEnv(key, value string, had bool) {
	if had {
		_ = os.Setenv(key, value)
	} else {
		_ = os.Unsetenv(key)
	}
}

const (
	authorName  = "mkrepo"
	authorEmail = "mkrepo@nebegit.invalid"
	baseTime    = 1_700_000_000 // fixed epoch, deterministic across runs

	branchAdvanceEvery = 137 // "advances every 137th main-line step" (Design)
)

func featureBranchName(n int) string { return fmt.Sprintf("feature-%d", n) }

// stream writes cfg's deterministic git fast-import stream to w: one blob +
// one commit per history entry on branch "main" by default; cfg.FeatureBranches
// branches open (parented on main's then-current tip), each advances every
// branchAdvanceEvery main-line step while active and merges back into main
// every cfg.MergeEvery steps (oldest-active-branch-first); cfg.TagCount tags
// land on fixed, seed-derived positions. A fixed-seed math/rand/v2.Rand
// (cfg.Seed) picks branch-open/tag positions.
func stream(w io.Writer, cfg Config) {
	rng := rand.New(rand.NewPCG(uint64(cfg.Seed), uint64(cfg.Seed>>32)+1))
	fi := &fastImport{w: w}

	openAt := schedulePositions(rng, cfg.Commits, cfg.FeatureBranches)
	tagAt := schedulePositions(rng, cfg.Commits, cfg.TagCount)

	var active []string     // open feature branches, oldest first
	tip := map[string]int{} // branch -> its current tip's mark
	opened, tagged := 0, 0

	for i := 0; i < cfg.Commits; i++ {
		switch {
		case opened < len(openAt) && i == openAt[opened]:
			branch := featureBranchName(opened + 1)
			tip[branch] = fi.commit(branch, fmt.Sprintf("start %s", branch), "main")
			active = append(active, branch)
			opened++

		case len(active) > 0 && (i+1)%cfg.MergeEvery == 0:
			branch := active[0]
			active = active[1:]
			tip["main"] = fi.merge("main", fmt.Sprintf("merge %s", branch), tip[branch])

		case len(active) > 0 && i%branchAdvanceEvery == 0:
			branch := active[i/branchAdvanceEvery%len(active)]
			tip[branch] = fi.commit(branch, fmt.Sprintf("advance %s", branch), "")

		default:
			tip["main"] = fi.commit("main", fmt.Sprintf("commit %d", i), "")
		}

		if tagged < len(tagAt) && i == tagAt[tagged] {
			fi.tag(fmt.Sprintf("v0.%d.0", tagged+1), tip["main"])
			tagged++
		}
	}
}

// schedulePositions returns count strictly increasing, seed-jittered
// positions in [0, total), evenly spread across the range.
func schedulePositions(rng *rand.Rand, total, count int) []int {
	if count == 0 {
		return nil
	}
	step := total / (count + 1)
	positions := make([]int, count)
	prev := -1
	for i := range positions {
		pos := step * (i + 1)
		if step > 1 {
			pos += int(rng.IntN(step)) - step/2
		}
		if pos <= prev {
			pos = prev + 1
		}
		if pos > total-1 {
			pos = total - 1
		}
		positions[i] = pos
		prev = pos
	}
	return positions
}

// fastImport writes a git fast-import stream (Own code vs library: no
// maintained Go library builds one under a permissive licence, Research
// used).
type fastImport struct {
	w    io.Writer
	mark int
}

// commit writes one blob and one commit that modifies it, on ref; from, if
// not "", is the ref the first commit on a brand-new branch forks from
// (an existing ref's own tip is otherwise implicit from the stream so far).
// Returns the commit's own mark.
func (fi *fastImport) commit(ref, message, from string) int {
	blobMark := fi.blob(message)

	fi.mark++
	commitMark := fi.mark
	fi.writeCommitHeader(ref, commitMark, message)
	if from != "" {
		_, _ = fmt.Fprintf(fi.w, "from refs/heads/%s\n", from)
	}
	_, _ = fmt.Fprintf(fi.w, "M 100644 :%d file.txt\n\n", blobMark)
	return commitMark
}

// merge writes a merge commit on ref whose first parent is ref's own
// current tip (implicit) and whose second parent is otherMark.
func (fi *fastImport) merge(ref, message string, otherMark int) int {
	blobMark := fi.blob(message)

	fi.mark++
	commitMark := fi.mark
	fi.writeCommitHeader(ref, commitMark, message)
	_, _ = fmt.Fprintf(fi.w, "merge :%d\n", otherMark)
	_, _ = fmt.Fprintf(fi.w, "M 100644 :%d file.txt\n\n", blobMark)
	return commitMark
}

// blob writes one blob command and returns its mark.
func (fi *fastImport) blob(content string) int {
	fi.mark++
	data := content + "\n"
	_, _ = fmt.Fprintf(fi.w, "blob\nmark :%d\ndata %d\n%s\n", fi.mark, len(data), data)
	return fi.mark
}

// writeCommitHeader writes the "commit"/mark/author/committer/data lines
// shared by commit and merge.
func (fi *fastImport) writeCommitHeader(ref string, mark int, message string) {
	when := baseTime + mark
	data := message + "\n"
	_, _ = fmt.Fprintf(fi.w, "commit refs/heads/%s\nmark :%d\nauthor %s <%s> %d +0000\ncommitter %s <%s> %d +0000\ndata %d\n%s",
		ref, mark, authorName, authorEmail, when, authorName, authorEmail, when, len(data), data)
}

// tag writes a lightweight tag ref at commitMark.
func (fi *fastImport) tag(name string, commitMark int) {
	_, _ = fmt.Fprintf(fi.w, "reset refs/tags/%s\nfrom :%d\n\n", name, commitMark)
}
