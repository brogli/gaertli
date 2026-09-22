// Package repocheck holds tests that pin repository conventions from
// docs/architecture/technical-requirements.md. They read files, not code.
package repocheck_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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

func assertFileExists(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
		t.Errorf("%s missing; it must be committed", rel)
	}
}

func assertContainsAll(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s: missing %q", name, w)
		}
	}
}

var jsonLineComment = regexp.MustCompile(`(?m)//.*$`)

// readJSONC reads rel (relative to root) and unmarshals it into v. oxlint's
// and TypeScript's own config formats allow "//" line comments (JSONC);
// Go's json package doesn't, so they're stripped first.
func readJSONC(t *testing.T, root, rel string, v any) {
	t.Helper()
	text := jsonLineComment.ReplaceAllString(readFile(t, root, rel), "")
	if err := json.Unmarshal([]byte(text), v); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

func TestTR_24_NixShellSuppliesEverythingButGo(t *testing.T) {
	root := repoRoot(t)

	if got := strings.TrimSpace(readFile(t, root, ".envrc")); got != "use flake" {
		t.Errorf(".envrc = %q, want \"use flake\"", got)
	}
	assertFileExists(t, root, "flake.lock")
	assertContainsAll(t, "flake.nix", readFile(t, root, "flake.nix"),
		"nixos-26.05", "nodejs_24", "pnpm_10", "golangci-lint", "pkg-config", "gtk4", "webkitgtk_6_0")
}

type goMod struct {
	Go        string
	Toolchain string
	Tool      []struct{ Path string }
}

func TestTR_25_GoModPinsToolchainAndTools(t *testing.T) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go mod edit -json: %v", err)
	}
	var m goMod
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}

	if m.Go != "1.26.0" {
		t.Errorf("go line = %q, want 1.26.0", m.Go)
	}
	if m.Toolchain == "" {
		t.Error("toolchain line missing")
	}

	tools := map[string]bool{}
	for _, tl := range m.Tool {
		tools[tl.Path] = true
		if strings.Contains(tl.Path, "golangci-lint") {
			t.Errorf("golangci-lint must not be a tool directive: %s", tl.Path)
		}
	}
	for _, want := range []string{
		"github.com/wailsapp/wails/v3/cmd/wails3",
		"github.com/go-task/task/v3/cmd/task",
	} {
		if !tools[want] {
			t.Errorf("tool directive missing: %s", want)
		}
	}
}

// taskfiles mirrors the includes in Taskfile.yml.
var taskfiles = []string{
	"Taskfile.yml",
	"build/Taskfile.yml",
	"build/linux/Taskfile.yml",
	"build/windows/Taskfile.yml",
	"build/darwin/Taskfile.yml",
}

var callsNpm = regexp.MustCompile(`(^|[\s"'])npm\s`)

func TestTR_23_PnpmIsTheOnlyPackageManager(t *testing.T) {
	root := repoRoot(t)

	var pkg struct{ PackageManager string }
	if err := json.Unmarshal([]byte(readFile(t, root, "frontend/package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pkg.PackageManager, "pnpm@") {
		t.Errorf("packageManager = %q, want pnpm@<version>", pkg.PackageManager)
	}
	assertFileExists(t, root, "frontend/pnpm-lock.yaml")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == "node_modules" || name == ".git" || name == ".direnv") {
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

	assertContainsAll(t, "Taskfile.yml", readFile(t, root, "Taskfile.yml"), "PACKAGE_MANAGER: pnpm")
	for _, tf := range taskfiles {
		for i, line := range strings.Split(readFile(t, root, tf), "\n") {
			if callsNpm.MatchString(line) {
				t.Errorf("%s:%d calls npm: %s", tf, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// offlineEnv is what a test run must set so it can never reach the network.
var offlineEnv = []string{"GOPROXY: off", "GOFLAGS: -mod=readonly", "GOTOOLCHAIN: local"}

// taskBody returns the YAML block of one top-level task in a Taskfile.
func taskBody(t *testing.T, taskfile, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?ms)^  ` + name + `:\n(.*?)(?:^  \S|\z)`).FindStringSubmatch(taskfile)
	if m == nil {
		t.Fatalf("no task %q", name)
	}
	return m[1]
}

func TestQA_MNT_03_TestTaskAndCIRunOffline(t *testing.T) {
	root := repoRoot(t)

	testTask := taskBody(t, readFile(t, root, "Taskfile.yml"), "test")
	assertContainsAll(t, "Taskfile.yml test task", testTask, offlineEnv...)
	assertContainsAll(t, "ci.yml", readFile(t, root, ".github/workflows/ci.yml"), offlineEnv...)
	assertContainsAll(t, "build/Taskfile.yml", readFile(t, root, "build/Taskfile.yml"), "pnpm install --frozen-lockfile")
}

func TestTR_31_PerfBenchAndPlaywrightWiring(t *testing.T) {
	root := repoRoot(t)

	var pkg struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(readFile(t, root, "frontend/package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	if pin := pkg.DevDependencies["@playwright/test"]; pin == "" || strings.ContainsAny(pin, "^~") {
		t.Errorf(`@playwright/test = %q, want an exact version (no "^"/"~")`, pin)
	}

	assertContainsAll(t, "flake.nix", readFile(t, root, "flake.nix"), "playwright-driver.browsers")
	assertContainsAll(t, ".golangci.yml", readFile(t, root, ".golangci.yml"), "build-tags: [bench]")
	assertContainsAll(t, "bench.yml", readFile(t, root, ".github/workflows/bench.yml"), "schedule:")
	assertContainsAll(t, ".gitignore", readFile(t, root, ".gitignore"), ".bench/")

	taskfile := readFile(t, root, "Taskfile.yml")
	ciTask := taskBody(t, taskfile, "ci")
	for _, name := range []string{"bench", "e2e"} {
		taskBody(t, taskfile, name) // fails via t.Fatalf if the task is missing
		if strings.Contains(ciTask, "task: "+name) {
			t.Errorf("ci task must not run %s", name)
		}
	}
}

// The frontend's config files and package.json match what the official
// create-vue scaffold generates.

type packageJSON struct {
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func readFrontendPackageJSON(t *testing.T, root string) packageJSON {
	t.Helper()
	var pkg packageJSON
	if err := json.Unmarshal([]byte(readFile(t, root, "frontend/package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	return pkg
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

func runFrontendScript(t *testing.T, root, script string) (string, error) {
	t.Helper()
	cmd := exec.Command("pnpm", "run", script)
	cmd.Dir = filepath.Join(root, "frontend")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestTR_33_OxlintConfigured(t *testing.T) {
	root := repoRoot(t)

	assertFileExists(t, root, "frontend/.oxlintrc.json")
	var oxlintrc struct {
		Plugins []string `json:"plugins"`
	}
	readJSONC(t, root, "frontend/.oxlintrc.json", &oxlintrc)
	for _, want := range []string{"typescript", "vue", "vitest"} {
		if !slices.Contains(oxlintrc.Plugins, want) {
			t.Errorf(".oxlintrc.json plugins = %v, want %q", oxlintrc.Plugins, want)
		}
	}

	pkg := readFrontendPackageJSON(t, root)
	for _, dep := range []string{"oxlint", "eslint-plugin-oxlint"} {
		if pkg.DevDependencies[dep] == "" {
			t.Errorf("devDependencies missing %s", dep)
		}
	}
	for _, script := range []string{"lint:oxlint", "lint:eslint"} {
		if pkg.Scripts[script] == "" {
			t.Errorf("scripts missing %s", script)
		}
	}
	lint := pkg.Scripts["lint"]
	if !strings.Contains(lint, "lint:oxlint") || !strings.Contains(lint, "lint:eslint") {
		t.Errorf("scripts.lint = %q, want it chaining lint:oxlint and lint:eslint", lint)
	}

	eslintConfig := readFile(t, root, "frontend/eslint.config.ts")
	assertContainsAll(t, "eslint.config.ts", eslintConfig, "eslint-plugin-oxlint")
	// oxlint's config must be the last entry: it turns off the ESLint rules
	// oxlint already covers, so it must win over everything before it. The
	// call's closing paren must follow directly, no further entry.
	last := regexp.MustCompile(`oxlint\.configs\['flat/recommended'\],?\s*\)`)
	if !last.MatchString(eslintConfig) {
		t.Errorf("eslint.config.ts: oxlint.configs[...] must be the last entry in defineConfigWithVueTs(...)")
	}
}

func TestTR_33_OxlintCatchesRealFindings(t *testing.T) {
	root := repoRoot(t)
	if !frontendToolchainAvailable(root) {
		t.Skip("pnpm not on PATH or frontend/node_modules missing")
	}

	const fixture = "src/__fixture_debugger.ts"
	writeFrontendFixture(t, root, fixture, "export function fixture(): void {\n  debugger\n}\n")

	out, err := runFrontendScript(t, root, "lint")
	if err == nil {
		t.Fatalf("pnpm run lint with a debugger statement: want non-zero exit, got 0\n%s", out)
	}
	if !strings.Contains(out, "__fixture_debugger.ts") {
		t.Errorf("lint output doesn't name the fixture:\n%s", out)
	}
	if !strings.Contains(out, "debugger") {
		t.Errorf("lint output doesn't name the correctness finding:\n%s", out)
	}

	if err := os.Remove(filepath.Join(root, "frontend", fixture)); err != nil {
		t.Fatal(err)
	}
	if out, err := runFrontendScript(t, root, "lint"); err != nil {
		t.Errorf("pnpm run lint on a clean tree: want exit 0, got error: %v\n%s", err, out)
	}
}

func TestTR_33_EslintConfigPrettierReplacesVueWrapper(t *testing.T) {
	root := repoRoot(t)

	pkg := readFrontendPackageJSON(t, root)
	if pkg.DevDependencies["eslint-config-prettier"] == "" {
		t.Error("devDependencies missing eslint-config-prettier")
	}
	if _, ok := pkg.DevDependencies["@vue/eslint-config-prettier"]; ok {
		t.Error("devDependencies must not have @vue/eslint-config-prettier")
	}

	eslintConfig := readFile(t, root, "frontend/eslint.config.ts")
	assertContainsAll(t, "eslint.config.ts", eslintConfig, "eslint-config-prettier")
	if strings.Contains(eslintConfig, "@vue/eslint-config-prettier") {
		t.Error("eslint.config.ts must not reference @vue/eslint-config-prettier")
	}
}

// tsconfigCompilerOptions is the subset of a leaf tsconfig's compilerOptions
// these checks read.
type tsconfigCompilerOptions struct {
	Composite       bool   `json:"composite"`
	TsBuildInfoFile string `json:"tsBuildInfoFile"`
}

func readTsconfig(t *testing.T, root, rel string) (compilerOptions tsconfigCompilerOptions, include, exclude []string, extends string) {
	t.Helper()
	var cfg struct {
		CompilerOptions tsconfigCompilerOptions `json:"compilerOptions"`
		Include         []string                `json:"include"`
		Exclude         []string                `json:"exclude"`
		Extends         string                  `json:"extends"`
	}
	readJSONC(t, root, "frontend/"+rel, &cfg)
	return cfg.CompilerOptions, cfg.Include, cfg.Exclude, cfg.Extends
}

func TestTR_33_TsconfigSplitIntoProjects(t *testing.T) {
	root := repoRoot(t)

	var rootCfg struct {
		Files      []string `json:"files"`
		References []struct {
			Path string `json:"path"`
		} `json:"references"`
	}
	readJSONC(t, root, "frontend/tsconfig.json", &rootCfg)
	if len(rootCfg.Files) != 0 {
		t.Errorf("tsconfig.json files = %v, want []", rootCfg.Files)
	}
	var refs []string
	for _, r := range rootCfg.References {
		refs = append(refs, r.Path)
	}
	for _, want := range []string{"./tsconfig.node.json", "./tsconfig.app.json", "./tsconfig.vitest.json"} {
		if !slices.Contains(refs, want) {
			t.Errorf("tsconfig.json references = %v, want %q", refs, want)
		}
	}

	for _, leaf := range []string{"tsconfig.app.json", "tsconfig.node.json", "tsconfig.vitest.json"} {
		compilerOptions, include, exclude, _ := readTsconfig(t, root, leaf)
		if !compilerOptions.Composite {
			t.Errorf("%s: compilerOptions.composite = false, want true", leaf)
		}
		if compilerOptions.TsBuildInfoFile == "" {
			t.Errorf("%s: compilerOptions.tsBuildInfoFile missing", leaf)
		}
		if leaf == "tsconfig.app.json" && !slices.Contains(exclude, "src/**/*.test.ts") {
			t.Errorf("%s: exclude = %v, want it to contain src/**/*.test.ts", leaf, exclude)
		}
		if leaf == "tsconfig.vitest.json" && !slices.Contains(include, "src/**/*.test.ts") {
			t.Errorf("%s: include = %v, want it to contain src/**/*.test.ts", leaf, include)
		}
	}
}

// assertTypeCheckCatchesFixture writes rel with content (a deliberate type
// error), runs `pnpm run type-check`, asserts it fails and names the
// fixture, removes the fixture, then asserts a clean run passes.
func assertTypeCheckCatchesFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	if !frontendToolchainAvailable(root) {
		t.Skip("pnpm not on PATH or frontend/node_modules missing")
	}

	writeFrontendFixture(t, root, rel, content)

	out, err := runFrontendScript(t, root, "type-check")
	if err == nil {
		t.Fatalf("pnpm run type-check with a type error: want non-zero exit, got 0\n%s", out)
	}
	if !strings.Contains(out, filepath.Base(rel)) {
		t.Errorf("type-check output doesn't name the fixture:\n%s", out)
	}

	if err := os.Remove(filepath.Join(root, "frontend", rel)); err != nil {
		t.Fatal(err)
	}
	if out, err := runFrontendScript(t, root, "type-check"); err != nil {
		t.Errorf("pnpm run type-check on a clean tree: want exit 0, got error: %v\n%s", err, out)
	}
}

func TestTR_33_TypeCheckCatchesVueTemplateError(t *testing.T) {
	assertTypeCheckCatchesFixture(t, repoRoot(t), "src/__fixture_type_error.vue", `<script setup lang="ts">
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
	assertTypeCheckCatchesFixture(t, repoRoot(t), "src/__fixture_type_error.test.ts", `import { ref } from 'vue'

const count = ref<number>()
count.value = 'not a number'
`)
}

func TestTR_33_ScriptsRenamedToScaffoldNames(t *testing.T) {
	root := repoRoot(t)
	pkg := readFrontendPackageJSON(t, root)

	for _, script := range []string{"test:unit", "type-check", "test:e2e"} {
		if pkg.Scripts[script] == "" {
			t.Errorf("scripts missing %s", script)
		}
	}
	for _, script := range []string{"test", "typecheck"} {
		if _, ok := pkg.Scripts[script]; ok {
			t.Errorf("scripts must not have %s", script)
		}
	}
	if got := pkg.Scripts["format"]; got != "prettier --write --experimental-cli src/" {
		t.Errorf(`scripts.format = %q, want "prettier --write --experimental-cli src/"`, got)
	}
}

var callsOldFrontendScript = regexp.MustCompile(`pnpm run test(\s|$)|SCRIPT: test \}|SCRIPT: typecheck`)

func TestTR_33_TaskfileCallersUseRenamedScripts(t *testing.T) {
	root := repoRoot(t)
	taskfile := readFile(t, root, "Taskfile.yml")

	testTask := taskBody(t, taskfile, "test")
	assertContainsAll(t, "Taskfile.yml test task", testTask, "SCRIPT: test:unit")

	lintTask := taskBody(t, taskfile, "lint")
	assertContainsAll(t, "Taskfile.yml lint task", lintTask, "SCRIPT: lint", "SCRIPT: type-check")

	e2eTask := taskBody(t, taskfile, "e2e")
	assertContainsAll(t, "Taskfile.yml e2e task", e2eTask, "common:frontend:run", "SCRIPT: test:e2e")

	for _, tf := range []string{"Taskfile.yml", "build/Taskfile.yml"} {
		for i, line := range strings.Split(readFile(t, root, tf), "\n") {
			if callsOldFrontendScript.MatchString(line) {
				t.Errorf("%s:%d still references the old script name: %s", tf, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestTR_33_DocsNameOxlint(t *testing.T) {
	root := repoRoot(t)
	assertContainsAll(t, "CONTRIBUTING.md", readFile(t, root, "CONTRIBUTING.md"), "oxlint")
	assertContainsAll(t, "docs/architecture/testing.md", readFile(t, root, "docs/architecture/testing.md"), "oxlint")
}
