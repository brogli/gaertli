package main

import (
	"bytes"
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
	if err := os.MkdirAll(cfg.GoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.FrontendRoot, 0o755); err != nil {
		t.Fatal(err)
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

func TestQA_MNT_02_RequiredIDsAreTheMustFRFromEveryFunctionalFile(t *testing.T) {
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

	want := map[string]bool{"FR-AB-01": true, "FR-GH-01": true}
	if len(required) != len(want) {
		t.Fatalf("required = %v, want %v", required, want)
	}
	for id := range want {
		if !required[id] {
			t.Errorf("required missing %s", id)
		}
	}
	if required["FR-CD-01"] {
		t.Error("SHOULD requirement FR-CD-01 must be excluded")
	}
}

func TestQA_MNT_02_FunctionalDirWithoutRequirementFilesIsAnError(t *testing.T) {
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

func TestQA_MNT_02_GoTestCoversRequiredID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "foo_test.go"),
		"package foo_test\n\nfunc TestFR_AB_01_Thing(t *testing.T) {}\n")

	assertReport(t, cfg, "")
}

func TestQA_MNT_02_GoTestNameDigitBoundaryDoesNotFalseMatch(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "foo_test.go"),
		"package foo_test\n\nfunc TestFR_AB_010_Other(t *testing.T) {}\n")

	out, code := runFixture(t, cfg)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(out, "FR-AB-01") {
		t.Errorf("output = %q, want it to mention FR-AB-01", out)
	}
}

func TestQA_MNT_02_VitestCoversRequiredID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.FrontendRoot, "src/components/__tests__/foo.spec.ts"),
		"import { it } from 'vitest'\n\nit('FR-AB-01 does a thing', () => {})\n")

	assertReport(t, cfg, "")
}

func TestQA_MNT_02_PlaywrightSpecCoversRequiredID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.FrontendRoot, "e2e-real", "foo.spec.ts"),
		"import { test } from './fixtures'\n\ntest('FR-AB-01 does a thing', async () => {})\n")

	assertReport(t, cfg, "")
}

func TestQA_MNT_02_ManualChecklistCoversRequiredID(t *testing.T) {
	checklist := "| FR-AB-01 | some manual check | all |\n"
	cfg := newConfig(t, oneMustFR, noRows, noRows, checklist)

	assertReport(t, cfg, "")
}

func TestQA_MNT_02_ManualChecklistRowCoversMultipleIDs(t *testing.T) {
	checklist := "| FR-AB-01, FR-CD-01 | some manual check | all |\n"
	cfg := newConfig(t, twoMustFR, noRows, noRows, checklist)

	assertReport(t, cfg, "")
}

func TestQA_MNT_02_UncoveredMustFRIsReportedMissing(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)

	assertReport(t, cfg, "FR-AB-01: no test name and no manual checklist entry\n")
}

func TestQA_MNT_02_UncoveredQualityAndDistributionIDsAreNotRequired(t *testing.T) {
	cfg := newConfig(t, oneMustFR, qualityFixture, oneDistID, noRows)

	assertReport(t, cfg, "FR-AB-01: no test name and no manual checklist entry\n")
}

const oneDistID = `| ID | Requirement |
|---|---|
| DIST-01 | a two-segment id |
`

func TestQA_MNT_02_TestNamedForNonexistentIDLeavesRealIDReportedMissing(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "foo_test.go"),
		"package foo_test\n\nfunc TestFR_AB_99_Thing(t *testing.T) {}\n")

	assertReport(t, cfg, "FR-AB-01: no test name and no manual checklist entry\n")
}

const oneShouldFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-CD-01 | SHOULD | a should requirement |
`

func TestQA_MNT_02_UncoveredShouldFRIsNotReported(t *testing.T) {
	cfg := newConfig(t, oneShouldFR, noRows, noRows, noRows)

	assertReport(t, cfg, "")
}

const twoMustFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
| FR-CD-01 | MUST | another must requirement |
`

func TestQA_MNT_02_EachUncoveredIDGetsExactlyOneSortedLine(t *testing.T) {
	cfg := newConfig(t, twoMustFR, noRows, noRows, noRows)

	assertReport(t, cfg, "FR-AB-01: no test name and no manual checklist entry\n"+
		"FR-CD-01: no test name and no manual checklist entry\n")
}

func TestTR_30_PendingEntrySuppressesMissingID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	assertReport(t, cfg, "")
}

func TestTR_30_PendingIgnoresBlankAndCommentLines(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, cfg.PendingPath, "# header\n\nFR-AB-01\n\n# trailer\n")

	assertReport(t, cfg, "")
}

func TestTR_30_MissingPendingFileIsAnEmptyList(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 || !strings.Contains(out, "FR-AB-01: no test name") {
		t.Errorf("code = %d, output = %q; want 1 and the missing line", code, out)
	}
}

func TestTR_30_StalePendingEntryIsReported(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "a_test.go"), "package a\n\nfunc TestFR_AB_01_Thing(t *testing.T) {}\n")
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	assertReport(t, cfg, "FR-AB-01: no longer missing; remove it from "+cfg.PendingPath+"\n")
}
