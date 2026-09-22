// Package main implements tools/trace, the requirement-traceability check
// (TR-30). Tests build synthetic fixtures in t.TempDir(); none reads the
// real repo.
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
// functional.md/quality.md/testing.md content, and returns a Config
// pointing at it. GoRoot and FrontendRoot start out empty; PendingPath
// starts out missing.
func newConfig(t *testing.T, functional, quality, testing string) Config {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		FunctionalPath: filepath.Join(dir, "functional.md"),
		QualityPath:    filepath.Join(dir, "quality.md"),
		TestingPath:    filepath.Join(dir, "testing.md"),
		GoRoot:         filepath.Join(dir, "go"),
		FrontendRoot:   filepath.Join(dir, "frontend"),
		PendingPath:    filepath.Join(dir, "pending.txt"),
	}
	writeFile(t, cfg.FunctionalPath, functional)
	writeFile(t, cfg.QualityPath, quality)
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

const functionalFixture = `# Functional requirements (FR)

| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
| FR-CD-01 | SHOULD | a should requirement |
`

const qualityFixture = `# Quality attributes (QA)

| ID | Requirement |
|---|---|
| QA-EF-01 | a quality requirement |
`

func TestQA_MNT_02_RequiredIDsAreMustFRAndAllQA(t *testing.T) {
	dir := t.TempDir()
	functionalPath := filepath.Join(dir, "functional.md")
	qualityPath := filepath.Join(dir, "quality.md")
	writeFile(t, functionalPath, functionalFixture)
	writeFile(t, qualityPath, qualityFixture)

	required, err := requiredIDs(functionalPath, qualityPath)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"FR-AB-01": true, "QA-EF-01": true}
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

const oneMustFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
`

const noRows = `| ID | Requirement |
|---|---|
`

func TestQA_MNT_02_GoTestCoversRequiredID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "foo_test.go"),
		"package foo_test\n\nfunc TestFR_AB_01_Thing(t *testing.T) {}\n")

	out, code := runFixture(t, cfg)
	if code != 0 {
		t.Errorf("code = %d, want 0; output:\n%s", code, out)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestQA_MNT_02_GoTestNameDigitBoundaryDoesNotFalseMatch(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)
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
	cfg := newConfig(t, oneMustFR, noRows, noRows)
	writeFile(t, filepath.Join(cfg.FrontendRoot, "foo.test.ts"),
		"import { it } from 'vitest'\n\nit('FR-AB-01 does a thing', () => {})\n")

	out, code := runFixture(t, cfg)
	if code != 0 {
		t.Errorf("code = %d, want 0; output:\n%s", code, out)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestQA_MNT_02_ManualChecklistCoversRequiredID(t *testing.T) {
	checklist := "| FR-AB-01 | some manual check | all |\n"
	cfg := newConfig(t, oneMustFR, noRows, checklist)

	out, code := runFixture(t, cfg)
	if code != 0 {
		t.Errorf("code = %d, want 0; output:\n%s", code, out)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestQA_MNT_02_ManualChecklistRowCoversMultipleIDs(t *testing.T) {
	checklist := "| FR-AB-01, QA-EF-01 | some manual check | all |\n"
	cfg := newConfig(t, oneMustFR, qualityFixture, checklist)

	out, code := runFixture(t, cfg)
	if code != 0 {
		t.Errorf("code = %d, want 0; output:\n%s", code, out)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestQA_MNT_02_UncoveredMustFRIsReportedMissing(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	want := "FR-AB-01: no test name and no manual checklist entry\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestQA_MNT_02_UncoveredQAIsReportedMissing(t *testing.T) {
	cfg := newConfig(t, noRows, qualityFixture, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	want := "QA-EF-01: no test name and no manual checklist entry\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

const oneShouldFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-CD-01 | SHOULD | a should requirement |
`

func TestQA_MNT_02_UncoveredShouldFRIsNotReported(t *testing.T) {
	cfg := newConfig(t, oneShouldFR, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 0 {
		t.Errorf("code = %d, want 0; output:\n%s", code, out)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

const twoMustFR = `| ID | Prio | Requirement |
|---|---|---|
| FR-AB-01 | MUST | a must requirement |
| FR-CD-01 | MUST | another must requirement |
`

func TestQA_MNT_02_EachUncoveredIDGetsExactlyOneSortedLine(t *testing.T) {
	cfg := newConfig(t, twoMustFR, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	want := "FR-AB-01: no test name and no manual checklist entry\n" +
		"FR-CD-01: no test name and no manual checklist entry\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestTR_30_PendingEntrySuppressesMissingID(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	out, code := runFixture(t, cfg)
	if code != 0 || out != "" {
		t.Errorf("code = %d, output = %q; want 0 and empty", code, out)
	}
}

func TestTR_30_PendingIgnoresBlankAndCommentLines(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)
	writeFile(t, cfg.PendingPath, "# header\n\nFR-AB-01\n\n# trailer\n")

	out, code := runFixture(t, cfg)
	if code != 0 || out != "" {
		t.Errorf("code = %d, output = %q; want 0 and empty", code, out)
	}
}

func TestTR_30_MissingPendingFileIsAnEmptyList(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)

	out, code := runFixture(t, cfg)
	if code != 1 || !strings.Contains(out, "FR-AB-01: no test name") {
		t.Errorf("code = %d, output = %q; want 1 and the missing line", code, out)
	}
}

func TestTR_30_StalePendingEntryIsReported(t *testing.T) {
	cfg := newConfig(t, oneMustFR, noRows, noRows)
	writeFile(t, filepath.Join(cfg.GoRoot, "a_test.go"), "package a\n\nfunc TestFR_AB_01_Thing(t *testing.T) {}\n")
	writeFile(t, cfg.PendingPath, "FR-AB-01\n")

	out, code := runFixture(t, cfg)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	want := "FR-AB-01: no longer missing; remove it from " + cfg.PendingPath + "\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}
