# Components

## Backend (Go)

| Package | Responsibility |
|---|---|
| `internal/core` | The core contract: the `API` interface, its DTOs, the error type and events (ADR-3, TR-28). |
| `internal/services` | Implements `core.API` in one `Core` type, one file per feature area. Validates input, calls git, returns DTOs. |
| `internal/session` | Repo and worktree sessions (ADR-9): caches, write locks, watchers, the command log buffer, the commit message draft. |
| `internal/git` | The git runner (environment, timeout, cancel, stderr capture, command log) and the parsers for git's machine-readable output. |
| `internal/graph` | Commit graph lane layout. A pure function over the commit stream. |
| `internal/watch` | Debounced change signals: a git-dir watcher (fsnotify with a poll fallback) and a worktree status poller. |
| `internal/jobs` | Long-running tasks: id, state, progress, cancel. |
| `internal/editor` | Editor profiles and starting the editor command. |
| `internal/proc` | Child-process attributes: hidden console on Windows, detaching an editor so it outlives nebegit (TR-12). |
| `internal/remote` | The generated JSON-RPC client and dispatcher. WSL detection, starting and deploying a core in a distro, the version handshake, reconnect, and the WSL-side launcher. |
| `internal/app` | The shell: Wails setup, the generated facade, the window registry, single-instance hand-over, event routing, settings. The only package that imports Wails. |
| `internal/settings` | Loading and saving the settings file (TR-13). |
| `internal/buildinfo` | The version string, stamped at release build time. |

Test helpers: `internal/core/coretest` (assertions on core errors), `internal/git/gittest` (temp repos), `internal/testbin` (builds helper binaries for tests).

Binaries: `cmd/nebegit` is the GUI with an in-process core. `cmd/nebegit-core` is the headless core for WSL; it serves JSON-RPC on `--stdio`, and acts as the launcher when given a path. It builds without cgo.

Dependencies point one way: `app` → `services` and `remote` → `core`. `services` uses `session`, `git`, `graph`, `watch`, `jobs` and `editor`. Nothing below `app` knows about windows or Wails.

Feature areas in `internal/services`:

| Files | Requirements |
|---|---|
| `repo.go` | FR-REPO: open, summary, refresh, focus, close |
| `log.go`, `logfilter.go`, `refs.go`, `commitdetails.go` | FR-LOG: log rows, filters and search, refs, commit details |
| `diff.go`, `compare.go` | FR-DIFF, FR-CMP: file content for diffs, comparing commits and the working tree |
| `changes.go`, `hunks.go`, `discard.go` | FR-CHG: status, staging files and lines, commit, discard |
| `branch.go`, `cherrypick.go` | FR-BR, FR-CP, FR-RV: create, checkout, reset, rename, delete, cherry-pick, revert |
| `worktree.go` | FR-WT: list, add, remove, prune |
| `sync.go` | FR-SYNC, FR-UPD: fetch, pull, push, stash-and-pull, updating other branches |
| `conflicts.go` | FR-CONF: continue, abort, skip |
| `editor.go` | FR-ED: open folder and file, test a profile, fallbacks |
| `jobs.go`, `commandlog.go` | job list and cancel; FR-UX-02 |

Settings (FR-UX-04) belong to the shell, not to `core.API`. The shell pushes the relevant subset, such as a distro's editor profile, to each core on connect (TR-13). Opening a worktree in a new app window (FR-WT-08) is a shell action too, because a core has no windows.

## Frontend (Vue)

- The generated Wails bindings of the facade are the only way to reach Go.
- Each window is its own WebView with its own Pinia instance, so window state is isolated by construction.
- One setup store per area: `repo`, `log`, `refs`, `diff`, `compare`, `changes`, `jobs`, `commandLog`, `settings`.
- Stores hold view state only. After a write the store reloads from Go; there are no optimistic updates (QA-COMP-01).
- Hard actions share one confirmation (`composables/useConfirmAction.ts`, QA-SAFE-01). Core calls share one error handling path (`composables/useCoreCall.ts`, FR-UX-01).
- Context menus are built in `menus/`, one builder per object (commit, branch), so features add items without touching the list components.
- Theme (FR-UX-06): OpenVue's Aura preset. The `settings` store toggles a dark class on the document from the theme setting and the OS preference.

### Main layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│ project name | branch ↑2 ↓1 | Fetch Pull Push | Refresh  Open in editor  │
├───────────┬───────────┬────────────┬─────────────────┬───────────────────┤
│ Selector  │ Refs      │ Changes    │ Log             │ Commit details /  │
│ ▾ repo A  │ Branches  │ Staged     │ filter bar      │ compare, changed  │
│  ● wt main│ Remotes   │ Unstaged   │ (graph + virtual│ files             │
│  ○ wt feat│ Tags      │ Untracked  │  scroll)        ├───────────────────┤
│  + wt     │           │ Conflicted │                 │ Diff              │
│ ▸ repo B  │           │ Commit box │                 │ (unified |        │
│ ⚙ Settings│           │            │                 │  side-by-side)    │
├───────────┴───────────┴────────────┴─────────────────┴───────────────────┤
│ Status bar: running jobs · last result · command log                     │
└──────────────────────────────────────────────────────────────────────────┘
```

- **Selector panel** (`SelectorPanel.vue`, FR-WT-13): in every window. Open folder, open WSL folder, recent repos, and per repo its worktrees and branches, fetched when first expanded. Clicking a worktree switches this window's selection; a separate control opens it in a new window. With no selection it is the window's empty state.
- **Worktree content** (`WorktreeContent.vue`): everything to the right of the selector, for the selected worktree. Refs, changes, log and the details/diff column are panels of one splitter, in that fixed order (FR-UX-08). The details never sit below the log.
- No tabs and no navigation (FR-UX-07). Every panel is resizable and collapsible.
- Changes sit left of the log, next to the refs: selector, refs and changes describe the worktree's current state, while log, details and diff describe history.
- **Compare**: selecting two commits replaces the commit details with the comparison (FR-CMP). Double-clicking a file opens that comparison in its own window (`CompareDiffView.vue`), one window per comparison.
- **Diff** (`DiffView.vue`, ADR-7): unified or side-by-side, syntax highlighting, ignore-whitespace, next/previous change and file. For a modified working-tree file it offers hunk and line staging and discard.

## Communication

- **Calls** (UI → Go): generated bindings. Each returns a DTO or rejects with the core error (`errors-security.md`).
- **Events** (Go → UI), small payloads only:

| Event | Payload | Meaning |
|---|---|---|
| `repo:changed` | `{scope}` | something changed outside or inside the app; `scope` is `refs`, `index`, `worktrees` or `state` (`git-mapping.md`) |
| `log:progress` | `{loaded, done, filtered}` | more log rows are available; `filtered` tells a window's filtered load from the shared one |
| `job:update` | `{id, state, progress, message}` | a job started, progressed or ended |
| `command` | one command log entry | a git command ran (FR-UX-02) |
| `core:reconnected` | none | a WSL core was restarted; the window reloads from scratch |
| `compare:show-file` | `{path, oldPath}` | the shell retargets an open compare window |

- Cancel is a call: `CancelJob(id)`.
- Large data (log rows, file contents) always goes through calls, never events.
