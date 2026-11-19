// Requirements: QA-MNT-01

package repocheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// maxRequirementLength caps a requirement's text, in characters, so a row
// holds one rule.
const maxRequirementLength = 400

var requirementIDPattern = regexp.MustCompile(`^[A-Z]{2,6}(?:-[A-Z]{2,6})?-[0-9]{2,3}$`)

// findLongRequirements returns the ID of every Markdown table row below dir
// that starts with a requirement ID and whose last cell is longer than
// maxRequirementLength.
func findLongRequirements(t *testing.T, dir string) []string {
	t.Helper()
	var long []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(content), "\n") {
			cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
			id := strings.TrimSpace(cells[0])
			text := strings.TrimSpace(cells[len(cells)-1])
			if requirementIDPattern.MatchString(id) && utf8.RuneCountInString(text) > maxRequirementLength {
				long = append(long, id)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return long
}

func TestRequirementRowsStayShort(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		dir := t.TempDir()
		rows := "| ID | Prio | Requirement |\n| --- | --- | --- |\n" +
			"| FR-XX-01 | MUST | " + strings.Repeat("ä", maxRequirementLength) + " |\n" +
			"| FR-XX-02 | MUST | " + strings.Repeat("ä", maxRequirementLength+1) + " |\n" +
			"| a term | " + strings.Repeat("a", maxRequirementLength+1) + " |\n"
		if err := os.WriteFile(filepath.Join(dir, "area.md"), []byte(rows), 0o644); err != nil {
			t.Fatal(err)
		}

		got := findLongRequirements(t, dir)
		if len(got) != 1 || got[0] != "FR-XX-02" {
			t.Errorf("findLongRequirements(fixture) = %v, want exactly FR-XX-02", got)
		}
	})

	t.Run("real tree", func(t *testing.T) {
		long := findLongRequirements(t, filepath.Join(repoRoot(t), "docs/requirements"))
		if len(long) > 0 {
			t.Errorf("requirements longer than %d characters; split each into one rule per row:\n%s",
				maxRequirementLength, strings.Join(long, "\n"))
		}
	})
}
