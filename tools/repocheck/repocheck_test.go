// Package repocheck holds the tests that run the repo's tools on planted
// faults or scan the working tree. Tests that plant files live here only,
// so they never run in parallel with each other.
package repocheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func assertContainsAll(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s: missing %q", name, w)
		}
	}
}

func TestTR_23_NoOtherPackageManagersLockfile(t *testing.T) {
	root := repoRoot(t)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == "node_modules" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		if name == "package-lock.json" || name == "yarn.lock" {
			t.Errorf("foreign lockfile: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// frontendToolchainAvailable reports whether the real pnpm/oxlint/vue-tsc
// toolchain can be shelled out to. It's absent on a bare `go test ./...`
// checkout (no network, no `frontend/node_modules`); `task ci` installs the
// frontend deps first, so those checks run for real there.
func frontendToolchainAvailable(root string) bool {
	if _, err := exec.LookPath("pnpm"); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "frontend", "node_modules"))
	return err == nil
}

// writeFrontendFixture creates rel (relative to frontend/) with content and
// removes it via t.Cleanup, so a failing test run never leaves it behind.
func writeFrontendFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, "frontend", rel)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

// assertScriptCatchesFixture writes rel (relative to frontend/) with
// content, a deliberate finding, runs `pnpm run <script>`, asserts it fails
// and names the fixture and every one of wants, removes the fixture, then
// asserts a clean run passes.
func assertScriptCatchesFixture(t *testing.T, root, script, rel, content string, wants ...string) {
	t.Helper()
	if !frontendToolchainAvailable(root) {
		t.Skip("pnpm not on PATH or frontend/node_modules missing")
	}

	writeFrontendFixture(t, root, rel, content)

	out, err := runFrontendScript(t, root, script)
	if err == nil {
		t.Fatalf("pnpm run %s with %s: want non-zero exit, got 0\n%s", script, rel, out)
	}
	assertContainsAll(t, script+" output", out, append(wants, filepath.Base(rel))...)

	if err := os.Remove(filepath.Join(root, "frontend", rel)); err != nil {
		t.Fatal(err)
	}
	if out, err := runFrontendScript(t, root, script); err != nil {
		t.Errorf("pnpm run %s on a clean tree: want exit 0, got error: %v\n%s", script, err, out)
	}
}

func runFrontendScript(t *testing.T, root, script string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "pnpm", "run", script)
	cmd.Dir = filepath.Join(root, "frontend")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestTR_33_OxlintCatchesRealFindings(t *testing.T) {
	assertScriptCatchesFixture(t, repoRoot(t), "lint", "src/__fixture_debugger.ts", "export function fixture(): void {\n  debugger\n}\n", "debugger")
}

func TestTR_33_EslintCatchesCrossFolderRelativeImport(t *testing.T) {
	root := repoRoot(t)
	if !frontendToolchainAvailable(root) {
		t.Skip("pnpm not on PATH or frontend/node_modules missing")
	}

	appFile := "src/lib/__fixture_import.ts"
	testFile := "src/lib/__tests__/__fixture_import.spec.ts"
	writeFrontendFixture(t, root, appFile, "import { useRepoStore } from '../stores/repo'\n\nuseRepoStore()\n")
	writeFrontendFixture(t, root, testFile,
		"import { useRepoStore } from '../../stores/repo'\n\nuseRepoStore()\n")

	out, err := runFrontendScript(t, root, "lint")
	if err == nil {
		t.Fatalf("pnpm run lint with %s and %s: want non-zero exit, got 0\n%s", appFile, testFile, out)
	}
	assertContainsAll(t, "lint output", out,
		filepath.Base(appFile), filepath.Base(testFile), "no-restricted-imports")

	for _, f := range []string{appFile, testFile} {
		if err := os.Remove(filepath.Join(root, "frontend", f)); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := runFrontendScript(t, root, "lint"); err != nil {
		t.Errorf("pnpm run lint on a clean tree: want exit 0, got error: %v\n%s", err, out)
	}
}

func TestTR_33_TypeCheckCatchesVueTemplateError(t *testing.T) {
	assertScriptCatchesFixture(t, repoRoot(t), "type-check", "src/__fixture_type_error.vue", `<script setup lang="ts">
import { ref } from 'vue'
const count = ref<number>()
count.value = 'not a number'
</script>

<template>
  <div>{{ count }}</div>
</template>
`)
}

func TestTR_33_TypeCheckCatchesTestFileError(t *testing.T) {
	assertScriptCatchesFixture(t, repoRoot(t), "type-check", "src/__tests__/__fixture_type_error.spec.ts", `import { ref } from 'vue'

const count = ref<number>()
count.value = 'not a number'
`)
}

// vitestTestPattern matches Vitest's default test file naming (.test. or
// .spec., ts or js).
var vitestTestPattern = regexp.MustCompile(`\.(test|spec)\.(ts|js)$`)

func TestTR_33_TestFilesLiveInTestsFolderAsSpecTs(t *testing.T) {
	root := filepath.Join(repoRoot(t), "frontend/src")
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		inTestsFolder := filepath.Base(filepath.Dir(path)) == "__tests__"
		isTestFile := vitestTestPattern.MatchString(path)
		if (isTestFile || inTestsFolder) && (!inTestsFolder || !strings.HasSuffix(path, ".spec.ts")) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("test files outside src/**/__tests__/*.spec.ts:\n%s", strings.Join(offenders, "\n"))
	}
}

// homePathPattern, remoteURLPattern, and branchFieldPattern flag content
// that assumes one developer's own setup: an absolute home directory, a
// hard-coded git remote address, or a hard-coded "branch" field.
var (
	homePathPattern    = regexp.MustCompile(`/home/[a-z][a-z0-9_-]*/|C:\\Users\\[A-Za-z0-9_ .-]+`)
	remoteURLPattern   = regexp.MustCompile(`git@[A-Za-z0-9._-]+:|[a-z][a-z0-9+.-]*://[^\s"']*\.git\b|ssh://`)
	branchFieldPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_-])branch\b\s*[:=]\s*"([^"]+)"`)
)

// allowedPlumbingBranches are the branch names git itself defaults to;
// anything else hard-coded as a "branch" field is a suspect owner default.
var allowedPlumbingBranches = map[string]bool{"main": true, "master": true, "HEAD": true}

// findOwnerSpecificDefaults walks root's files, skipping _test.go files and
// any testing/fixtures/__tests__ directory, and returns the paths of files
// containing a hard-coded home directory, remote URL, or non-plumbing
// branch name.
func findOwnerSpecificDefaults(t *testing.T, root string) []string {
	t.Helper()
	var violations []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "fixtures" || d.Name() == "__tests__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)

		if homePathPattern.MatchString(text) || remoteURLPattern.MatchString(text) {
			violations = append(violations, path)
			return nil
		}
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts") {
			for _, m := range branchFieldPattern.FindAllStringSubmatch(text, -1) {
				if !allowedPlumbingBranches[m[1]] {
					violations = append(violations, path)
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return violations
}

func TestOSS_04_NoOwnerSpecificDefaults(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture := func(name, content string) {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		writeFixture("dirty.go", "package fixture\n\n// installed at /home/someone/repos/app\n")
		writeFixture("clean.go", "package fixture\n")

		got := findOwnerSpecificDefaults(t, dir)
		if len(got) != 1 || !strings.HasSuffix(got[0], "dirty.go") {
			t.Errorf("findOwnerSpecificDefaults(fixture) = %v, want exactly one match, dirty.go", got)
		}
	})

	t.Run("real tree", func(t *testing.T) {
		root := repoRoot(t)
		var violations []string
		for _, dir := range []string{"internal", "cmd", "frontend/src", "build"} {
			violations = append(violations, findOwnerSpecificDefaults(t, filepath.Join(root, dir))...)
		}
		if len(violations) > 0 {
			t.Errorf("owner-specific defaults found:\n%s", strings.Join(violations, "\n"))
		}
	})
}

// plant writes content to rel under the repo root for the rest of the test.
func plant(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

// A native control in a template and a restyled OpenVue class in CSS are
// lint findings, not a repo check.
func TestTR_37_TreefmtCIFailsOnANativeControlAndARestyledControl(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	vueFault := "frontend/src/components/__fixture_tr37.vue"
	cssFault := "frontend/src/assets/__fixture_tr37.css"
	plant(t, root, vueFault, "<template>\n  <button>x</button>\n</template>\n")
	plant(t, root, cssFault, ".fixture .p-button {\n  color: red;\n}\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", vueFault, cssFault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed a native control and a restyled OpenVue class:\n%s", out)
	}
	for _, want := range []string{"vue/no-restricted-html-elements", "selector-disallowed-list", cssFault} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}
}

// treefmt --ci runs every linter, then every formatter, and fails on a
// finding a fix cannot remove as well as on any file it would change. Like
// every test that plants files in the tree, it lives in this package so
// they run one after another.
func TestTR_38_TreefmtCIFailsOnALintErrorAndAnUnformattedFile(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	lintFault := "frontend/src/__fixture_treefmt_lint.ts"
	formatFault := "docs/__fixture_treefmt_format.md"
	plant(t, root, lintFault, "export function fixture(): void {\n  for (let i = 0; i < 3; i--) {\n    console.log(i)\n  }\n}\n")
	plant(t, root, formatFault, "#  Title\n\n\n*   item\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", lintFault, formatFault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed a lint error and an unformatted file:\n%s", out)
	}
	for _, want := range []string{"for-direction", formatFault} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}
}

// One treefmt run covers both faults: an e2e spec only the Playwright
// plugin catches, and a __tests__ spec only the Vitest plugin catches
// (oxlint's own vitest plugin already reports the more common findings).
func TestTR_39_TreefmtCIFailsOnPlaywrightAndVitestFindings(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	playwrightFault := "frontend/e2e/__fixture_missing_await.spec.ts"
	vitestFault := "frontend/src/lib/__tests__/__fixture_identical_title.spec.ts"
	// expect(locator).toBeVisible() is itself auto-fixable (the rule
	// inserts the missing await), so it never survives to be named in
	// --ci output; page.waitForResponse has no such fix.
	plant(t, root, playwrightFault,
		"import { test } from '@playwright/test'\n\n"+
			"test('fixture case', async ({ page }) => {\n"+
			"  page.waitForResponse('**/api')\n"+
			"})\n")
	plant(t, root, vitestFault,
		"import { expect, it } from 'vitest'\n\n"+
			"it('fixture case', () => {\n  expect(true).toBe(true)\n})\n\n"+
			"it('fixture case', () => {\n  expect(true).toBe(true)\n})\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", playwrightFault, vitestFault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed a missing Playwright await and a duplicate Vitest title:\n%s", out)
	}
	for _, want := range []string{"playwright/missing-playwright-await", "vitest/no-identical-title"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}
}

func TestTR_40_TreefmtCIFailsOnUntypedTypeScriptAndRuntimeProps(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	libFault := "frontend/src/lib/__fixture_tr40_lib.ts"
	plant(t, root, libFault, "export function fixture(value: string | null) {\n"+
		"  const cast = value as string\n"+
		"  // eslint-disable-next-line @typescript-eslint/no-explicit-any\n"+
		"  const loose: any = cast\n"+
		"  return value!.length + loose.length\n"+
		"}\n")

	vueFault := "frontend/src/components/__fixture_tr40.vue"
	plant(t, root, vueFault, "<script setup lang=\"ts\">\n"+
		"defineProps({ label: String })\n"+
		"</script>\n"+
		"<template>\n"+
		"  <span>{{ label }}</span>\n"+
		"</template>\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", libFault, vueFault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed an unexplained '!', an 'as' cast, an untyped export, an undescribed disable and a runtime defineProps:\n%s", out)
	}
	for _, want := range []string{
		"no-non-null-assertion",
		"consistent-type-assertions",
		"explicit-module-boundary-types",
		"require-description",
		"define-props-declaration",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}
}

func TestTR_40_TreefmtCIAllowsNonNullAssertionInTests(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	testFault := "frontend/src/lib/__tests__/__fixture_tr40.spec.ts"
	plant(t, root, testFault, "import { describe, expect, it } from 'vitest'\n\n"+
		"describe('fixture', () => {\n"+
		"  it('reads the first row', () => {\n"+
		"    const rows: { x: number }[] = [{ x: 1 }]\n"+
		"    expect(rows[0]!.x).toBe(1)\n"+
		"  })\n"+
		"})\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", testFault)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("treefmt --ci on a test's non-null assertion: want exit 0, got error: %v\n%s", err, out)
	}
}

// stylelint checks CSS files and <style> blocks alike and knows
// Tailwind's own at-rules.
func TestTR_39_TreefmtCIFailsOnUnknownCSSPropertiesAndPassesTailwindAtRules(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	cssFault := "frontend/src/assets/__fixture_stylelint.css"
	vueFault := "frontend/src/components/__fixture_stylelint.vue"
	plant(t, root, cssFault, ".cm-fixture {\n  colr: red;\n}\n")
	plant(t, root, vueFault, "<template>\n  <span />\n</template>\n\n"+
		"<style>\n.cm-fixture {\n  colr: red;\n}\n</style>\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", cssFault, vueFault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed an unknown CSS property:\n%s", out)
	}
	for _, want := range []string{"property-no-unknown", "__fixture_stylelint.css", "__fixture_stylelint.vue"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}

	tailwind := "frontend/src/assets/__fixture_tailwind.css"
	plant(t, root, tailwind, "@import 'tailwindcss';\n\n"+
		"@custom-variant fixture (&:where(.fixture, .fixture *));\n\n"+
		"@theme {\n  --color-fixture: red;\n}\n")
	cmd = exec.CommandContext(t.Context(), "treefmt", "--ci", tailwind)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("treefmt --ci on Tailwind's at-rules: want exit 0, got error: %v\n%s", err, out)
	}
}

// A planted component's utility classes reach the built CSS: one from
// OpenVue's tokens, one from the app's tones, one under the app's dark
// class.
func TestTR_43_BuildTurnsUtilityClassesIntoCSS(t *testing.T) {
	root := repoRoot(t)
	if !frontendToolchainAvailable(root) {
		t.Skip("pnpm not on PATH or frontend/node_modules missing")
	}
	writeFrontendFixture(t, root, "src/__fixture_utilities.vue",
		"<template>\n  <p class=\"bg-primary text-tone-green dark:bg-surface-900\">x</p>\n</template>\n")

	if out, err := runFrontendScript(t, root, "build"); err != nil {
		t.Fatalf("pnpm run build: %v\n%s", err, out)
	}
	sheets, err := filepath.Glob(filepath.Join(root, "frontend", "dist", "assets", "*.css"))
	if err != nil || len(sheets) == 0 {
		t.Fatalf("no built stylesheet: %v", err)
	}
	var css strings.Builder
	for _, sheet := range sheets {
		css.WriteString(readFile(t, "/", sheet))
	}
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`\.bg-primary\{`),
		regexp.MustCompile(`\.text-tone-green\{`),
		regexp.MustCompile(`\.dark\\:bg-surface-900:where\(\.nebegit-dark`),
	} {
		if !want.MatchString(css.String()) {
			t.Errorf("built CSS has no rule matching %s", want)
		}
	}
}

// A <style> rule utilities can express is a stylelint finding, not a repo
// check; a keyframes block and a rule for CodeMirror's own DOM stay.
func TestTR_43_TreefmtCIFailsOnAStyleBlockRuleUtilitiesCanExpress(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("treefmt"); err != nil {
		t.Skip("treefmt not on PATH (outside the Nix shell)")
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules missing")
	}

	fault := "frontend/src/components/__fixture_tr43.vue"
	plant(t, root, fault, "<template>\n  <div class=\"fixture-row\" />\n</template>\n\n"+
		"<style scoped>\n.fixture-row {\n  display: flex;\n}\n</style>\n")

	cmd := exec.CommandContext(t.Context(), "treefmt", "--ci", fault)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("treefmt --ci passed a <style> rule utilities can express:\n%s", out)
	}
	for _, want := range []string{"selector-disallowed-list", fault} {
		if !strings.Contains(string(out), want) {
			t.Errorf("treefmt --ci output lacks %q:\n%s", want, out)
		}
	}

	pass := "frontend/src/components/__fixture_tr43_pass.vue"
	plant(t, root, pass, "<template>\n  <div class=\"fixture-editor\" />\n</template>\n\n"+
		"<style scoped>\n@keyframes fixture-spin {\n  from {\n    transform: rotate(0deg);\n  }\n\n"+
		"  to {\n    transform: rotate(360deg);\n  }\n}\n\n.cm-editor {\n  height: 100%;\n}\n</style>\n")

	cmd = exec.CommandContext(t.Context(), "treefmt", "--ci", pass)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("treefmt --ci on keyframes and a CodeMirror rule: want exit 0, got error: %v\n%s", err, out)
	}
}
