# Components

## Backend (Go)

| Package | Responsibility |
|---|---|
| `internal/core` | The core contract: the `API` interface, its DTOs, the error type and events (ADR-3, TR-28). |
| `internal/services` | Implements `core.API` in one `Core` type, one file per feature area. Validates input, calls git, returns DTOs. |
| `internal/session` | Repo and worktree sessions (ADR-9): caches, write locks, watchers, the commit message draft. |
| `internal/git` | The git runner (environment, timeout, cancel, stderr capture, the app log's per-command record) and the parsers for git's machine-readable output. |
| `internal/graph` | Commit graph lane layout. A pure function over the commit stream. |
| `internal/watch` | Debounced change signals: a git-dir watcher (fsnotify with a poll fallback) and a worktree status poller. |
| `internal/jobs` | Long-running tasks: id, state, progress, cancel. |
| `internal/editor` | Editor profiles and starting the editor command. |
| `internal/proc` | Child-process attributes: hidden console on Windows, detaching an editor so it outlives nebegit (TR-12). |
| `internal/remote` | The generated JSON-RPC client and dispatcher. WSL detection, starting and deploying a core in a distro, the version handshake, reconnect, and the WSL-side launcher. |
| `internal/app` | The shell: Wails setup, the generated facade, the window registry, single-instance hand-over, event routing, settings. The only package that imports Wails. |
| `internal/settings` | Loading and saving the settings file (TR-13). |
| `internal/buildinfo` | The version string, stamped at release build time. |
| `internal/applog` | Opens the app's own log file: location, rotation at open, level (FR-UX-02). |

Test helpers: `internal/core/coretest` (assertions on core errors, draining events), `internal/git/gittest` (temp repos), `internal/testbin` (builds helper binaries for tests).

Binaries: `cmd/nebegit` is the GUI with an in-process core. `cmd/nebegit-core` is the headless core for WSL; it serves JSON-RPC on `--stdio`, and acts as the launcher when given a path. It builds without cgo.

Dependencies point one way: `app` → `services` and `remote` → `core`. `services` uses `session`, `git`, `graph`, `watch`, `jobs` and `editor`; `remote` also uses `git`, for its own credential masking (the one masking rule for a command's arguments and for stderr, git's or a WSL core's). Nothing below `app` knows about windows or Wails.

Feature areas in `internal/services`:

| Files | Requirements |
|---|---|
| `repo.go` | FR-REPO: open, summary, refresh, focus, close |
| `log.go`, `logfilter.go`, `refs.go`, `commitdetails.go` | FR-LOG: log rows, filters and search, refs, a commit's details |
| `diff.go`, `compare.go` | FR-DIFF, FR-CMP: file content for diffs, comparing commits and the working tree |
| `changes.go`, `hunks.go`, `discard.go` | FR-CHG: status, staging files and lines, commit, discard |
| `branch.go`, `cherrypick.go` | FR-BR, FR-CP, FR-RV: create, checkout, reset, rename, delete, cherry-pick, revert |
| `worktree.go` | FR-WT: list, add, remove, prune |
| `sync.go` | FR-SYNC, FR-UPD: fetch, pull, push, stash-and-pull, updating other branches |
| `conflicts.go` | FR-CONF: continue, abort, skip |
| `editor.go` | FR-ED: open folder and file |
| `jobs.go` | job list and cancel |

Settings (FR-UX-04) belong to the shell, not to `core.API`. The shell pushes the relevant subset, such as a distro's editor profile, to each core on connect (TR-13). Opening a worktree in a new app window (FR-WT-08) is a shell action too, because a core has no windows.

The shell owns the one app log file (FR-UX-02): `internal/applog` opens it and picks its level. Every Go package, Wails itself and the frontend's uncaught errors (through Wails' log service) write to it; git commands reach it through the `command` event (below).

## Frontend (Vue)

- The generated Wails bindings are the only way to reach Go.
- Each window is its own WebView with its own Pinia instance, so window state is isolated by construction.
- One setup store per area: `repo`, `log`, `refs`, `diff`, `compare`, `changes`, `jobs`, `settings`.
- Stores hold view state only. After a write the store reloads from Go; there are no optimistic updates (QA-COMP-01).
- Hard actions share one confirmation (`composables/useConfirmAction.ts`, QA-SAFE-01). Core calls share one error handling path (`composables/useCoreCall.ts`, FR-UX-01).
- The log view's context menu is built in `menus/`, so features add items without touching the list component.
- Theme (FR-UX-06): OpenVue's Aura preset. The `settings` store toggles a dark class on the document from the theme setting and the OS preference.
- Icons: OpenIcons through `lib/icons.ts`.
- Texts (FR-UX-11, TR-35): vue-i18n, Composition API, one English catalogue (`src/locales/en.json`) with keys grouped by glossary area (`toolbar.fetch`, `changesView.commitBox.amend`); `errors.*` holds one entry per message key Go can produce. Components use the app's typed `useI18n` (`src/i18n.ts`); stores, `lib/` and `menus/` take `t` as an argument or read the global instance, and hold no English text. A Vitest guard (`src/i18n.test.ts`) fails on untranslated template text or attributes and on keys missing from the catalogue.
- Notices: OpenVue `Toast`, one notice area (`NoticeArea.vue`) per main window, bottom-right so it never covers the toolbar; the diff window has none. A notice can carry an error (`ErrorBanner`, with git's command and stderr behind its details toggle), a Copy button, or recovery buttons: a push rejection offers Pull and Force push, a blocked pull Stash and pull. Sync jobs and branch updates (`useJobNotices`), resets and editor start failures report through notices. One notice per kind (`useNotices`): a newer one replaces only its own kind's; a failure or offer stays until closed, a success closes itself.

### Main layout

The window body is three views — the log view, the commit detail view and the changes view — laid out by the window's width (`lib/layout.ts`, `useLayout`, VueUse `useBreakpoints`): wide at 1400px and above, medium from 900px, narrow below. The Wails window's own minimum width is 400px.

Wide, an OpenVue `Splitter` with all three side by side (log view 50%, commit detail view 25%, changes view 25%, remembered per layout):

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [repo ▾] [worktree ▾] [⎇ branch ↑2 ↓1 ▾] Fetch Pull Push  Jobs Refresh ✎ ⚙│
├───────────────────────────┬──────────────────────┬───────────────────────┤
│ Log view                  │ Commit detail view   │ Changes view          │
│ filter bar                │ (selected commit, or │ change list: staged,  │
│ graph + virtual scroll    │ the comparison)       │ unstaged, untracked,  │
│                           │                      │ conflicted (scrolls)  │
│                           │                      ├───────────────────────┤
│                           │                      │ Commit box (fixed)    │
└───────────────────────────┴──────────────────────┴───────────────────────┘
```

Medium, a `Splitter` with the log view (60%) and a side panel (40%): an OpenVue `SelectButton` [Details, Changes (n)] on top, the chosen view below. Narrow, down to the 400px minimum, a `SelectButton` [Log, Details, Changes (n)] on top and the chosen view below at full width; the toolbar's buttons show icon only (text moves into `title`/`aria-label`) and its three pickers drop their label and dropdown chevron the same way, down to their icon alone. "(n)" is the changes view's file count (staged + unstaged + untracked + conflicted), absent at 0. Every view stays mounted while hidden (`v-show`), so switching keeps scroll position and the draft commit message.

- **Toolbar** (`WorktreeContent.vue`): every action button is an OpenVue `Button` (Pull a `SplitButton`) with one shared look (`useToolbarButton`); below the medium width they also shrink to the small size. A conflict in progress and a pull's stash waiting are states, not job endings, so they stay as banners below the toolbar (`ConflictBanner.vue`, `StashBanner.vue`); everything else an action ends with is a notice.
- **Jobs menu** (`JobsMenu.vue`, FR-UX-12): a fixed-size toolbar button, muted when idle, spinning with a count badge while jobs run, so starting or ending a job never resizes the toolbar. Its popover lists each running job with git's progress message, a progress bar and an abort button (`CancelJob`).
- **Pickers** (`RepoPicker.vue`, `WorktreePicker.vue`, `BranchPicker.vue`; FR-WT-13, FR-BR-09): one button style and one list component (`PickerList.vue`): a filter field, keyboard navigation, and row buttons that never activate the row. The repo picker names the repo by its main worktree; no repo in its list starts a core until chosen, so no WSL core starts uninvited. If the worktree it would switch to is gone, it retries at the repo's main worktree before showing the error. In the branch picker, checked local branches update together (FR-UPD-06).
- **Settings** (`SettingsPanel.vue`, FR-UX-04): the worktree content stays mounted behind the dialog; Save closes it.
- **Log view** (`LogView.vue`): the filter bar (`FilterBar.vue`) above the commit list (`LogList.vue`, the graph and virtual scroll). A click selects a commit; Enter or a double-click opens it (switches to Details in medium and narrow).
- **Commit detail view** (`CommitDetailView.vue`): the selected commit's fields (FR-UX-10) above its changed files, or in their place the comparison (`CompareView.vue`, FR-CMP) or the working-tree comparison (`CompareWorkingView.vue`) when one is active. The older/newer colour of a compared commit (FR-CMP-05) goes by log row index.
- **Changes view** (`ChangesView.vue`, FR-CHG-11, FR-CHG-12): a flex column; the change list (a header with Stage all/Unstage all, then the groups) scrolls in its own region above the commit box (`CommitBox.vue`), whose message field is 6 rows tall.
- **Diff window** (`DiffWindowView.vue`, FR-DIFF-07): the target's files on the left, the working tree's in their groups, and `DiffView` on the right. In every file list of the main window a click selects a row and a double-click or Enter opens it; another file retargets the open diff window (`diff:show`). A working-tree list and diff reload on `repo:changed` with scope `index`.
- **Diff** (`DiffView.vue`, ADR-7): git's diff drawn by CodeMirror; `lib/gitHunks.ts` maps git's hunks onto it. Hunk and line actions appear for a modified working-tree file with whitespace shown. Its toolbar stays in view while the diff scrolls.

## Communication

- **Calls** (UI → Go): generated bindings. Each returns a DTO or rejects with the core error (`errors-security.md`).
- **Events** (Go → UI), small payloads only:

| Event | Payload | Meaning |
|---|---|---|
| `repo:changed` | `core.RepoChanged` | something changed outside or inside the app; `scope` is `refs`, `index`, `worktrees` or `state` (`git-mapping.md`) |
| `log:progress` | `core.LogProgress` | more log rows are available; `filtered` tells a window's filtered load from the shared one |
| `job:update` | `core.JobInfo` | a job started, progressed or ended |
| `command` | `core.CommandRecord` | a git command ran; shell-only, written to the app log, never forwarded to a window (FR-UX-02) |
| `core:reconnected` | none | a WSL core was restarted; the window reloads from scratch |
| `settings:changed` | none | settings were saved; every window reloads its settings and summary, so the theme and the "Open in …" label follow |
| `diff:show` | `app.DiffShow` | the shell retargets an open diff window to another file |

Each event is registered in the shell with its Go payload type; the binding generator types the frontend's handlers from that registration, so no event type is written by hand in TypeScript. A core's events carry JSON, so they survive the RPC hop from a WSL core; the shell decodes each into its registered type before emitting it and drops, with a log line, one that does not decode.

Wails delivers every event to every window and stamps it with the window it was emitted through as its sender. The shell emits each event through each window it concerns (ADR-9), and every frontend listener goes through one helper (`lib/windowEvents.ts`) that drops events whose sender is not its own window. Only `settings:changed`, which is for every window, skips it. A WSL core's events reach only windows on that distro, with the id rewritten to the shell's UNC form (ADR-3).

- Cancel is a call: `CancelJob(id)`.
- Large data (log rows, file contents) always goes through calls, never events.
