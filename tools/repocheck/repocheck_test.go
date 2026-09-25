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

// assertNoLineMatches fails once per line of text that re matches.
func assertNoLineMatches(t *testing.T, name, text string, re *regexp.Regexp, problem string) {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if re.MatchString(line) {
			t.Errorf("%s:%d %s: %s", name, i+1, problem, strings.TrimSpace(line))
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
	cmd := exec.CommandContext(t.Context(), "go", "mod", "edit", "-json")
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

	if pkg := readFrontendPackageJSON(t, root); !strings.HasPrefix(pkg.PackageManager, "pnpm@") {
		t.Errorf("packageManager = %q, want pnpm@<version>", pkg.PackageManager)
	}
	assertFileExists(t, root, "frontend/pnpm-lock.yaml")

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

	assertContainsAll(t, "Taskfile.yml", readFile(t, root, "Taskfile.yml"), "PACKAGE_MANAGER: pnpm")
	for _, tf := range taskfiles {
		assertNoLineMatches(t, tf, readFile(t, root, tf), callsNpm, "calls npm")
	}
}

// offlineEnv is what a test run must set so it can never reach the network.
var offlineEnv = []string{"GOPROXY: off", "GOFLAGS: -mod=readonly", "GOTOOLCHAIN: local"}

// yamlBlock returns the two-space-indented body of a top-level YAML key
// ("key:\n" through the next line at that indent) — the shape of both a
// Taskfile task and a GitHub Actions job.
func yamlBlock(t *testing.T, text, key string) string {
	t.Helper()
	m := regexp.MustCompile(`(?ms)^  ` + key + `:\n(.*?)(?:^  \S|\z)`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no key %q", key)
	}
	return m[1]
}

func TestQA_MNT_03_TestTaskAndCIRunOffline(t *testing.T) {
	root := repoRoot(t)

	testTask := yamlBlock(t, readFile(t, root, "Taskfile.yml"), "test:go")
	assertContainsAll(t, "Taskfile.yml test:go task", testTask, offlineEnv...)
	assertContainsAll(t, "ci.yml", readFile(t, root, ".github/workflows/ci.yml"), offlineEnv...)
	assertContainsAll(t, "build/Taskfile.yml", readFile(t, root, "build/Taskfile.yml"), "pnpm install --frozen-lockfile")
}

func TestTR_31_PerfBenchAndPlaywrightWiring(t *testing.T) {
	root := repoRoot(t)

	pkg := readFrontendPackageJSON(t, root)
	if pin := pkg.DevDependencies["@playwright/test"]; pin == "" || strings.ContainsAny(pin, "^~") {
		t.Errorf(`@playwright/test = %q, want an exact version (no "^"/"~")`, pin)
	}

	assertContainsAll(t, "flake.nix", readFile(t, root, "flake.nix"), "playwright-driver.browsers")
	assertContainsAll(t, ".golangci.yml", readFile(t, root, ".golangci.yml"), "build-tags: [bench]")
	assertContainsAll(t, "bench.yml", readFile(t, root, ".github/workflows/bench.yml"), "schedule:")
	assertContainsAll(t, ".gitignore", readFile(t, root, ".gitignore"), ".bench/")

	taskfile := readFile(t, root, "Taskfile.yml")
	ciTask := yamlBlock(t, taskfile, "ci")
	for _, name := range []string{"bench", "e2e"} {
		yamlBlock(t, taskfile, name) // fails via t.Fatalf if the task is missing
		if strings.Contains(ciTask, "task: "+name) {
			t.Errorf("ci task must not run %s", name)
		}
	}
}

// The frontend's config files and package.json match what the official
// create-vue scaffold generates.

type packageJSON struct {
	PackageManager  string            `json:"packageManager"`
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
	assertScriptCatchesFixture(t, repoRoot(t), "lint", "src/__fixture_debugger.ts", "export function fixture(): void {\n  debugger\n}\n", "debugger")
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

// leafTsconfig is the subset of a leaf tsconfig these checks read.
type leafTsconfig struct {
	CompilerOptions struct {
		Composite       bool   `json:"composite"`
		TsBuildInfoFile string `json:"tsBuildInfoFile"`
	} `json:"compilerOptions"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
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
		var cfg leafTsconfig
		readJSONC(t, root, "frontend/"+leaf, &cfg)
		if !cfg.CompilerOptions.Composite {
			t.Errorf("%s: compilerOptions.composite = false, want true", leaf)
		}
		if cfg.CompilerOptions.TsBuildInfoFile == "" {
			t.Errorf("%s: compilerOptions.tsBuildInfoFile missing", leaf)
		}
		if leaf == "tsconfig.app.json" && !slices.Contains(cfg.Exclude, "src/**/*.test.ts") {
			t.Errorf("%s: exclude = %v, want it to contain src/**/*.test.ts", leaf, cfg.Exclude)
		}
		if leaf == "tsconfig.vitest.json" && !slices.Contains(cfg.Include, "src/**/*.test.ts") {
			t.Errorf("%s: include = %v, want it to contain src/**/*.test.ts", leaf, cfg.Include)
		}
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
	assertScriptCatchesFixture(t, repoRoot(t), "type-check", "src/__fixture_type_error.test.ts", `import { ref } from 'vue'

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

	testTask := yamlBlock(t, taskfile, "test")
	assertContainsAll(t, "Taskfile.yml test task", testTask, "SCRIPT: test:unit")

	lintTask := yamlBlock(t, taskfile, "lint")
	assertContainsAll(t, "Taskfile.yml lint task", lintTask, "SCRIPT: lint", "SCRIPT: type-check")

	e2eTask := yamlBlock(t, taskfile, "e2e")
	assertContainsAll(t, "Taskfile.yml e2e task", e2eTask, "common:frontend:run", "SCRIPT: test:e2e")

	for _, tf := range []string{"Taskfile.yml", "build/Taskfile.yml"} {
		assertNoLineMatches(t, tf, readFile(t, root, tf), callsOldFrontendScript, "still references the old script name")
	}
}

func TestTR_33_DocsNameOxlint(t *testing.T) {
	root := repoRoot(t)
	assertContainsAll(t, "CONTRIBUTING.md", readFile(t, root, "CONTRIBUTING.md"), "oxlint")
	assertContainsAll(t, "docs/architecture/testing.md", readFile(t, root, "docs/architecture/testing.md"), "oxlint")
}

// matrixEntry is one `- os: ..." / "task: ...` pair of a CI matrix.
type matrixEntry struct{ os, task string }

var matrixEntryPattern = regexp.MustCompile(`(?m)^\s*-\s*os:\s*(\S+)\n\s*task:\s*(\S+)`)

func matrixEntries(workflow string) []matrixEntry {
	var entries []matrixEntry
	for _, m := range matrixEntryPattern.FindAllStringSubmatch(workflow, -1) {
		entries = append(entries, matrixEntry{os: m[1], task: m[2]})
	}
	return entries
}

func TestQA_PORT_01_TR_22_CIMatrixCoversSupportedPlatforms(t *testing.T) {
	root := repoRoot(t)
	entries := matrixEntries(readFile(t, root, ".github/workflows/ci.yml"))

	want := []matrixEntry{
		{os: "ubuntu-24.04", task: "ci"},
		{os: "ubuntu-24.04", task: "e2e:real"},
		{os: "windows-latest", task: "ci"},
		{os: "macos-latest", task: "build"},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("CI matrix = %v, want %v", entries, want)
	}
}

func TestQA_COMP_03_CIRunsGoTestsOnGit242(t *testing.T) {
	root := repoRoot(t)
	taskfile := readFile(t, root, "Taskfile.yml")

	yamlBlock(t, taskfile, "test:go") // fails via t.Fatalf if missing

	// test:go:race skips packages that need a frontend build (this job
	// builds none): cmd/nebegit (embeds frontend/dist) and tools/repocheck
	// itself (its own tests shell out to pnpm).
	raceTask := yamlBlock(t, taskfile, "test:go:race")
	assertContainsAll(t, "Taskfile.yml test:go:race task", raceTask, "./internal/...", "./cmd/nebegit-core/...")
	if strings.Contains(raceTask, "./...") {
		t.Error(`Taskfile.yml test:go:race task must name its packages, not "./..."`)
	}

	job := yamlBlock(t, readFile(t, root, ".github/workflows/ci.yml"), "git-2-42")
	assertContainsAll(t, "ci.yml git-2-42 job", job,
		append([]string{"runs-on: ubuntu-24.04", "git-2.42.4", "actions/cache", "GITHUB_PATH", "task test:go:race"}, offlineEnv...)...)
}

// TestQA_PORT_04_ReleaseDarwinJobIsBestEffort also checks that each release
// job runs on a host of its target architecture, so no GOARCH override is
// needed.
func TestQA_PORT_04_ReleaseDarwinJobIsBestEffort(t *testing.T) {
	root := repoRoot(t)
	release := readFile(t, root, ".github/workflows/release.yml")

	darwin := yamlBlock(t, release, "darwin")
	if !strings.Contains(darwin, "continue-on-error: true") {
		t.Error(`darwin job missing "continue-on-error: true"`)
	}

	publish := yamlBlock(t, release, "publish")
	if strings.Contains(publish, "needs.darwin.result") {
		t.Error("publish job's if: must not gate on needs.darwin.result")
	}

	runsOn := map[string]string{"windows": "windows-latest", "linux": "ubuntu-24.04", "darwin": "macos-latest"}
	for job, want := range runsOn {
		body := yamlBlock(t, release, job)
		if !strings.Contains(body, "runs-on: "+want) {
			t.Errorf("%s job missing \"runs-on: %s\"", job, want)
		}
		if strings.Contains(body, "GOARCH") || strings.Contains(body, "GOOS:") {
			t.Errorf("%s job sets an explicit GOOS/GOARCH override; the runner's own architecture already matches the target", job)
		}
	}
}

func TestQA_INST_02_ReleaseWorkflowTriggersOnVersionTagOnly(t *testing.T) {
	root := repoRoot(t)
	release := readFile(t, root, ".github/workflows/release.yml")

	on := regexp.MustCompile(`(?ms)^on:\n(.*?)(?:^\S|\z)`).FindStringSubmatch(release)
	if on == nil {
		t.Fatal("no on: trigger block")
	}
	assertContainsAll(t, "release.yml on:", on[1], "tags: ['v*']")
	for _, unwanted := range []string{"branches:", "workflow_dispatch", "schedule:"} {
		if strings.Contains(on[1], unwanted) {
			t.Errorf("release.yml on: must not contain %q, only a version-tag push", unwanted)
		}
	}

	publish := yamlBlock(t, release, "publish")
	if !strings.Contains(publish, "sha256sum") {
		t.Error("publish job missing a checksum command")
	}
	if !strings.Contains(publish, "action-gh-release") {
		t.Error("publish job missing a GitHub Release upload action")
	}
}

// forbiddenInstallerTasks are task names release.yml must never invoke: they
// build an installer or a package format v1 doesn't ship. deb/rpm are
// shipped, built via linux:package:dryrun, so they're not forbidden;
// linux:create:aur (Arch) isn't shipped.
var forbiddenInstallerTasks = []string{
	"linux:create:aur",
	"darwin:package:dmg", "create:nsis:installer", "create:msix:package",
}

// bareWindowsPackage matches windows:package, with or without a :dryrun
// suffix; only the :dryrun form (the zip-into-dist/ step) is allowed.
var bareWindowsPackage = regexp.MustCompile(`\bwindows:package\b(?::dryrun)?`)

func TestQA_INST_03_ReleaseWorkflowBuildsNoInstaller(t *testing.T) {
	root := repoRoot(t)
	release := readFile(t, root, ".github/workflows/release.yml")

	for i, line := range strings.Split(release, "\n") {
		if !strings.Contains(line, "task") {
			continue
		}
		for _, forbidden := range forbiddenInstallerTasks {
			if strings.Contains(line, forbidden) {
				t.Errorf("release.yml:%d invokes an installer task (%q): %s", i+1, forbidden, strings.TrimSpace(line))
			}
		}
		if m := bareWindowsPackage.FindString(line); m != "" && m != "windows:package:dryrun" {
			t.Errorf("release.yml:%d invokes windows:package (an installer), not windows:package:dryrun: %s", i+1, strings.TrimSpace(line))
		}
	}
}

func TestQA_OSS_03_ReadmeDocumentsInstallAndLimits(t *testing.T) {
	root := repoRoot(t)
	assertContainsAll(t, "README.md", readFile(t, root, "README.md"),
		"## Install", "## First start", "## Editor setup", "## Known limits", "WSL", "ssh-agent", "SmartScreen")
}

// nfpmTemplateLeftovers are unedited-Wails-template values that must not
// survive in the Linux package's nfpm config.
var nfpmTemplateLeftovers = []string{
	"My Company", "wails.io", `license: "MIT"`, "/usr/local/bin/nebegit",
	"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "128x128",
	`description: "A nebegit application"`,
}

func TestQA_INST_01_NfpmPackageIdentity(t *testing.T) {
	root := repoRoot(t)
	nfpm := readFile(t, root, "build/linux/nfpm/nfpm.yaml")

	assertContainsAll(t, "nfpm.yaml", nfpm,
		`name: "nebegit"`,
		`license: "BSD-3-Clause"`,
		`homepage: "https://github.com/brogli/nebegit"`,
		`dst: "/usr/bin/nebegit"`,
		"hicolor/1024x1024/apps/nebegit.png",
	)
	for _, unwanted := range nfpmTemplateLeftovers {
		if strings.Contains(nfpm, unwanted) {
			t.Errorf("nfpm.yaml still has the unedited template's %q", unwanted)
		}
	}
}

// nfpmDependsBlock matches one YAML "depends:" list (the top-level block or
// an override's).
var nfpmDependsBlock = regexp.MustCompile(`(?m)^depends:\n((?:\s+- .+\n)+)`)

func TestQA_INST_01_NfpmDependsOnUnversionedGit(t *testing.T) {
	root := repoRoot(t)
	nfpm := readFile(t, root, "build/linux/nfpm/nfpm.yaml")

	blocks := nfpmDependsBlock.FindAllStringSubmatch(nfpm, -1)
	if len(blocks) == 0 {
		t.Fatal("no depends: blocks found")
	}
	for _, m := range blocks {
		if !strings.Contains(m[1], "- git\n") {
			t.Errorf("depends block missing unversioned git:\n%s", m[1])
		}
	}
}

func TestQA_INST_03_ReleaseWorkflowShipsDebRpmTarball(t *testing.T) {
	root := repoRoot(t)
	release := readFile(t, root, ".github/workflows/release.yml")

	linux := yamlBlock(t, release, "linux")
	assertContainsAll(t, "release.yml linux job", linux,
		"nebegit-linux-amd64.deb", "nebegit-linux-amd64.rpm", "nebegit-linux-amd64.tar.gz",
		"linux:package:dryrun")
	if strings.Contains(release, "AppImage") {
		t.Error("release.yml still references an AppImage")
	}
}

func TestBuildTaskfileDropsUnusedWailsScaffoldTasks(t *testing.T) {
	root := repoRoot(t)
	build := readFile(t, root, "build/Taskfile.yml")

	unusedTask := regexp.MustCompile(`(?m)^  (build:server|run:server|build:docker|run:docker):`)
	if m := unusedTask.FindString(build); m != "" {
		t.Errorf("build/Taskfile.yml still has unused task %q", strings.TrimSuffix(strings.TrimSpace(m), ":"))
	}

	rootTaskfile := readFile(t, root, "Taskfile.yml")
	if !strings.Contains(rootTaskfile, "\n  build:server:\n") {
		t.Error("root Taskfile.yml lost its own build:server task (used by the real-backend e2e suite)")
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

// findOwnerSpecificDefaults walks root's files, skipping _test.go/.test.ts
// files and any testing/fixtures directory, and returns the paths of files
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
			if d.Name() == "fixtures" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts") {
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

func TestQA_OSS_04_NoOwnerSpecificDefaults(t *testing.T) {
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

// castsEventData matches an event handler casting its data rather than
// relying on the type generated from the event's Go registration.
var castsEventData = regexp.MustCompile(`\bev\.data as\b`)

func TestTR_28_FrontendReadsEventDataWithoutCasts(t *testing.T) {
	root := repoRoot(t)
	err := filepath.WalkDir(filepath.Join(root, "frontend/src"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		assertNoLineMatches(t, path, string(content), castsEventData, "casts event data")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// replacesToastLook matches a Toast that is moved from its default position
// or whose container slot replaces the standard message.
var replacesToastLook = regexp.MustCompile(`#container|\bposition=`)

var (
	// nativeControl matches a native form control in a template.
	nativeControl = regexp.MustCompile(`<(button|input|select|textarea)([\s>/]|$)`)
	styleBlock    = regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)
	cssComment    = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssRule       = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	// controlSelector matches a selector naming a native control or one of
	// OpenVue's own classes.
	controlSelector = regexp.MustCompile(`(^|[\s,>+~(])(button|input|select|textarea)\b|\.p-[a-z]`)
	// lookProperty matches a declaration that changes how something looks,
	// not where it sits.
	lookProperty = regexp.MustCompile(`(^|;)\s*(color|background[\w-]*|border[\w-]*|padding[\w-]*|font[\w-]*|box-shadow|outline[\w-]*|opacity)\s*:`)
)

// restyledControls lists the CSS rules in css that change the look of a
// native control or an OpenVue component.
func restyledControls(css string) []string {
	var found []string
	for _, m := range cssRule.FindAllStringSubmatch(cssComment.ReplaceAllString(css, ""), -1) {
		selector := strings.TrimSpace(m[1])
		if controlSelector.MatchString(selector) && lookProperty.MatchString(m[2]) {
			found = append(found, selector)
		}
	}
	return found
}

func TestTR_37_ControlsAreOpenVueComponentsInTheirDefaultLook(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		css := `button:not(.p-button) { padding: 0; } .row { display: flex; }
			.bar :deep(.p-inputtext) { width: 100%; } .x :deep(.p-button) { color: red; }`
		got := restyledControls(css)
		want := []string{"button:not(.p-button)", ".x :deep(.p-button)"}
		if !slices.Equal(got, want) {
			t.Errorf("restyledControls(fixture) = %q, want %q", got, want)
		}
	})

	t.Run("real tree", func(t *testing.T) {
		root := repoRoot(t)
		err := filepath.WalkDir(filepath.Join(root, "frontend/src"), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			var css strings.Builder
			switch filepath.Ext(path) {
			case ".vue":
				content := readFile(t, "", path)
				assertNoLineMatches(t, path, content, nativeControl, "native control instead of an OpenVue component")
				for _, m := range styleBlock.FindAllStringSubmatch(content, -1) {
					css.WriteString(m[1])
				}
			case ".css":
				css.WriteString(readFile(t, "", path))
			}
			for _, selector := range restyledControls(css.String()) {
				t.Errorf("%s: %s restyles a control", path, selector)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestTR_36_NoticesUseTheStandardToast(t *testing.T) {
	const noticeArea = "frontend/src/components/NoticeArea.vue"
	assertNoLineMatches(t, noticeArea, readFile(t, repoRoot(t), noticeArea), replacesToastLook, "replaces the Toast's standard look or position")
}
