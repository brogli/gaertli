package services

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// mutateLines inserts, deletes and replaces a few lines.
func mutateLines(r *rand.Rand, lines []string) []string {
	out := append([]string(nil), lines...)
	letter := func() string { return string(rune('A' + r.Intn(4))) }
	for n := r.Intn(3) + 1; n > 0; n-- {
		i := r.Intn(len(out) + 1)
		switch r.Intn(3) {
		case 0:
			out = append(out[:i], append([]string{letter()}, out[i:]...)...)
		case 1:
			if i < len(out) {
				out = append(out[:i], out[i+1:]...)
			}
		case 2:
			if i < len(out) {
				out[i] = letter()
			}
		}
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// assertBaseIsMarkerSection checks that each region's base lines are the base section git wrote
// in its marker block.
func assertBaseIsMarkerSection(t *testing.T, got core.ConflictFile) {
	t.Helper()
	blocks := parseMarkerBlocks(got.Result, got.MarkerSize)
	for i, region := range got.Regions {
		if have, want := selected(got.Base.Content, region.Base), strings.Join(blocks[i].base, ""); have != want {
			t.Errorf("base %q, current %q, incoming %q: region %d base = %q, git's base section %q",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, have, want)
		}
	}
}

// assertBaseInBounds checks that each region's base lines lie within the base.
func assertBaseInBounds(t *testing.T, got core.ConflictFile) {
	t.Helper()
	lines := len(splitLines(got.Base.Content))
	for i, region := range got.Regions {
		if start, end := edges(region.Base); start < 0 || end > lines {
			t.Errorf("base %q, current %q, incoming %q: region %d base %+v lies outside the base's %d lines",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, region.Base, lines)
		}
	}
}

// resolvedWith is the result with each region's lines replaced by those of the side.
func resolvedWith(t *testing.T, got core.ConflictFile, side func(core.ConflictRegion) core.LineRange, content string) string {
	t.Helper()
	result, lines := splitLines(got.Result), splitLines(content)
	var out []string
	next := 0
	for _, region := range got.Regions {
		out = append(out, result[next:region.Result.Start-1]...)
		taken := side(region)
		if taken.Lines > 0 {
			out = append(out, lines[taken.Start-1:taken.Start-1+taken.Lines]...)
		}
		next = region.Result.Start - 1 + region.Result.Lines
	}
	return strings.Join(append(out, result[next:]...), "")
}

func TestFR_CONF_05_TakingASideInEveryRegionMatchesMergingWithThatSidePreferred(t *testing.T) {
	t.Parallel()
	// Each style is set up before the merge (config) and may rewrite the file after it.
	styles := map[string]struct {
		config  []string
		rewrite []string
		diff3   bool
	}{
		"merge":           {config: []string{"merge.conflictStyle", "merge"}},
		"zdiff3":          {config: []string{"merge.conflictStyle", "zdiff3"}},
		"diff3":           {config: []string{"merge.conflictStyle", "diff3"}, diff3: true},
		"histogram diff3": {config: []string{"merge.conflictStyle", "diff3", "diff.algorithm", "histogram"}, diff3: true},
		"checkout diff3":  {rewrite: []string{"checkout", "--conflict=diff3", "--", "a.txt"}, diff3: true},
	}
	for style, setup := range styles {
		t.Run(style, func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewSource(7))
			checked := 0
			for n := 0; checked < 25 && n < 400; n++ {
				var base []string
				for i := r.Intn(6) + 2; i > 0; i-- {
					base = append(base, string(rune('a'+r.Intn(5))))
				}
				// Half the merges share a change on both sides, which git may leave outside the markers.
				shared := base
				if r.Intn(2) == 0 {
					shared = mutateLines(r, base)
				}
				current, incoming := mutateLines(r, shared), mutateLines(r, shared)
				if joinLines(current) == joinLines(base) || joinLines(incoming) == joinLines(base) || joinLines(current) == joinLines(incoming) {
					continue
				}
				dir := divergedRepo(t, "a.txt", version(joinLines(base)), version(joinLines(current)), version(joinLines(incoming)))
				for i := 0; i < len(setup.config); i += 2 {
					gittest.Run(t, dir, "config", setup.config[i], setup.config[i+1])
				}
				tryGit(t, dir, "merge", "side")
				if len(tryGit(t, dir, "ls-files", "-u", "--", "a.txt")) == 0 {
					continue
				}
				if setup.rewrite != nil {
					gittest.Run(t, dir, setup.rewrite...)
				}
				c, wt := openWorktree(t, dir)
				got, err := c.Conflict(t.Context(), wt, "a.txt")
				if err != nil || got.Kind != core.ConflictText || len(got.Regions) == 0 {
					continue
				}
				checked++
				assertBaseInBounds(t, got)
				if setup.diff3 {
					assertBaseIsMarkerSection(t, got)
				}
				for _, taken := range []struct {
					name, strategy string
					side           func(core.ConflictRegion) core.LineRange
					content        string
				}{
					{"current", "ours", func(r core.ConflictRegion) core.LineRange { return r.Current }, got.Current.Content},
					{"incoming", "theirs", func(r core.ConflictRegion) core.LineRange { return r.Incoming }, got.Incoming.Content},
				} {
					gittest.Run(t, dir, "merge", "--abort")
					tryGit(t, dir, "merge", "--no-commit", "-X", taken.strategy, "side")
					want, err := os.ReadFile(filepath.Join(dir, "a.txt"))
					if err != nil {
						t.Fatal(err)
					}
					if have := resolvedWith(t, got, taken.side, taken.content); have != string(want) {
						t.Errorf("%s: base %q current %q incoming %q: taking %s gives %q, merge -X %s gives %q",
							style, joinLines(base), joinLines(current), joinLines(incoming), taken.name, have, taken.strategy, want)
					}
				}
			}
			if checked < 10 {
				t.Fatalf("only %d conflicting merges generated", checked)
			}
		})
	}
}
