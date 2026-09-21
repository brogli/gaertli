# Components

### Backend (Go)

| Package | Job |
|---|---|
| `internal/core` | The core API: one interface, event stream, DTOs (TR-28). |
| `internal/remote` | Generated JSON-RPC client (implements `core.API`) and dispatcher (serves it in `nebegit-core --stdio`). WSL detection, starting a core through `wsl.exe`, deployment into a distro, version handshake, reconnect. |
| `internal/app` | Wails setup, window manager (one window per worktree, focus existing), single-instance hand-over, event bridge, settings. Binds the generated facade. The only package that imports Wails. |
| `internal/session` | Repo and worktree sessions (ADR-9): mints/reuses a session from a validated path, buffers the command log. Later specs add caches, write queues, watchers. |
| `internal/git` | Runner (exec, env, timeout, cancel, stderr capture, command log) and parsers (log, refs, status v2, worktree list, diff name-status, hunks). |
| `internal/graph` | Lane layout. Pure function over a commit stream. |
| `internal/jobs` | Long-running tasks: id, state, progress, cancel. Emits events. |
| `internal/watch` | Debounced "changed" signals with a scope (refs, index, worktrees, state). |
| `internal/settings` | Settings JSON load/save (TR-13): recent repos, windows open at last quit, and the FR-UX-04 fields. |
| `internal/editor` | Editor profiles (presets + custom); builds and starts the editor command. |
| `internal/services` | Implements `core.API`. Validates input, calls git, returns DTOs. One service per feature area. |
| `internal/buildinfo` | Version string stamped by `-ldflags -X` in release builds; `dev` otherwise. |

Services and the requirements they serve:

| Service | Covers |
|---|---|
| `RepoService` | FR-REPO-*: open, recent, summary (branch, ahead/behind, state). `OpenWorktree`/`Summary` delivered (spec 05); ahead/behind and refs are spec 11 |
| `LogService` | FR-LOG-*: load, row window, details, search, filters |
| `DiffService` | FR-CMP-*, FR-DIFF-*, FR-LOG-04, FR-CHG-04: file lists, file contents, hunks |
| `BranchService` | FR-BR-*, FR-UPD-*, FR-CP-*, FR-RV-*: create, reset, checkout, rename, delete, update in background, cherry-pick, revert, checkout commit |
| `WorktreeService` | FR-WT-*: list, add, remove, prune. "Open in app window" (FR-WT-08) is a shell action in `internal/app`; a core has no windows. |
| `ChangesService` | FR-CHG-*: status, stage/unstage (file, hunk, lines), commit, amend, discard |
| `SyncService` | FR-SYNC-*: fetch, pull, push, force push, auto-fetch |
| `ConflictService` | FR-CONF-*: state, mark resolved, continue, abort |
| `EditorService` | FR-ED-* |
| `JobService` | list jobs, cancel |
| `CommandLogService` | FR-UX-02. Delivered (spec 05) |

Settings (FR-UX-04) are a shell service (TR-13), not part of `core.API`.

### Frontend (Vue)

- Generated Wails bindings of the facade are the only way to reach Go. One Pinia store per area: `repo`, `commandLog`, `log`, `changes`, `worktrees`, `jobs`, `settings`.
- Stores hold view state only. After a write action the store reloads from Go. No optimistic updates (QA-COMP-01).

Main layout:

```
┌───────────────────────────────────────────────────────────────┐
│ Toolbar: repo · worktree (fixed) | branch ↑2 ↓1 | Fetch Pull Push │
├───────────┬───────────────────────────────────────────────────┤
│ Sidebar   │  Tabs: Log | Changes | Worktrees                  │
│ Branches  │  ┌─────────────────────────────┬────────────────┐ │
│ Remotes   │  │ commit list + graph         │ details /      │ │
│ Tags      │  │ (virtual scroll)            │ changed files  │ │
│ Worktrees │  ├─────────────────────────────┴────────────────┤ │
│           │  │ diff (side-by-side | unified)                │ │
├───────────┴──┴──────────────────────────────────────────────┤ │
│ Status bar: running jobs · last result · command log         │
└───────────────────────────────────────────────────────────────┘
```

### Communication

- Calls (UI → Go): generated bindings, returning a DTO or a typed error.
- Events (Go → UI): `repo:changed {scope}`, `job:update {id, state, progress, message}`, `log:progress {loaded, done}`.
- Cancel is a call: `JobService.Cancel(id)`.
- Events carry small payloads only. Large data (log rows, file contents) goes through bound method calls.

---
