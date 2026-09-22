# Components

### Backend (Go)

| Package | Job |
|---|---|
| `internal/core` | The core API: one interface, event stream, DTOs (TR-28). |
| `internal/remote` | Generated JSON-RPC client (implements `core.API`) and dispatcher (serves it in `nebegit-core --stdio`). WSL detection, starting a core through `wsl.exe`, deployment into a distro, version handshake, reconnect. Delivered, including the WSL-side launcher (`nebegit <path>`, FR-REPO-06) and the per-connect hook (`OnConnect`/`LiveClient`, TR-13). |
| `internal/app` | Wails setup, window manager (one window per worktree, focus existing), single-instance hand-over, event bridge, settings. Binds the generated facade. The only package that imports Wails. |
| `internal/session` | Repo and worktree sessions (ADR-9): mints/reuses a session from a validated path, buffers the command log. Later specs add caches, write queues, watchers. |
| `internal/git` | Runner (exec, env, timeout, cancel, stderr capture, command log) and parsers (log, refs, status v2, worktree list, diff name-status, hunks). |
| `internal/proc` | Child-process attributes (TR-12): `HiddenConsole()`, the `CREATE_NO_WINDOW` flag every Windows child (git, `wsl.exe`) starts with, nil elsewhere; `Detach(cmd)`, a new session/process group so an editor or the handed-off GUI outlives nebegit. |
| `internal/testbin` | Test support: `Build(out, pkg)` compiles a helper binary from a `TestMain` (`nebegit-core`, the editor's argv writer, the protocol-mismatch core). |
| `internal/graph` | Lane layout. Pure function over a commit stream. Delivered. |
| `internal/jobs` | Long-running tasks: id, state, progress, cancel. Emits events. Delivered. |
| `internal/watch` | Debounced "changed" signals with a scope (refs, index, worktrees, state): a repo-dir watcher (fsnotify + poll fallback) and a worktree status poller. Delivered. |
| `internal/settings` | Settings JSON load/save (TR-13): recent repos, windows open at last quit, per-environment editor profiles (FR-ED-11), and the FR-UX-04 fields. |
| `internal/editor` | Editor profiles (presets + custom); builds and starts the editor command. Delivered. |
| `internal/services` | Implements `core.API`. Validates input, calls git, returns DTOs. One service per feature area. |
| `internal/buildinfo` | Version string stamped by `-ldflags -X` in release builds; `dev` otherwise. |

Services and the requirements they serve:

| Service | Covers |
|---|---|
| `RepoService` | FR-REPO-*: open, recent, summary (branch, ahead/behind, state). `OpenWorktree`/`Summary` delivered; ahead/behind come from `LogService`'s `Refs`; `Refresh`/`SetFocused`/`CloseWorktree` (manual refresh, focus-driven poll, session teardown) delivered |
| `LogService` | FR-LOG-*: load, row window, details, search, filters. `LoadLog`/`LogRows`/`CommitDetails`, `ReloadLog` (reload only if ref tips changed), `Refs` (branches/remotes/tags for the sidebar), `LogIndexOf` (a ref's row index, for click-to-jump) and `SetLogFilter` (text/hash, author, branch, path search, all combinable — a window's own filtered view, independent of the shared unfiltered cache) delivered, as is the commit-row context-menu shell (FR-LOG-11: copy hash/subject only; the branch, compare, cherry-pick and worktree items belong to their own features) |
| `DiffService` | FR-CMP-*, FR-DIFF-*, FR-LOG-04, FR-CHG-04: file lists, file contents, hunks. `DiffFile` (one file's two-sided content at a commit, size cap, binary detection), `CompareCommits`/`DiffRange` (changed files and content between two arbitrary commits) and `DiffWorking` (a working-tree change's two-sided content: staged against the index, unstaged/untracked against the literal file on disk) delivered; changes-view hunks still open |
| `BranchService` | FR-BR-*, FR-UPD-*, FR-CP-*, FR-RV-*: create, reset, checkout, rename, delete, update in background, cherry-pick, revert, checkout commit. `CreateBranch` (a commit, an existing branch, or HEAD as the start point; optional atomic checkout), `Checkout` (local or remote-tracking, git's own DWIM), `CheckoutCommit` (detach HEAD), `Reset` (soft/mixed/hard, moves the current branch or a detached HEAD, returns the pre-reset HEAD), `UncommittedCount` (the hard-reset confirmation's count) and `CherryPick`/`Revert` (one or more commits onto the current worktree's HEAD, app-sorted oldest/newest-first, merge commits refused up front) delivered; rename/delete still open |
| `WorktreeService` | FR-WT-*: list, add, remove, prune. `ListWorktrees`/`ListBranches` (stateless reads, `session.Manager.Resolve`, no session minted) and `AddWorktree` (existing or new branch, `check-ref-format` validated first) delivered. "Open in app window" (FR-WT-08) is a shell action in `internal/app`; a core has no windows. FR-WT-04 ("switch to worktree" = open in the editor via `EditorService.OpenFolderAt`, a path, no session), FR-WT-06 ("create worktree here…" from a log commit or a sidebar branch, pre-filling `AddWorktree`) and FR-WT-12 (after a create, offer to open the result in a window, the editor, both, or neither) delivered. Remove/prune still open. |
| `ChangesService` | FR-CHG-*: status, stage/unstage (file, hunk, lines), commit, amend, discard. `Status` (FR-CHG-01, one `git status --porcelain=v2` call grouped into staged/unstaged/untracked/conflicted) and `StageFile`/`UnstageFile`/`StageAll`/`UnstageAll` (FR-CHG-02) delivered; hunk/line staging (FR-CHG-03), commit, amend, and discard still open |
| `SyncService` | FR-SYNC-*: fetch, pull, push, force push, auto-fetch. `Fetch`/`Pull`/`Push`/`ForcePush` (FR-SYNC-01, 02, 03, 05) delivered, each a job; rejection classification (FR-SYNC-04) and sign-in failure classification (FR-SYNC-08, QA-SEC-02) delivered with them. `SyncControls.vue` (Fetch/Pull/Push top-bar controls, force-push confirmation with its protected-branch warning FR-SYNC-06, the push-rejection banner) mounted in `WorktreeContent.vue`. Auto-fetch (FR-SYNC-10) still open |
| `ConflictService` | FR-CONF-*: state, mark resolved, continue, abort |
| `EditorService` | FR-ED-*: editor profiles and launch. `SetEditorProfile`/`OpenFolder`/`OpenFile` (FR-ED-01, 03, 04, 05, 06, 11), `OpenFolderAt` (a path, no session) and `OpenFile`'s path resolved against the worktree with `FILE_NOT_FOUND` for a since-deleted or renamed file (FR-ED-02: the commit-details, compare and compare-diff-window file lists and the diff pane's toolbar; the conflict list is still open) delivered; the presets picker, the "test" button, open-at-line/column from the diff view, and the git-editor/OS-opener fallback chain (FR-ED-07..10) still open |
| `JobService` | list jobs, cancel. Delivered; fetch/pull/push/force push (spec 71) are its first real job kinds. |
| `CommandLogService` | FR-UX-02. Delivered |

Settings (FR-UX-04) are a shell service (TR-13), not part of `core.API`.

### Frontend (Vue)

- Generated Wails bindings of the facade are the only way to reach Go. One Pinia store per area: `repo`, `commandLog`, `log`, `refs`, `diff`, `compare`, `changes`, `jobs`, `settings`. The selector panel's own recent-repo/worktree/branch state is component-local (`SelectorPanel.vue`), not a store — nothing else in the app reads it, and each window's panel is independent.
- Stores hold view state only. After a write action the store reloads from Go. No optimistic updates (QA-COMP-01).
- OpenVue's `ConfirmationService`/`ConfirmDialog`/`useConfirm` (registered once in `main.ts`/`App.vue`) back QA-SAFE-01/FR-UX-03's shared hard-action confirmation, wrapped in `composables/useConfirmAction.ts`, first used by `ResetDialog.vue`'s hard reset.

Main layout:

```
┌──────────────────────────────────────────────────────────────────────────┐
│ project name | branch ↑2 ↓1 | Refresh  Open in editor                    │
├───────────┬───────────┬────────────┬─────────────────┬───────────────────┤
│ Selector  │ Refs      │ Changes    │ Log             │ Commit details /  │
│ ▾ repo A  │ Branches  │ Staged     │ (graph + virtual│ compare, changed  │
│  ● wt main│ Remotes   │ Unstaged   │  scroll)        │ files             │
│  ○ wt feat│ Tags      │ Untracked  │                 ├───────────────────┤
│  + wt     │           │ Conflicted │                 │ Diff              │
│ ▸ repo B  │           │            │                 │ (unified |        │
│ ⚙ Settings│           │            │                 │  side-by-side)    │
├───────────┴───────────┴────────────┴─────────────────┴───────────────────┤
│ Status bar: running jobs · last result · command log                     │
└──────────────────────────────────────────────────────────────────────────┘
```

The left column is the selector panel (FR-WT-13, ADR-9 as revised 2026-09-22, delivered): `SelectorPanel.vue`, present in every window (not gated behind a selection, FR-UX-04), replacing the old dedicated home window — "Open folder…"/"Open WSL folder…"/the recent-repos list moved in unchanged, an OpenVue `Tree` below them lists each recent repo's worktrees (● = this window's own selection, ○ otherwise, `ListWorktrees`) and branches (`ListBranches`), lazily fetched on first expand; "+ New worktree" (`CreateWorktreeDialog.vue`, `AddWorktree`) and "New branch…"/per-branch "Checkout" (reusing `CreateBranchDialog.vue`/`Facade.Checkout`, scoped to the repo matching this window's own selection) sit under each repo node, and a "Settings" entry point (`SettingsPanel.vue`) needs no selection. Clicking a worktree row switches this window's own selection (`AppService.SelectWorktree`) in place; a small control opens it in a new window instead (`OpenRecent`, FR-WT-08's existing dedup). `MainView.vue` hosts the panel beside the selected worktree's own content (`WorktreeContent.vue`) or its empty state, a resizable/collapsible left column (CSS `resize: horizontal`, no splitter component — OpenVue's `Splitter` renders unstyled, so every resizable panel in this app uses this same plain pattern instead, spec 50/60); the panel's own "collapsed" toggle is local state, not persisted. Selecting a worktree switches the whole window, no new OS window.

FR-UX-07: no tabs or views — every panel is resizable and collapsible, never navigated to. The working-tree changes get their own `.changes-column` (`ChangesPanel.vue`, `stores/changes.ts`, delivered spec 60), between `RefsSidebar` and `.log-column` inside `WorktreeContent.vue`'s own flex row — left of the log, right of the selector/refs zone. Placed there rather than inside the details+diff column because this diagram already sketched it there; because FR-UX-08's three fixed relationships (refs leftmost, the log between two other things, the details+diff pairing one unbroken column immediately right of the log) stay literally true only to the log's left; and because selector, refs, and changes are all facts about the worktree's *current* state, kept apart from the log/details/diff cluster's own *history* facts. FR-UX-08: `WorktreeContent.vue` lays out a `.log-column` (filter bar, log) and a `.side-column` (commit details or compare view, then the diff) side by side; the details never sit below the log.

Sidebar's Branches/Remotes/Tags trees, the current-branch marker, click-to-jump into the log, and the toolbar's ahead/behind counts are delivered (`RefsSidebar.vue`, `stores/refs.ts`) — a different, richer view of the *selected* worktree's own refs (full upstream/ahead-behind detail) than the selector panel's lightweight per-repo branches list above.

The diff pane (`DiffView.vue`, `stores/diff.ts`) is delivered (FR-DIFF-01..04, FR-LOG-04): a changed file clicked in `CommitDetailsPanel` renders its visual diff below the commit list/details row, unified or side-by-side via `@codemirror/merge` (ADR-7), with next/previous-change and next/previous-file navigation.

Comparing two commits (`CompareView.vue`, `stores/compare.ts`) is delivered (FR-CMP-01..03): Ctrl/Cmd-click two rows in `LogList` selects them (FIFO of 2), replacing `CommitDetailsPanel` with `CompareView`'s from→to header, swap action, and changed-file list (`ChangedFileList.vue`, extracted for reuse between the two panels); a file clicked there opens in `DiffView` via its `range` prop (`DiffRange`, next/previous-file hidden), and Escape leaves compare mode. Double-clicking a file there, or right-clicking a commit and choosing "Compare with selected" (FR-LOG-11), opens the comparison's own OS window (FR-CMP-01/02, `CompareDiffView.vue`, `internal/app`'s window registry): the full changed-file list beside a `DiffView` with next/previous-file (`files` prop) added, keyed per `(wt, a, b)` comparison so a second double-click retargets the existing window (`"compare:show-file"`) instead of opening another.

The filter bar (`FilterBar.vue`, `stores/log.ts`) is delivered (FR-LOG-05..08): text/hash, author, branch, and path search sit directly above the commit list, debounced free-text fields, an OpenVue `MultiSelect` for branches, active-filter chips, one clear action (button or Escape). A text/author query hides the graph column (FR-LOG-02, `LogRowsResult.Flat`). Right-clicking a commit row opens an OpenVue `ContextMenu` (FR-LOG-11) with "copy hash"/"copy subject" (FR-LOG-12); later specs append their own items to `menus/commitMenu.ts`'s builder.

### Communication

- Calls (UI → Go): generated bindings, returning a DTO or a typed error.
- Events (Go → UI): `repo:changed {scope}`, `job:update {id, state, progress, message}`, `log:progress {loaded, done, filtered}` (`filtered` tells apart the repo-wide unfiltered stream from a window's own filtered one — one window can have both running at once), `core:reconnected` (no payload; a WSL window's distro core respawned after dying, ADR-4 — `repo`/`log`/`refs` stores each reload from scratch, not folded into `repo:changed`'s scoped reload).
- Cancel is a call: `JobService.Cancel(id)`.
- Events carry small payloads only. Large data (log rows, file contents) goes through bound method calls.

---
