// Requirements: QA-USE-03

package repocheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// glossaryHit is one retired term found in one file.
type glossaryHit struct {
	file string
	term string
}

// glossaryRetiredTerms reads the "Not" column of docs/requirements/
// glossary.md and returns every entry with a space in it: a single word is
// too common (e.g. "header", "tabs") to check on its own.
func glossaryRetiredTerms(t *testing.T, root string) []string {
	t.Helper()
	var terms []string
	for line := range strings.SplitSeq(readFile(t, root, "docs/requirements/glossary.md"), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 4 {
			continue
		}
		not := strings.TrimSpace(cols[3])
		if not == "" || not == "Not" || strings.Trim(not, "-") == "" {
			continue // header row, the separator row, or an empty cell
		}
		for term := range strings.SplitSeq(not, ",") {
			if term = strings.TrimSpace(term); strings.Contains(term, " ") {
				terms = append(terms, term)
			}
		}
	}
	return terms
}

// findGlossaryHits scans root's docs/ (except glossary.md itself),
// README.md and frontend/src/**/*.{vue,ts} for any of terms, case
// insensitive. A code identifier is camel-case and never matches a
// space-containing term.
func findGlossaryHits(t *testing.T, root string, terms []string) []glossaryHit {
	t.Helper()
	var hits []glossaryHit

	checkFile := func(path string) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(content))
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				hits = append(hits, glossaryHit{file: rel, term: term})
			}
		}
	}

	walkFiles := func(dir string, include func(path string) bool) {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !include(path) {
				return nil //nolint:nilerr // a missing optional dir (e.g. no frontend/src) just yields no hits
			}
			checkFile(path)
			return nil
		})
	}

	walkFiles(filepath.Join(root, "docs"), func(path string) bool {
		return filepath.Base(path) != "glossary.md"
	})
	if readme := filepath.Join(root, "README.md"); fileExists(readme) {
		checkFile(readme)
	}
	walkFiles(filepath.Join(root, "frontend", "src"), func(path string) bool {
		return strings.HasSuffix(path, ".vue") || strings.HasSuffix(path, ".ts")
	})

	return hits
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestGlossaryTermsUsedConsistently(t *testing.T) {
	root := repoRoot(t)

	t.Run("fixture", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		doc := filepath.Join(dir, "docs", "some-doc.md")
		if err := os.WriteFile(doc, []byte("Open the workspace picker to switch repos.\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		hits := findGlossaryHits(t, dir, []string{"workspace picker"})

		if len(hits) != 1 {
			t.Fatalf("findGlossaryHits(fixture) = %v, want exactly one hit", hits)
		}
		if hits[0].term != "workspace picker" || !strings.HasSuffix(hits[0].file, "some-doc.md") {
			t.Errorf("hit = %+v, want term %q in some-doc.md", hits[0], "workspace picker")
		}
	})

	t.Run("real tree", func(t *testing.T) {
		terms := glossaryRetiredTerms(t, root)
		if len(terms) == 0 {
			t.Fatal("parsed no retired terms from glossary.md's Not column")
		}

		hits := findGlossaryHits(t, root, terms)

		if len(hits) > 0 {
			var lines []string
			for _, h := range hits {
				lines = append(lines, h.file+": "+h.term)
			}
			t.Errorf("retired glossary terms found:\n%s", strings.Join(lines, "\n"))
		}
	})
}
