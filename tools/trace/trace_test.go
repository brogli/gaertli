// Requirements: QA-MNT-02

package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newConfig lays out a fixture repo under t.TempDir() with the given
// functional/area.md, quality.md, distribution.md and testing.md content, and
// returns a Config pointing at it. GoRoot and FrontendRoot start out empty;
// PendingPath starts out missing.
func newConfig(t *testing.T, functional, quality, distribution, testing string) Config {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		RequirementsDir: dir,
		TestingPath:     filepath.Join(dir, "testing.md"),
		GoRoot:          filepath.Join(dir, "go"),
		FrontendRoot:    filepath.Join(dir, "frontend"),
		PendingPath:     filepath.Join(dir, "pending.txt"),
	}
	writeFile(t, filepath.Join(cfg.RequirementsDir, "functional", "area.md"), functional)
	writeFile(t, filepath.Join(cfg.RequirementsDir, "quality.md"), quality)
	writeFile(t, filepath.Join(cfg.RequirementsDir, "distribution.md"), distribution)
	writeFile(t, cfg.TestingPath, testing)
	for _, root := range []string{cfg.GoRoot, cfg.FrontendRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}

func runFixture(t *testing.T, cfg Config) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code, err := run(cfg, &out)
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), code
}

// assertReport runs trace on cfg and wants exactly the output want: exit
// code 0 when want is empty, else 1.
func assertReport(t *testing.T, cfg Config, want string) {
	t.Helper()
	wantCode := 0
	if want != "" {
		wantCode = 1
	}
	if out, code := runFixture(t, cfg); code != wantCode || out != want {
		t.Errorf("code = %d, output = %q; want %d and %q", code, out, wantCode, want)
	}
}

const functionalFixture = `# Functional requirements (FR)

| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
| FR-CD-01 | SHOULD | a should requirement |
`

const otherFunctionalFixture = `# Other area

| ID | Prio | Requirement |
|---|---|---|
| FR-GH-01 | MUST | a must requirement in another file |
`

const qualityFixture = `# Quality attributes (QA)

| ID | Requirement |
|---|---|
| QA-EF-01 | a quality requirement |
`

func TestRequiredIDsAreTheMustFRFromEveryFunctionalFile(t *testing.T) {
	dir := t.TempDir()
	functionalDir := filepath.Join(dir, "functional")
	writeFile(t, filepath.Join(functionalDir, "ab.md"), functionalFixture)
	writeFile(t, filepath.Join(functionalDir, "gh.md"), otherFunctionalFixture)
	writeFile(t, filepath.Join(functionalDir, "notes.txt"), "| FR-XY-01 | MUST | not a requirements file |\n")
	writeFile(t, filepath.Join(dir, "quality.md"), qualityFixture)
	writeFile(t, filepath.Join(dir, "distribution.md"), oneDistID)

	required, err := requiredIDs(dir)
	if err != nil {
		t.Fatal(err)
	}

	if want := map[string]bool{"FR-AB-01": true, "FR-GH-01": true}; !maps.Equal(required, want) {
		t.Errorf("required = %v, want %v: MUST rows of every .md file, no SHOULD row", required, want)
	}
}

func TestFunctionalDirWithoutRequirementFilesIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "quality.md"), qualityFixture)

	if _, err := requiredIDs(dir); err == nil {
		t.Error("requiredIDs on a requirements dir without functional/*.md: err = nil, want an error")
	}
}

const oneMustFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
`

const noRows = `| ID | Requirement |
|---|---|
`

func TestEachSourceKindCoversRequiredID(t *testing.T) {
	goRoot := func(c Config) string { return c.GoRoot }
	frontendRoot := func(c Config) string { return c.FrontendRoot }
	const header = "// Requirements: FR-AB-01\n"
	cases := []struct {
		name, file, content string
		root                func(Config) string
	}{
		{"go test", "foo_test.go", header + "\npackage foo_test\n", goRoot},
		{"vitest", "src/components/__tests__/foo.spec.ts", header + "\nimport { it } from 'vitest'\n", frontendRoot},
		{"playwright", "e2e-real/foo.spec.ts", header + "\nimport { test } from './fixtures'\n", frontendRoot},
		{"several lines union", "foo_test.go", "// Requirements: FR-CD-01\n// Requirements: FR-XX-01, FR-AB-01\n\npackage foo_test\n", goRoot},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
			writeFile(t, filepath.Join(c.root(cfg), c.file), c.content)

			assertReport(t, cfg, "")
		})
	}
}

func TestIDsOutsideTheLeadingCommentBlockProveNothing(t *testing.T) {
	cases := []struct{ name, file, content string }{
		{"go after package", "foo_test.go", "package foo_test\n\n// Requirements: FR-AB-01\n"},
		{"ts after import", "foo.spec.ts", "import { it } from 'vitest'\n// Requirements: FR-AB-01\n"},
		{"go test name", "foo_test.go", "package foo_test\n\nfunc TestFR_AB_01_Thing(t *testing.T) {}\n"},
		{"vitest title", "foo.spec.ts", "it('FR-AB-01 does a thing', () => {})\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
			writeFile(t, filepath.Join(cfg.GoRoot, c.file), c.content)
			writeFile(t, filepath.Join(cfg.FrontendRoot, c.file), c.content)

			assertReport(t, cfg, "FR-AB-01: no requirements header and no manual checklist entry\n")
		})
	}
}

func TestManualChecklistCoversRequiredID(t *testing.T) {
	checklist := "| FR-AB-01 | some manual check | all |\n"
	cfg := newConfig(t, oneMustFR, noRows, noRows, checklist)

	assertReport(t, cfg, "")
}

func TestManualChecklistRowCoversMultipleIDs(t *testing.T) {
	checklist := "| FR-AB-01, FR-CD-01 | some manual check | all |\n"
	cfg := newConfig(t, twoMustFR, noRows, noRows, checklist)

	assertReport(t, cfg, "")
}

func TestUncoveredMustFRIsReportedMissing(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)

	assertReport(t, cfg, "FR-AB-01: no requirements header and no manual checklist entry\n")
}

func TestUncoveredQualityAndDistributionIDsAreNotRequired(t *testing.T) {
	cfg := newConfig(t, oneMustFR, qualityFixture, oneDistID, noRows)

	assertReport(t, cfg, "FR-AB-01: no requirements header and no manual checklist entry\n")
}

const oneDistID = `| ID | Requirement |
|---|---|
| DIST-01 | a two-segment id |
`

func TestHeaderForNonexistentIDLeavesRealIDReportedMissing(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "foo_test.go"),
		"// Requirements: FR-AB-99\n\npackage foo_test\n")

	assertReport(t, cfg, "FR-AB-01: no requirements header and no manual checklist entry\n")
}

const oneShouldFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-CD-01 | SHOULD | a should requirement |
`

func TestUncoveredShouldFRIsNotReported(t *testing.T) {
	cfg := newConfig(t, oneShouldFR, noRows, noRows, noRows)

	assertReport(t, cfg, "")
}

const twoMustFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
| FR-CD-01 | MUST | another must requirement |
`

func TestEachUncoveredIDGetsExactlyOneSortedLine(t *testing.T) {
	cfg := newConfig(t, twoMustFR, noRows, noRows, noRows)

	assertReport(t, cfg, "FR-AB-01: no requirements header and no manual checklist entry\n"+
		"FR-CD-01: no requirements header and no manual checklist entry\n")
}

func TestPendingEntrySuppressesMissingID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	assertReport(t, cfg, "")
}

func TestPendingIgnoresBlankAndCommentLines(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, cfg.PendingPath, "# header\n\nFR-AB-01\n\n# trailer\n")

	assertReport(t, cfg, "")
}

func TestMissingPendingFileIsAnEmptyList(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 || !strings.Contains(out, "FR-AB-01: no requirements header") {
		t.Errorf("code = %d, output = %q; want 1 and the missing line", code, out)
	}
}

func TestStalePendingEntryIsReported(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "a_test.go"), "// Requirements: FR-AB-01\n\npackage a\n")
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	assertReport(t, cfg, "FR-AB-01: no longer missing; remove it from "+cfg.PendingPath+"\n")
}
