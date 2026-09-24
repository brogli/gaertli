# Glossary

One name per thing (QA-USE-03). Requirements, architecture, code and UI use the term in the first column, never the ones in the last.

| Term | Meaning | Not |
|---|---|---|
| repo | A git repository. The recent repos are the ones opened before. | project, folder |
| worktree | One working folder of a repo: its main worktree or a linked one. A window shows one worktree at a time. | workspace, checkout |
| window | A main app window. Several can be open, each on its own worktree. | |
| toolbar | The row at the top of a window: the pickers, the sync controls, the jobs menu, Refresh, "Open in <editor>", Settings. | header |
| repo picker, worktree picker, branch picker | The three toolbar buttons that name the current repo, worktree and branch and open a searchable list of the others (FR-WT-13, FR-BR-09). | workspace picker, selector |
| sync controls | Fetch, Pull and Push in the toolbar. | |
| job | A git action that runs in the background: fetch, pull, push, a branch update. | task, process |
| jobs menu | The toolbar button that lists the running jobs, each with an abort icon (FR-UX-12). | jobs strip |
| log view | The repo's commits as a list with the graph, and the filter bar above it. | git graph, history |
| graph | The log view's column that draws branches, forks and merges. | |
| filter bar | The log view's search and filter fields. | |
| selection | The commits selected in the log view: one commit, a range (Shift-click), or commits picked one by one (Ctrl-click); exactly two picked commits form a comparison. | |
| comparison | Two compared commits, the older (selected first) and the newer (selected second), with a direction (from → to) and the files that differ. | compare mode |
| commit detail view | The view with the selected commit's details and changed files, or the comparison. | details panel, commit details |
| changes view | The view with the working tree's changes: the change list above the commit box. | changes panel, make commit view |
| change list | The changes view's scrolling part: staged, unstaged, untracked and conflicted files. | |
| commit box | The changes view's fixed part: commit message, amend, push after commit, the Commit button. | commit form |
| switch | The control that picks the visible view when not all three fit side by side (FR-UX-08). | tabs |
| diff window | The separate window every file diff opens in (FR-DIFF-07). | compare window |
| diff view | The diff inside the diff window: git's own diff, unified or side by side. | |
| hunk | One block of changed lines, as git's diff cuts it. | chunk |
| notice | A short message about an action's result that does not block the window. | toast |
| editor | The external editor the user configured, named in "Open in <editor>". | IDE |
| app log | The log file the app writes: every git command it runs and its errors (FR-UX-02). | command log |
| settings | The dialog behind the toolbar's gear (FR-UX-04). | preferences |
