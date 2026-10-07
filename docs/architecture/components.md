# Components

## Backend (Go)

| Package              | Responsibility                                                                                                                                                                          |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/core`      | The core contract: the `API` interface, its DTOs, the error type and the events (ADR-3, TR-28).                                                                                         |
| `internal/services`  | Implements `core.API`, one feature area at a time: validates input, calls git, returns DTOs.                                                                                            |
| `internal/session`   | Repo and worktree sessions (ADR-9): caches, write locks, watchers, the commit message draft.                                                                                            |
| `internal/git`       | The git runner (environment, timeout, cancel, stderr capture, each command's record for the app log) and the parsers for git's machine-readable output.                                 |
| `internal/graph`     | Commit graph lane layout, a pure function over the commit stream (ADR-5).                                                                                                               |
| `internal/watch`     | Debounced change signals: a git-dir watcher (fsnotify with a poll fallback) and a worktree status poller.                                                                               |
| `internal/jobs`      | Long-running tasks: id, state, progress, cancel.                                                                                                                                        |
| `internal/editor`    | Editor profiles and starting the editor command.                                                                                                                                        |
| `internal/proc`      | Child-process attributes (TR-12): no console on Windows, a process group of its own for git so a cancel can interrupt it, a detached editor that outlives nebegit.                      |
| `internal/remote`    | The WSL side of ADR-3: the generated JSON-RPC client and dispatcher, WSL detection, deploying and starting a core in a distro, the version handshake, reconnect.                        |
| `internal/app`       | The shell: Wails setup, the generated facade, the window registry, single-instance hand-over, event routing, settings. The only package that imports Wails.                             |
| `internal/userpath`  | Adds or removes one directory in the Windows user PATH, keeping every other entry, its order and its `%VAR%` text; the installer and its uninstaller call it through the app's own exe. |
| `internal/settings`  | Loading and saving the settings file (TR-13).                                                                                                                                           |
| `internal/buildinfo` | The version string, stamped at release build time.                                                                                                                                      |
| `internal/applog`    | The app log file: its location, rotation at open, and its level, which debug mode switches (FR-UX-02, FR-UX-19).                                                                        |

Test helpers: `internal/core/coretest` (core errors, events), `internal/git/gittest` (temp repos),
`internal/testbin` (helper binaries), `internal/remote/remotetest` (a core served over an in-memory
pipe).

Binaries: `cmd/nebegit` is the GUI with an in-process core. `cmd/nebegit-core` is the headless core
for WSL: it serves JSON-RPC on stdio. It builds without cgo.

Dependencies point one way: `app` → `services` and `remote` → `core`. `services` uses `session`,
`git`, `graph`, `watch`, `jobs` and `editor`. `remote` uses `git` too, so one rule masks credentials
in a command's arguments and in stderr, git's or a WSL core's. Nothing below `app` knows about
windows or Wails.

Settings (FR-UX-04) belong to the shell, not to `core.API`; each core gets its subset on connect,
such as its distro's editor profiles (TR-13). Opening a worktree in a new window (FR-WT-08) is a
shell action too, because a core has no windows.

The shell owns the one app log (FR-UX-02). Every Go package, Wails and the frontend's uncaught
errors write to it. Git commands and editor starts reach it as the `command` and `editor` events
(below), so a WSL core's commands land in the same log, and debug mode takes effect at once for
local and WSL repos alike. The default level is Info: the app's errors, a failed git command or
editor start (Warn), a WSL core's stderr (Warn). A successful git command or editor start is Debug.
`NEBEGIT_LOG=debug` forces Debug for one run, for problems before settings load.

## Frontend (Vue)

- The generated Wails bindings are the only way to reach Go.
- Each window is its own WebView with its own Pinia instance, so window state is isolated by
  construction.
- One setup store per area: `repo`, `log`, `refs`, `diff`, `compare`, `changes`, `jobs`, `settings`.
- Stores hold view state only. After a write the store reloads from Go; there are no optimistic
  updates (QA-COMP-01).
- Hard actions share one confirmation path (QA-SAFE-01); core calls share one error path (FR-UX-01).
- Context menus are built apart from the lists that show them, so a feature adds menu items without
  touching the list.
- Theme (FR-UX-06, TR-37, TR-43): Aura, light or dark by the theme setting or the OS preference,
  through one project preset, the only place OpenVue's look may change (design tokens). The views
  style with Tailwind utilities, which read OpenVue's tokens; OpenVue's styles sit in their own CSS
  layer between Tailwind's base and its utilities. The app's styles draw only what OpenVue has no
  component for: the graph, file status tones, the comparison marks, the file and picker rows.
- Fonts (FR-UX-18): Inter for text, JetBrains Mono for hashes, paths and diffs, both bundled from
  Fontsource. OpenVue's components inherit the text font; the views and the diff view share one
  fixed-width font setting.
- Texts (FR-UX-11, TR-35): one English catalogue, keys grouped by glossary area, with one `errors.*`
  entry per message key Go can produce. Components, stores and helpers hold no English text.
  OpenVue's built-in texts come from the same catalogue through OpenVue's locale option.
- Notices (TR-36, FR-UX-15): one notice area per window, main and diff alike. It is OpenVue's Toast;
  its message adds the notice's extras: an error's details (git's command and stderr), a Copy
  button, and recovery buttons (a push rejection offers Pull and Force push when the branch has an
  upstream, a blocked pull Stash and pull, a held lock Try again, FR-UX-16). Every notice has a
  kind, and a newer notice replaces only its own kind's. Closing after a while and pausing under the
  pointer are the Toast's own behaviour.
- Dialogs (FR-UX-17): a modal dialog of one width, its body a column of rows: a label above its
  control, a checkbox beside its label, the buttons (Cancel, then the action) in the footer.
  - A new worktree's folder name and location stay in step (FR-WT-02). The base folder is the
    settings entry for the repo's environment, so a WSL repo gets a folder inside its distro.
  - A new branch's start point (FR-BR-01) is a filterable drop-down over the window's refs, grouped
    like the log's branch filter and sent by its full ref name (`git-mapping.md`). From the log
    view, or while detached, it is a fixed commit.
  - Reset (FR-BR-02, FR-BR-07) names the branch and the target commit and lists the four modes one
    below the other, each with its one-line explanation. On success it closes, and the notice naming
    the previous commit is the only result (FR-BR-04).
  - A local branch checked out in another worktree, or being rebased or bisected there (FR-BR-05),
    gets a dialog naming that worktree and the operation when there is one, with "Switch to
    worktree" and Cancel.
- An error shown in place is an inline error message with the same headline, Try again and details
  as an error notice.

### Main layout

The window body is three views: the log view, the commit detail view and the changes view, laid out
by the window's width: wide at 1400px and above, medium from 900px, narrow below, down to the
window's 400px minimum.

Wide, all three sit side by side in resizable panes (log view 50%, commit detail view 25%, changes
view 25%, remembered per layout):

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [repo ▾] [worktree ▾] [⎇ branch ↑2 ↓1 ▾] Fetch Pull Push  Jobs Refresh ✎ ⚙│
├───────────────────────────┬──────────────────────┬───────────────────────┤
│ Log view                  │ Commit detail view   │ Changes view          │
│ filter bar                │ (selected commit, or │ change list: staged,  │
│ graph + virtual scroll    │ the comparison)      │ unstaged, untracked,  │
│                           │                      │ conflicted (scrolls)  │
│                           │                      ├───────────────────────┤
│                           │                      │ Commit box (fixed)    │
└───────────────────────────┴──────────────────────┴───────────────────────┘
```

Medium, resizable panes hold the log view (60%) and a side panel (40%): a tab row [Details, Changes
(n)] on top, the chosen view below. Narrow, a tab row [Log, Details, Changes (n)] on top and the
chosen view below at full width. "(n)" is the changes view's file count (FR-UX-08), absent at 0.
Hidden views stay mounted, so switching keeps the scroll position and the typed commit message. The
arrow keys move between tabs and pick as they go.

- **Toolbar**: every action is a button with one shared look, Pull a split button whose menu picks
  the pull mode (Settings' default pull mode) without pulling. Below wide the buttons show their
  icon only, below medium they are the small size, and in narrow the three pickers show only their
  icon too. A conflict in progress and a pull's stash waiting are states, not results, so they show
  as banners below the toolbar (warn and info message boxes); every other result is a notice.
- **Jobs menu** (FR-UX-12): a fixed-size toolbar button, a clock when idle, spinning with a count
  badge while jobs run, so the toolbar never resizes. Its popover lists each running job with git's
  progress message, a progress bar and an abort button.
- **Keyboard** (FR-UX-05): one shortcut guard wraps every window-level shortcut. It acts only while
  no dialog, popup or menu is open, and leaves Escape to a focused text field. Each shortcut shows
  on its control in muted text: at the right inside the Commit button and beside a menu entry, after
  the action's name in a tooltip ("Push · Ctrl+Shift+K").
- **Pickers** (FR-WT-13, FR-BR-09): one button style and one list: a filter field, keyboard
  navigation, and row buttons that never activate the row. The list owns each row's menu: a
  right-click opens it at the pointer; Shift+F10, the context-menu key, or the row's "…" button
  (where the picker asks for one) open it below the row or the button.
  - The worktree picker's entry shows the folder name, then its branch in smaller text, then its
    always-visible buttons (new window, editor, remove). Its menu repeats the buttons plus Switch to
    worktree, so it needs no "…". Every entry action closes the list; a refusal shows as a notice.
  - The repo picker names a repo by its main worktree. It starts no core until a repo is chosen, so
    no WSL core starts uninvited. If the worktree it would switch to is gone, it falls back to the
    repo's main worktree. Each recent entry but the window's own has a menu (Open in new window,
    Remove from recents) with a "…" button. Open in new window resolves the entry as choosing it
    does, falling back to the repo's main worktree, then opens or focuses that worktree's window.
  - The branch picker lists "New branch…" first, then local branches, remote branches and tags. A
    menu action's refusal shows as a notice; a row click's refusal stays in the list. A tag's Show
    history sets the log's filter and brings the log view forward. Merge and rebase that stop on
    conflicts open the diff window on the first conflicted file and leave the rest to the conflict
    banner, and a refusal over local changes offers the stash-and-retry as its notice's action.
    Every entry action is menu-only, so each entry has a "…" button.
- **Settings** (FR-UX-04): a dialog over the window, opened from the toolbar's gear button, laid out
  like every dialog, with a group per editor environment and a base-folder field per environment
  found (local and each WSL distro).
- **Debug area** (FR-UX-19): a strip along the bottom of each main window, with one button per
  sample notice (error, warning, success). Its sample notices take the normal notice path.
- **Log view**: the filter bar above the commit list, and between them the error a failed load ended
  with. Each filter is its own field with a clear icon (FR-LOG-08).
  - Every row splits its width into the same columns (FR-LOG-01): graph, subject with the ref labels
    after it, author, date, short hash. The graph and hash columns are as wide as the widest among
    the rendered rows, so columns line up while the list scrolls; each row draws its own graph strip
    (ADR-5). A column too narrow for a readable value hides, the author first, then the date
    (FR-UX-09).
  - The filter lives in the log store, so the filter bar shows it whoever set it. The branch filter
    names each branch and tag by its full ref, so a branch and a tag of the same name stay apart.
  - Search (message text or hash) does not filter: it highlights the matching rows of whichever view
    is showing, scrolls to the first, and Enter/Shift+Enter step through them, wrapping.
  - Enter or a double-click opens a commit, which brings up the commit detail view in medium and
    narrow (FR-UX-08). The list's text cannot be selected (FR-LOG-12), so a drag never fights the
    click handling.
- **Selection** (FR-LOG-13): the log store keeps one ordered selection, however it was built.
  - A plain click replaces it with one row and loads its details. Ctrl-click toggles one row,
    keeping the rest in order. Shift-click replaces it with the range from the anchor (the last row
    clicked plainly or with Ctrl) to the clicked row, anchor first. A right-click outside the
    selection selects that row alone first (FR-LOG-11).
  - Exactly two selected commits are a comparison, from the first to the second. The compare store
    derives it from the log store's selection and holds only its extras: its files and the
    working-tree comparison. Swap reverses the selection; stop comparing drops it. Three or more are
    no comparison, and the commit detail view says how many are selected.
  - The menu on several commits asks the core whether they can be squashed before it opens, and
    offers squash and fixup or says why not (FR-LOG-14).
  - Cherry-pick and revert pass the selected commits as selected; the core orders them, by commit
    date and, for commits sharing a date, topologically.
- **Commit detail view**: the selected commit's message and fields (FR-UX-10; parents on one line)
  above its changed files, split by a resizable divider that starts in the middle and is remembered
  like the view widths (FR-LOG-15). In its place, the comparison (FR-CMP), whose header shows from →
  to and a Swap button, or the working-tree comparison, or, with three or more selected, their
  count.
- **Changed files** (FR-LOG-16): a flat list, or a folder tree built from the paths, switched by a
  toggle above the files, beside their count. The choice lives in local storage (ADR-4), shared by
  the commit detail view, the diff window and the changes view (FR-CHG-13).
- **Changes view** (FR-CHG-11, FR-CHG-12, FR-CHG-13): Stage all, Unstage all and the list/tree
  toggle above the change list's groups, then the commit box. Each group shows as a list or a tree,
  the shared choice; a file's row starts with its stage, unstage or mark-resolved button, before its
  name, in both, and its other actions sit at the row's end (FR-CHG-02); a folder in the tree has no
  buttons. The commit message field spans the commit box's full width. A conflicted file opens in
  the diff window; a staged file resolved during the stopped operation can be reopened from its row,
  after a confirmation. A resolved file with no change left (resolved to the current side's version)
  is in no other group, so the conflicted list shows it for the same reopen. Every operation the
  window starts that stops on conflicts (pull, merge, rebase, cherry-pick, revert, squash, fixup)
  opens the diff window on the first conflicted file; a conflict a terminal command causes does not.
  The conflict banner's "Resolve conflicts" button, shown while a file is conflicted, opens it
  again.
- **Diff window** (FR-DIFF-07): a separate OS window; the main window stays usable behind it. The
  target's files on the left, a working tree's in their groups, and the diff view on the right. A
  target is a commit, a range, a commit against the working tree, a working-tree side, or a
  conflicted file, which shows the merge editor for text and otherwise a panel that says why and
  offers what fits: a side's version or deletion, or the external editor and mark resolved. When the
  shown file is resolved, the window moves to the next conflicted one. In every file list of the
  main window a click selects a row and a double-click or Enter opens it; opening another file
  retargets the open diff window (`diff:show`). A working-tree list and diff reload on
  `repo:changed` with scope `index`. Splitters between the file list and the diff, and between the
  diff's two sides, are remembered like the main window's view widths. The list's flat/tree toggle
  and its remembered choice are the commit detail view's. The window's size is kept in the settings
  file beside the open windows; its position is not, since a stored position can land off-screen
  after a monitor change and the window opens centred on the current screen.
- **Diff view** (ADR-7): git's diff drawn by CodeMirror, with git's hunks mapped onto it. Hunk and
  line actions appear for a modified working-tree file with whitespace shown. Its toolbar, with the
  side-by-side/unified and whitespace toggles (FR-DIFF-01, FR-DIFF-06), stays in view while the diff
  scrolls. Side by side, the two editors sit in a splitter's panels and scroll sideways together.
  The other side follows through the scroll event, which the browser fires after drawing the
  scrolled side, so it trails by a frame. Taking over wheel input to move both sides in one frame
  would give up the browser's asynchronous and smooth scrolling for the whole diff (a non-passive
  wheel listener blocks every wheel scroll over it), and scrollbar and keyboard scrolling would
  still go through the scroll event; one shared sideways scroller cannot serve two panes that each
  have their own width. Added and removed lines take their colours from the theme, mixed strong
  enough to meet FR-DIFF-11 in every theme, measured as: each changed line's background differs from
  the diff's background, and added from removed, by at least 0.08 ΔE in OKLab; text on them keeps
  4.5:1 contrast; the change marker keeps 3:1 against the diff's background. An e2e check measures
  each theme.
- **Merge editor** (FR-CONF-05, FR-CONF-06, ADR-7): three CodeMirror editors in a splitter, the
  current side and the incoming side read-only around the editable result. Each pane names its side
  and that side's role in the operation.
  - One list of regions covers every change the user decides: the conflicts (marker blocks) and the
    changes only one side made, which git already applied to the file. A change both sides made
    alike is applied and not a region. The core computes the list, each region with a kind and its
    lines in the result, base and both sides.
  - The result starts as the working file with each region's lines replaced by the base version's
    lines (none without a base), so neither git's markers nor a side's change appear in it until the
    user takes it. Each region keeps its range in the result through every edit (CodeMirror's
    position mapping) and carries an explicit state, unresolved or resolved. A take puts the left or
    the right side's lines, or both, in the region's place and sets resolved, the same for every
    kind; taking the side that left a change alone keeps the base. "Mark done" sets resolved on
    whatever the region holds then; a hand edit alone changes nothing. "Apply non-conflicting
    changes" takes the changed side of every open one-side region. Takes, "Apply non-conflicting
    changes" and "Mark done" are single undoable changes, and undo restores the state. "Save and
    mark resolved" waits until no region is unresolved and no marker line is left anywhere in the
    result.
  - The toolbar's actions act on the selected region: the one last clicked in any pane, or holding
    the result's cursor; previous and next move between unresolved regions.
  - The panes tint the lines of an unresolved region by kind: blue for changed, green for added, red
    for a conflict; a side pane tints a one-side change on the side that made it only. A resolved
    region and the lines both sides changed alike are a neutral tint. The selected region is
    outlined in the theme's accent colour and marked in a gutter in all three panes, and its tint is
    stronger. The tints come from the theme's tone tokens, mixed strong enough that each differs
    from the editor background by at least 0.08 ΔE in OKLab and keeps 4.5:1 text contrast, and the
    outline differs from the background and each tint; an e2e check measures each theme.
  - The panes stay aligned by region: each region's first and last line in every pane are anchor
    points, lines between anchors map linearly, and scrolling any pane moves the others through the
    scroll event, as the diff view's sideways scrolling does.
  - Unsaved results live per file in the worktree's session in the core, with their region states,
    in memory only: they outlive the diff window, die with the app, and closing the window never
    asks. The page writes them through as the user edits and reads them back when it shows the file.
    A result whose file changed on disk meanwhile stays, with a reload that discards it.

## Communication

- **Calls** (UI → Go): generated bindings. Each returns a DTO or rejects with the core error
  (`errors-security.md`). Large data (log rows, file contents) always goes through calls, and so
  does cancelling a job. A window can call only the window part of the core API; the calls the shell
  makes for itself (opening, focusing and closing a worktree session, setting a core's editor
  profile, finding a repo's main worktree) are not bound.
- **Events** (Go → UI), small payloads only:

| Event              | Meaning                                                                                                                                               |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `repo:changed`     | something changed, outside or inside the app; its scope is `refs`, `index`, `worktrees` or `state` (`git-mapping.md`)                                 |
| `log:progress`     | more log rows are available; it tells a worktree's filtered load from the repo's shared one, and a failed load's last one carries its error           |
| `job:update`       | a job started, progressed or ended                                                                                                                    |
| `command`          | a git command ran; shell-only, written to the app log, never forwarded to a window                                                                    |
| `editor`           | an editor command started or failed to; shell-only, like `command`                                                                                    |
| `core:reconnected` | a WSL core was restarted; a main window hands the new core its typed commit message and reloads from scratch, a diff window reloads its list and diff |
| `settings:changed` | settings were saved; every window reloads its settings and summary, so the theme and the "Open in …" label follow                                     |
| `diff:show`        | the shell retargets an open diff window to another file                                                                                               |

Each event a window receives is registered in the shell with its Go payload type, and the binding
generator types the frontend's handlers from that registration, so no event type is written by hand
in TypeScript. A value from a fixed set (a change's scope, a job's kind and state, a pull mode) is a
Go string type with constants, generated into a TypeScript enum, so a mistyped value fails the type
check. A core's events travel as JSON, so they survive the RPC hop from a WSL core; the shell
decodes each into its registered type before emitting it.

Wails delivers every event to every window, naming as its sender the window it was emitted through.
The shell emits each event through each window it concerns (ADR-9), and every frontend listener
drops events whose sender is not its own window; only `settings:changed`, meant for every window,
skips that filter. A WSL core's events reach only windows on that distro, with the worktree id
rewritten to the shell's UNC form (ADR-3).
