# Dependency register (TR-20)

One row per direct dependency in `go.mod` or `package.json`. The choices behind them are in ADR-8.

| Dependency | Purpose | Alternatives looked at | Why chosen | Licence | Checked |
|---|---|---|---|---|---|
| `github.com/wailsapp/wails/v3` v3.0.0-beta.24 | desktop shell (ADR-2) | Wails v2, Electron-style shells | service model, TypeScript bindings, multi-window; exact pin, beta accepted | MIT | 2026-09-21 |
| `github.com/go-task/task/v3` (tool) | Taskfile runner for the Wails build | Make, just | Wails v3 builds are Taskfiles; `go tool task` needs no install | MIT | 2026-09-21 |
| `github.com/creachadair/jrpc2` v1.3.5 | JSON-RPC 2.0 over stdio to the WSL core (ADR-3) | `sourcegraph/jsonrpc2` (v0.x), `x/exp/jsonrpc2` (unstable) | the only semver-stable option; line framing for stdio; server notifications | BSD-3-Clause | 2026-09-21 |
| `golang.org/x/tools` (`go/packages`) | type-loads `internal/core` for `tools/gencore` (dev tool) | `go/loader` (deprecated), runtime reflection (Wails needs a plain struct) | the endorsed base for `go generate` tools | BSD-3-Clause | 2026-09-21 |
| `golang.org/x/sys` v0.48.0 | Windows message box for a missing prerequisite (QA-INST-01); `CREATE_NO_WINDOW` for child processes (TR-12) | hand-rolled `syscall.NewLazyDLL` | already an indirect Wails dependency | BSD-3-Clause | 2026-09-21 |
| `github.com/fsnotify/fsnotify` v1.10.1 | git-dir change notifications (FR-REPO-04) | polling only (more CPU for worse latency, QA-PERF-06) | the standard Go watcher over inotify, `ReadDirectoryChangesW` and kqueue; `internal/watch` adds debounce and the poll fallback | BSD-3-Clause | 2026-09-21 |
| `github.com/bluekeyes/go-gitdiff` v0.8.1 | parses `git diff` output into fragments and lines for line staging (FR-CHG-03) | none found that exposes per-line old/new positions | parsing-only API, which is all we need; the patch builder is our own | MIT | 2026-09-22 |
| `github.com/google/go-licenses/v2` v2.0.1 (tool) | lists linked Go packages and their licences for the licence check (QA-OSS-01) | `TCOTC/go-licenses/v2`, `uw-labs/lichen` | Google-maintained, pure Go, recent release; a dev tool that is only run | Apache-2.0 | 2026-09-23 |
| `vue` ^3.5 | UI framework (TR-03) | — | required | MIT | 2026-09-21 |
| `pinia` ^4.0.3 | state management (TR-03) | plain `ref` with provide/inject | the official Vue store; setup stores fit `<script setup>` | MIT | 2026-09-21 |
| `@vueuse/core` ^14.4 | composables: relative time, intervals, the preferred colour scheme | hand-rolled helpers | small, well tested, the standard Vue composable library | MIT | 2026-09-21 |
| `openvue` 1.0.0, `@openvue/themes` 1.0.0 (exact) | UI components (tree, splitter, dialogs, menus) and the Aura theme | a hand-rolled tree view; CSS `resize` for columns | community continuation of PrimeVue 4.5.5, the last MIT release, same API | MIT | 2026-09-22 |
| `@tanstack/vue-virtual` ^3.13 | virtual-scrolled commit log (QA-PERF-03) | VueUse `useVirtualList` | commit rows wrap; `useVirtualList` handles fixed heights only | MIT | 2026-09-21 |
| `@codemirror/merge` 6.12.2 (exact) | visual diff, unified and side-by-side (ADR-7) | a hand-rolled line-diff renderer | covers FR-DIFF-01/02 including chunk navigation | MIT | 2026-09-21 |
| `@codemirror/view` 6.43.11, `@codemirror/state` 6.7.4 (exact) | editor views and state built directly by `DiffView.vue` | — | were transitive deps of `@codemirror/merge`; direct because app code imports them | MIT | 2026-09-21 |
| `@codemirror/language` 6.12.4, `@codemirror/language-data` 6.5.2 (exact) | filename-based, lazily loaded syntax highlighting (FR-DIFF-05) | a hand-maintained extension table | the official CodeMirror language packages | MIT | 2026-09-23 |
| `@lezer/highlight` 1.2.3 (exact) | `classHighlighter`: plain token classes the app's theme colours | CodeMirror's `defaultHighlightStyle`, which injects its own colours | CodeMirror's own highlighting base | MIT | 2026-09-23 |
| `@wailsio/runtime` 3.0.0-beta.24 (exact) | JS side of the Wails bindings | — | must match the Go module version | MIT | 2026-09-21 |
| `vite` ^8, `@vitejs/plugin-vue` ^6 | frontend build (dev) | — | the toolchain of the Wails template and create-vue | MIT | 2026-09-21 |
| `typescript` ^5.9, `vue-tsc` ^3 | type checking, including SFCs (dev) | — | the only type checker for `.vue` files | Apache-2.0, MIT | 2026-09-21 |
| `@types/node` ^26 | types for Node-side config files (dev) | — | needed by the create-vue `tsconfig.node.json` split (TR-33) | MIT | 2026-09-22 |
| `vitest` ^5, `@vue/test-utils` ^2, `happy-dom` ^20 | component tests (dev, TR-31) | Jest with jsdom | Vite-native and faster; happy-dom is lighter than jsdom | MIT | 2026-09-21 |
| `@playwright/test` 1.59.1 (exact) | e2e tests for what component tests cannot prove (dev, TR-31) | — | the project's only e2e tool; pinned to match the Nix shell's browsers | Apache-2.0 | 2026-09-21 |
| `eslint` ^10, `eslint-plugin-vue` ^10, `@vue/eslint-config-typescript` ^14, `eslint-config-prettier` ^10, `prettier` ^3, `jiti` ^2 | lint and format (dev) | — | Vue's recommended flat-config stack; jiti loads `eslint.config.ts` | MIT | 2026-09-22 |
| `oxlint` ^1.83, `eslint-plugin-oxlint` ^1.83 | fast linting alongside ESLint (dev, TR-33) | — | part of the create-vue scaffold whenever ESLint is selected | MIT | 2026-09-22 |

JS packages are only accepted after they have been published for 7 days (`frontend/.npmrc`, `minimum-release-age`).

Licence check (`task licence-check`, part of `task ci`): every Go package linked into the two binaries and every JS production dependency must carry an allowed licence: MIT, Apache-2.0, MPL-2.0, or a BSD-family licence (BSD-2-Clause, BSD-3-Clause, ISC, 0BSD). JS dev tooling is only run, not shipped, and is exempt under TR-19. The check writes `THIRD-PARTY-NOTICES.txt` with each dependency's licence text.
