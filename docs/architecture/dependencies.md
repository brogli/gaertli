# Dependency register (TR-20)

One row per dependency added to `go.mod` or `package.json`, written by the spec that adds it. Candidates are in ADR-8 and the plans dir `research/dependencies.md`.

| Dependency | Purpose | Alternatives looked at | Why chosen | Licence | Checked |
|---|---|---|---|---|---|
| `github.com/wailsapp/wails/v3` v3.0.0-beta.24 | desktop shell (ADR-2) | Wails v2, Electron-style shells | service model, TS bindings, multi-window; exact pin, beta accepted | MIT; transitive go-git Apache-2.0, x/* BSD-3, rest MIT/BSD | 2026-09-21 |
| `github.com/go-task/task/v3` (tool directive) | Taskfile runner used by the Wails build | Make, just | Wails v3 builds are Taskfiles; `go tool task` needs no install | MIT | 2026-09-21 |
| `vue` ^3.5 | UI framework (TR-03) | — | required | MIT | 2026-09-21 |
| `@wailsio/runtime` 3.0.0-beta.24 (exact) | JS side of the Wails bindings and Vite plugin | — | must match the Go module version | MIT | 2026-09-21 |
| `vite` ^8, `@vitejs/plugin-vue` ^6 | frontend build (dev tools) | — | the Wails template's toolchain | MIT | 2026-09-21 |
| `typescript` ^5.9, `vue-tsc` ^3 | type check incl. SFCs (dev tools) | — | only type checker for `.vue` | Apache-2.0, MIT | 2026-09-21 |
| `vitest` ^5, `@vue/test-utils` ^2, `happy-dom` ^20 | component tests (TR-31) | Jest + jsdom | Vite-native, faster; happy-dom lighter than jsdom | MIT | 2026-09-21 |
| `eslint` ^10, `eslint-plugin-vue` ^10, `@vue/eslint-config-typescript` ^14, `@vue/eslint-config-prettier` ^10, `prettier` ^3, `jiti` ^2 | lint and format (dev tools); jiti lets ESLint load `eslint.config.ts` | Oxlint (compat issues reported) | Vue's own recommended flat-config stack | MIT | 2026-09-21 |
| `github.com/creachadair/jrpc2` v1.3.5 | JSON-RPC 2.0 over stdio between shell and WSL core (ADR-3) | `sourcegraph/jsonrpc2` (v0.x), `x/exp/jsonrpc2` (unstable) | only semver-stable option; `channel.Line` framing for stdio; server notifications | BSD-3 | 2026-09-21 |
| `golang.org/x/tools` (`go/packages`) | `tools/gencore` type-loads `internal/core` (dev tool, TR-19 exempt) | `go/loader` (deprecated), reflection at runtime (Wails needs a plain struct) | endorsed base for `go generate` tools | BSD-3 | 2026-09-21 |
| `golang.org/x/sys` v0.48.0 | `windows.MessageBox` for the missing-prerequisite message (QA-INST-01); `CREATE_NO_WINDOW` for git subprocesses (TR-12) | hand-rolled `syscall.NewLazyDLL` (TR-17) | already an indirect Wails dependency; TR-19 already accepted (`research/dependencies.md`, TR-12 row); this spec is the first direct import | BSD-3 | 2026-09-21 |
| `pinia` ^4.0.3 | Vue state management for the `repo`, `commandLog`, `settings` stores (TR-19, TR-03: Composition API throughout) | plain `ref`/`provide`-`inject` | setup stores are the idiomatic fit for `<script setup>`; official Vue-team library | MIT | 2026-09-21 |
