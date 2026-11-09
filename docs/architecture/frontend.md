# Frontend

## State

- Each window is its own WebView with its own stores, so window state is isolated.
- The generated bindings are the only way to reach Go.
- One store per area (`repo`, `log`, `refs`, `diff`, `compare`, `changes`, `jobs`, `settings`).
  Stores hold view state only: after a write they reload from Go, never update optimistically
  (QA-COMP-01), and events tell them when to reload.
- Cross-cutting paths exist once: one for confirming risky actions (QA-SAFE-01), one for core errors
  (FR-UX-01), one guard for window shortcuts.
- Selection is the log store's one ordered list; a comparison is derived from it, not stored.

## Main window

Three views, laid out by the window's width: side by side when wide, the log beside a tabbed panel
at medium width, one tabbed view when narrow (FR-UX-08). Hidden views stay mounted, so they keep
their scroll position and the typed message.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [repo ▾] [worktree ▾] [⎇ branch ↑2 ↓1 ▾] Fetch Pull Push  Jobs Refresh ✎ ⚙│
├───────────────────────────┬──────────────────────┬───────────────────────┤
│ Log view                  │ Commit detail view   │ Changes view          │
│ filter bar                │ (selected commit, or │ staged, unstaged,     │
│ graph + virtual list      │ the comparison)      │ untracked, conflicted │
│                           │                      ├───────────────────────┤
│                           │                      │ Commit box            │
└───────────────────────────┴──────────────────────┴───────────────────────┘
```

States (a conflict in progress, a pull's waiting stash) show as banners below the toolbar; results
show as notices. The pickers start no core until a repo is chosen, so no WSL core starts uninvited.

## Diff window

A separate OS window per main window (FR-DIFF-07) that selects nothing: it shows a target (a commit,
a range, a working-tree side, a conflicted file) and is retargeted by the shell when another file is
opened. Its size is remembered, its position not, since a stored position can land off-screen.

Side by side, each diff pane scrolls on its own, as the merge editor's panes do, so its scrollbars
and the divider stay in view. The two scroll together through the scroll event and trail by a frame.
Taking over wheel input instead would block smooth scrolling for the whole diff and still miss
scrollbar and keyboard scrolling.

## Merge editor

Three editors: the current and incoming sides read-only around the editable result (FR-CONF-05).

- The core computes one list of changes: the conflicts and the changes only one side made. A change
  both sides made alike is not listed.
- The result starts with every region at the base version, so nothing enters it until the user takes
  it. Each region tracks its range through edits and carries an explicit state, unresolved or
  resolved; saving waits until all are resolved.
- The panes stay aligned by region boundaries.
- Unsaved results live in the worktree session in the core, in memory only (ADR-4): they outlive the
  diff window and die with the app.
