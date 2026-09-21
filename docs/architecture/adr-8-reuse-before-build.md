# ADR-8: Reuse before build

Rules: TR-17 to TR-20. Added dependencies are recorded in `dependencies.md`.

Go:

| Need | Choice |
|---|---|
| Git access | system git via `os/exec` (ADR-1) |
| Logging | `log/slog` |
| Processes, cancel | `os/exec` + `context` |
| Parallel work | `golang.org/x/sync` (`errgroup`, `singleflight`) |
| JSON-RPC over stdio | `creachadair/jrpc2` (ADR-3) |
| Facade, RPC client, dispatcher | own `tools/gencore` on `golang.org/x/tools/go/packages`; no library exists |
| File watching | `fsnotify/fsnotify`; no recursive watch, so the git dir is watched and status is polled (`git-mapping.md`) |
| Parse unified diffs | `bluekeyes/go-gitdiff` (parsing only) |
| Partial patches for hunk/line staging | own code after lazygit's `pkg/commands/patch`: parse the real diff once, track selected line ranges, re-render a restricted patch, apply with `git apply --cached` |
| Split editor command templates | own splitter with table tests; `google/shlex` and `kballard/go-shellquote` are unmaintained |
| Config dir | `os.UserConfigDir` |
| Test asserts | `google/go-cmp`; `testing/synctest` for concurrent core tests |
| Script-style integration tests | `rogpeppe/go-internal/testscript` |
| Lint | `golangci-lint` v2 |

Frontend:

| Need | Choice |
|---|---|
| UI components | OpenVue 1.0 (continuation of PrimeVue 4.5, same API): tree, context menu, dialog, splitter, toast, data table, tabs, menubar |
| Composables | VueUse: `useTimeAgo`, `useMagicKeys`, `onKeyStroke`, `useStorage`, `useDebounceFn`, `useWindowFocus`, `useResizeObserver` |
| State | Pinia 4, setup stores; `createTestingPinia` in tests |
| Virtual list for the log | `@tanstack/vue-virtual` (dynamic row height). VueUse `useVirtualList` is fixed-height only. Confirmed by the 100k-row benchmark (M1). |
| Diff view | `@codemirror/merge` (ADR-7) |
| Commit graph layout | own code in `internal/graph`, ported from lazygit's streaming pipe model; no maintained library exists |
| Dates | `Intl.*`, VueUse `useTimeAgo` |
| Tests | Vitest, Vue Test Utils, happy-dom; `vi.mock` for generated bindings |
| Lint / format | ESLint flat config (`eslint-plugin-vue`, `@vue/eslint-config-typescript`), `vue-tsc`, Prettier separate |

Own code, with reason: graph layout, partial-patch builder, command splitter, `tools/gencore` (no library in each case); job manager (small; `context` + `errgroup` do most of it).

