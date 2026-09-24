# ADR-8: Reuse before build

Status: accepted

## Context

nebegit is built by a small team of sessions and must stay maintainable as open source (G7). Every line of own code is code to test and maintain.

## Decision

For every non-trivial problem we look for an established library or proven pattern first, and write our own code only when nothing fits (TR-17 to TR-19). Every added dependency is recorded in `dependencies.md` (TR-20).

Go:

| Need | Choice |
|---|---|
| Git access | system git via `os/exec` (ADR-1) |
| Logging | `log/slog` |
| Processes, cancel | `os/exec` + `context` |
| JSON-RPC over stdio | `creachadair/jrpc2` (ADR-3) |
| Facade, RPC client, dispatcher | own generator `tools/gencore` on `golang.org/x/tools/go/packages` |
| File watching | `fsnotify/fsnotify`; it has no recursive watch, so the git dir is watched and status is polled (`git-mapping.md`) |
| Parse unified diffs | `bluekeyes/go-gitdiff` (parsing only) |
| Partial patches for hunk and line staging | own code after lazygit's `pkg/commands/patch` (ADR-7) |
| Commit graph layout | own code in `internal/graph`, ported from lazygit's streaming model (ADR-5) |
| Split editor command templates | own splitter; `google/shlex` and `kballard/go-shellquote` are unmaintained |
| Config dir | `os.UserConfigDir` |
| Lint | `golangci-lint` v2 |

Preferred when the need arises: `golang.org/x/sync` (`errgroup`, `singleflight`) for parallel work, `google/go-cmp` for test assertions, `testing/synctest` for concurrent tests, `rogpeppe/go-internal/testscript` for script-style integration tests.

Frontend:

| Need | Choice |
|---|---|
| UI components | OpenVue 1.0, the continuation of PrimeVue 4.5 with the same API: context menu, dialog, confirm dialog, splitter, multiselect |
| Composables | VueUse |
| State | Pinia, setup stores; `createTestingPinia` in tests |
| Virtual list for the log | `@tanstack/vue-virtual`: commit rows wrap (`dependencies.md`) |
| Diff view | `@codemirror/merge` (ADR-7) |
| Dates | VueUse `formatTimeAgo` |
| Tests | Vitest, Vue Test Utils, happy-dom; generated bindings mocked with `vi.mock` |
| Lint and format | ESLint flat config with oxlint, `vue-tsc`, Prettier (TR-33) |

## Consequences

- Own code exists only where no library fits, each for a stated reason: the graph layout and `tools/gencore` (no maintained library), the partial-patch builder, the command splitter, and the job manager (small; `context` does most of the work).
- Code adapted from another project comes only from permissively licensed ones and ships their notice (`tools/licensecheck` adds it to `THIRD-PARTY-NOTICES.txt`, QA-OSS-05). The graph layout and the partial-patch builder are adapted from lazygit (MIT).
