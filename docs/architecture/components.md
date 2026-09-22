# Components

### Backend (Go)

| Package | Job |
|---|---|
| `internal/core` | The core API: one interface, event stream, DTOs (TR-28). |
| `internal/remote` | Generated JSON-RPC client (implements `core.API`) and dispatcher (serves it in `nebegit-core --stdio`). WSL detection, starting a core through `wsl.exe`, deployment into a distro, version handshake, reconnect. Delivered (spec 30). The WSL-side launcher (`nebegit <path>`, FR-REPO-06) and the per-connect hook (`OnConnect`/`LiveClient`, TR-13) delivered (spec 31). |
| `internal/app` | Wails setup, window manager (one window per worktree, focus existing), single-instance hand-over, event bridge, settings. Binds the generated facade. The only package that imports Wails. |
| `internal/session` | Repo and worktree sessions (ADR-9): mints/reuses a session from a validated path, buffers the command log. Later specs add caches, write queues, watchers. |
| `internal/git` | Runner (exec, env, timeout, cancel, stderr capture, command log) and parsers (log, refs, status v2, worktree list, diff name-status, hunks). |
| `internal/proc` | Child-process attributes (TR-12): `HiddenConsole()`, the `CREATE_NO_WINDOW` flag every Windows child (git, `wsl.exe`) starts with, nil elsewhere; `Detach(cmd)`, a new session/process group so an editor or the handed-off GUI outlives nebegit. |
| `internal/testbin` | Test support: `Build(out, pkg)` compiles a helper binary from a `TestMain` (`nebegit-core`, the editor's argv writer, the protocol-mismatch core). |
| `internal/graph` | Lane layout. Pure function over a commit stream. Delivered (spec 10). |
| `internal/jobs` | Long-running tasks: id, state, progress, cancel. Emits events. |
| `internal/watch` | Debounced "changed" signals with a scope (refs, index, worktrees, state): a repo-dir watcher (fsnotify + poll fallback) and a worktree status poller. Delivered (spec 12). |
| `internal/settings` | Settings JSON load/save (TR-13): recent repos, windows open at last quit, per-environment editor profiles (FR-ED-11, spec 31), and the FR-UX-04 fields. |
| `internal/editor` | Editor profiles (presets + custom); builds and starts the editor command. Delivered (spec 31). |
| `internal/services` | Implements `core.API`. Validates input, calls git, returns DTOs. One service per feature area. |
| `internal/buildinfo` | Version string stamped by `-ldflags -X` in release builds; `dev` otherwise. |

Services and the requirements they serve:

| Service | Covers |
|---|---|
| `RepoService` | FR-REPO-*: open, recent, summary (branch, ahead/behind, state). `OpenWorktree`/`Summary` delivered (spec 05); ahead/behind come from `Refs` (spec 11, `LogService`); `Refresh`/`SetFocused`/`CloseWorktree` (manual refresh, focus-driven poll, session teardown) delivered (spec 12) |
| `LogService` | FR-LOG-*: load, row window, details, search, filters. `LoadLog`/`LogRows`/`CommitDetails` delivered (spec 10); `ReloadLog` (reload only if ref tips changed) delivered (spec 12); `Refs` (branches/remotes/tags for the sidebar) and `LogIndexOf` (a ref's row index, for click-to-jump) delivered (spec 11); `SetLogFilter` (text/hash, author, branch, path search, all combinable — a window's own filtered view, independent of the shared unfiltered cache) and the commit-row context-menu shell (FR-LOG-11: copy hash/subject only; branch/compare/cherry-pick/worktree items are 40/41/21/42/50's own) delivered (spec 22) |
| `DiffService` | FR-CMP-*, FR-DIFF-*, FR-LOG-04, FR-CHG-04: file lists, file contents, hunks. `DiffFile` (one file's two-sided content at a commit, size cap, binary detection) delivered (spec 20); `CompareCommits`/`DiffRange` (changed files and content between two arbitrary commits) delivered (spec 21); changes-view hunks are a later spec |
| `BranchService` | FR-BR-*, FR-UPD-*, FR-CP-*, FR-RV-*: create, reset, checkout, rename, delete, update in background, cherry-pick, revert, checkout commit. `CreateBranch` (a commit, an existing branch, or HEAD as the start point; optional atomic checkout), `Checkout` (local or remote-tracking, git's own DWIM), `CheckoutCommit` (detach HEAD) delivered (spec 40); `Reset` (soft/mixed/hard, moves the current branch or a detached HEAD, returns the pre-reset HEAD) and `UncommittedCount` (the hard-reset confirmation's count) delivered (spec 41); `CherryPick`/`Revert` (one or more commits onto the current worktree's HEAD, app-sorted oldest/newest-first, merge commits refused up front) delivered (spec 42); rename/delete (91) still open |
| `WorktreeService` | FR-WT-*: list, add, remove, prune. "Open in app window" (FR-WT-08) is a shell action in `internal/app`; a core has no windows. |
| `ChangesService` | FR-CHG-*: status, stage/unstage (file, hunk, lines), commit, amend, discard |
| `SyncService` | FR-SYNC-*: fetch, pull, push, force push, auto-fetch |
| `ConflictService` | FR-CONF-*: state, mark resolved, continue, abort |
| `EditorService` | FR-ED-*: editor profiles and launch. `SetEditorProfile`/`OpenFolder`/`OpenFile` (FR-ED-01, 03, 04, 05, 06, 11) delivered (spec 31); FR-ED-02 (open from the changes/diff/conflict list) is spec 51; the presets picker, "test" button, open-at-line/column from the diff view, and the git-editor/OS-opener fallback chain (FR-ED-07..10) are spec 93 |
| `JobService` | list jobs, cancel |
| `CommandLogService` | FR-UX-02. Delivered (spec 05) |

Settings (FR-UX-04) are a shell service (TR-13), not part of `core.API`.

### Frontend (Vue)

- Generated Wails bindings of the facade are the only way to reach Go. One Pinia store per area: `repo`, `commandLog`, `log`, `refs`, `diff`, `compare`, `changes`, `worktrees`, `jobs`, `settings`.
- Stores hold view state only. After a write action the store reloads from Go. No optimistic updates (QA-COMP-01).
- OpenVue's `ConfirmationService`/`ConfirmDialog`/`useConfirm` (registered once in `main.ts`/`App.vue`) back QA-SAFE-01/FR-UX-03's shared hard-action confirmation, wrapped in `composables/useConfirmAction.ts` (spec 41, first used by `ResetDialog.vue`'s hard reset; reused unchanged by specs 71/72/91/92).

Main layout:

```
┌────────────────────────────────────────────────────────────────┐
│ Toolbar: selected worktree | branch ↑2 ↓1 | Fetch Pull Push       │
├──────────────┬─────────────────────────────────────────────────┤
│ Selector     │  Changes (working tree)   │ Log                 │
│ ▾ repo A     │  ┌─────────────────────────────┬──────────────┐ │
│   ● wt main  │  │ commit list + graph         │ details /    │ │
│   ○ wt feat  │  │ (virtual scroll)            │ changed files│ │
│   + worktree │  ├─────────────────────────────┴──────────────┤ │
│ ▸ repo B     │  │ diff (side-by-side | unified)              │ │
│ ⚙ Settings   │  │                                            │ │
│ ─────────    │  │                                            │ │
│ Branches     │  │                                            │ │
│ Remotes      │  │                                            │ │
│ Tags         │  │                                            │ │
├──────────────┴─┴────────────────────────────────────────────┤ │
│ Status bar: running jobs · last result · command log          │
└────────────────────────────────────────────────────────────────┘
```

The left column is the selector panel (FR-WT-13, ADR-9 as revised 2026-09-22): recent repos, each with its worktrees (● = shown in this window), new-worktree and branch actions, Settings; below it the refs of the selected worktree. Selecting a worktree switches the whole window. Spec 50 delivers the panel; until then the home window (spec 04/05) and the per-window refs sidebar stand in. FR-UX-07: no tabs or views — the working-tree changes (spec 60) get their own panel beside the log, placement decided there; every panel is resizable and collapsible, never navigated to. FR-UX-08: `WorktreeView.vue` lays out a `.log-column` (filter bar, log) and a `.side-column` (commit details or compare view, then the diff) side by side; the details never sit below the log.

Sidebar's Branches/Remotes/Tags trees, the current-branch marker, click-to-jump into the log, and the toolbar's ahead/behind counts are delivered (spec 11, `RefsSidebar.vue`, `stores/refs.ts`). The sidebar's Worktrees section is spec 50 (FR-WT-01).

The diff pane (`DiffView.vue`, `stores/diff.ts`) is delivered (spec 20, FR-DIFF-01..04, FR-LOG-04): a changed file clicked in `CommitDetailsPanel` renders its visual diff below the commit list/details row, unified or side-by-side via `@codemirror/merge` (ADR-7), with next/previous-change and next/previous-file navigation.

Comparing two commits (`CompareView.vue`, `stores/compare.ts`) is delivered (spec 21, FR-CMP-01..03): Ctrl/Cmd-click two rows in `LogList` selects them (FIFO of 2), replacing `CommitDetailsPanel` with `CompareView`'s from→to header, swap action, and changed-file list (`ChangedFileList.vue`, extracted for reuse between the two panels); a file clicked there opens in `DiffView` via its `range` prop (`DiffRange`, next/previous-file hidden), and Escape leaves compare mode. Double-clicking a file there, or right-clicking a commit and choosing "Compare with selected" (FR-LOG-11), opens the comparison's own OS window (spec 23, FR-CMP-01/02, `CompareDiffView.vue`, `internal/app`'s window registry): the full changed-file list beside a `DiffView` with next/previous-file (`files` prop) added, keyed per `(wt, a, b)` comparison so a second double-click retargets the existing window (`"compare:show-file"`) instead of opening another.

The filter bar (`FilterBar.vue`, `stores/log.ts`) is delivered (spec 22, FR-LOG-05..08): text/hash, author, branch, and path search sit directly above the commit list, debounced free-text fields, an OpenVue `MultiSelect` for branches, active-filter chips, one clear action (button or Escape). A text/author query hides the graph column (FR-LOG-02, `LogRowsResult.Flat`). Right-clicking a commit row opens an OpenVue `ContextMenu` (FR-LOG-11) with "copy hash"/"copy subject" (FR-LOG-12); later specs append their own items to `menus/commitMenu.ts`'s builder.

### Communication

- Calls (UI → Go): generated bindings, returning a DTO or a typed error.
- Events (Go → UI): `repo:changed {scope}`, `job:update {id, state, progress, message}`, `log:progress {loaded, done, filtered}` (`filtered` tells apart the repo-wide unfiltered stream from a window's own filtered one, spec 22 — one window can have both running at once), `core:reconnected` (no payload; a WSL window's distro core respawned after dying, spec 30 ADR-4 — `repo`/`log`/`refs` stores each reload from scratch, not folded into `repo:changed`'s scoped reload).
- Cancel is a call: `JobService.Cancel(id)`.
- Events carry small payloads only. Large data (log rows, file contents) goes through bound method calls.

---
