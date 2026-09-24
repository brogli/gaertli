# Functional requirements (FR)

### Repository (REPO)

| ID | Prio | Requirement |
|---|---|---|
| FR-REPO-01 | MUST | Open a local repo by picking a folder. Any folder inside a repo or inside one of its worktrees works. |
| FR-REPO-02 | MUST | Remember recent repos. Started without a path: reopen the windows that were open at last quit, each on its last selected worktree. Started with a path (FR-REPO-06): open that one only. |
| FR-REPO-03 | MUST | Always show: repo, window's worktree, current branch, ahead/behind counts vs upstream, any operation in progress (merge, rebase, cherry-pick, revert), and detached HEAD. |
| FR-REPO-04 | MUST | Pick up changes made outside the app (terminal, editor) while the window has focus: git-state changes (commit, checkout, staging, new branch) within 2 seconds; edits to working files within 3 seconds. On very large working trees this may take longer; then the app shows that it is refreshing. Immediate refresh when the window gains focus. Manual refresh is also possible. |
| FR-REPO-06 | MUST | Start from a terminal: `nebegit [path]` opens (or focuses) the window for that worktree. From a WSL terminal this opens the app on Windows, showing the WSL repo — like the `editor` command does. Inside a WSL distro the command is set up when the app first opens a repo there, as part of the consent it asks then (QA-PORT-02); the Linux package installs it; for the Windows download and the Linux archive the user docs say how. |
| FR-REPO-07 | MUST | On Windows, one "Open folder…" opens a repo on the Windows file system or inside a WSL distro, picked in the same folder dialog; the app tells which kind it is by itself, and both behave the same afterwards. Recent repos show which kind each one is. |
| FR-REPO-05 | MUST | If the folder is not a git repo, say so clearly. Do nothing else. |

### Log viewer (LOG)

| ID | Prio | Requirement |
|---|---|---|
| FR-LOG-01 | MUST | List commits of all branches by default. Per commit: graph, subject, author, date, short hash, ref labels (local branches, remote branches, tags, HEAD). |
| FR-LOG-02 | MUST | Show a branch graph with merges and forks. While a text search (message, author) is active, a flat list without graph lines is acceptable. |
| FR-LOG-03 | MUST | Select a commit to see details: full message, author, committer, dates, parents, list of changed files. |
| FR-LOG-04 | MUST | Open the diff of any changed file of a commit (vs its parent), in the diff window (FR-DIFF-07). |
| FR-LOG-05 | MUST | Search by message text, author, or hash. |
| FR-LOG-06 | MUST | Filter by one or more branches, found by searching their names. |
| FR-LOG-07 | MUST | Filter by file or folder path. |
| FR-LOG-08 | MUST | Filters can be combined. Each filter shows its value in its own field and has a clear icon that empties it. No separate list of active filters, no clear-all action. |
| FR-LOG-09 | MUST | The log is usable before the full history is loaded. |
| FR-LOG-10 | MUST | Mark which commits are checked out, and in which worktree. |
| FR-LOG-11 | MUST | The log view's context menu offers only what applies to the selection it opens on, worded for it. Right-clicking a commit outside the selection selects that commit alone first. One commit: copy hash, copy subject, new branch from here, check out commit, reset current branch to here, cherry-pick, revert (each also with its options, FR-CP-06, FR-RV-04), create worktree here, compare with working tree, and "Compare with <short hash>" naming the commit selected before. A range of commits (Shift-click): cherry-pick and revert of all of them, labelled with their count, and copy their hashes. Two compared commits (FR-CMP-01): swap, stop comparing, copy both hashes. Two or more selected commits, as a range or picked one by one: squash and fixup (FR-LOG-14). |
| FR-LOG-12 | SHOULD | Copy the hash or the subject of a commit. |
| FR-LOG-13 | MUST | Ctrl-click adds a commit to the selection or takes it out. Two selected commits form a comparison (FR-CMP-01); with three or more there is no comparison, and the commit detail view says how many commits are selected. |
| FR-LOG-14 | MUST | Squash turns two or more selected commits of the current branch into one commit with their messages combined, shown for editing first. Fixup does the same but keeps the message of the oldest selected commit. Both ask first. They say when a selected commit is already pushed, since the branch then needs a force push (FR-SYNC-05), with the extra warning on a protected branch (FR-SYNC-06). If a selected commit is not on the current branch, or a merge commit is in the way, they are not offered and the menu says why. When moving a commit next to another conflicts, the operation stops with the conflict banner (FR-CONF). |

### Compare commits (CMP)

| ID | Prio | Requirement |
|---|---|---|
| FR-CMP-01 | MUST | Select two commits and compare them: Ctrl-click a second commit, or right-click a commit and choose "Compare with <short hash>" (FR-LOG-11). The commit detail view shows the comparison: its direction and the changed files with their status (FR-UX-13). A comparison runs from the commit selected first to the one selected second, whatever their dates. |
| FR-CMP-02 | MUST | Open the diff of any file in that list, in the diff window (FR-DIFF-07). |
| FR-CMP-03 | MUST | The commit detail view shows the comparison's direction (from → to) and a Swap button that reverses it; the log view's marks swap with it (FR-CMP-05). |
| FR-CMP-04 | SHOULD | Compare a commit with the current working files. |
| FR-CMP-05 | MUST | The log view marks the two compared commits in two colours, one for the side the comparison runs from and one for the side it runs to; the commit detail view uses the same colours and names the sides older (from) and newer (to). Older and newer mean selected first and selected second, not the commits' dates. Neither colour is green or red, which diffs use for added and removed lines. |

### Diff view (DIFF)

| ID | Prio | Requirement |
|---|---|---|
| FR-DIFF-01 | MUST | Side-by-side and unified view of git's own diff: the user's git config (diff algorithm, attributes, filters, line endings) decides which lines changed, and changed lines are marked whole. Toggle between the views. Remember the choice. |
| FR-DIFF-02 | MUST | Jump to next/previous change and next/previous file. |
| FR-DIFF-03 | MUST | Binary files are shown as "binary", without content. |
| FR-DIFF-04 | MUST | Very large files do not freeze the app. Warn and load on demand. |
| FR-DIFF-05 | SHOULD | Syntax highlighting. |
| FR-DIFF-06 | SHOULD | Toggle "ignore whitespace": the diff is then git's whitespace-ignoring diff, for reading. Hunk and line actions are offered only with whitespace shown, as `git add -p` has no whitespace-ignoring mode; the view says so. |
| FR-DIFF-07 | MUST | Every file diff opens in the diff window, by double-clicking the file: a commit's file (FR-LOG-04), a comparison's file (FR-CMP-02), a working-tree file (FR-CHG-04). The main window shows no diff. The diff window is a separate OS window: the files of what it shows on the left (the commit's, the comparison's, or the working tree's changed files), the diff on the right, next/previous change and file; for a working-tree file, hunk and line staging and discard (FR-CHG-03, FR-CHG-10). One diff window per main window; opening another diff reuses it. Escape or its close button closes it; the main window stays usable behind it; inspired by an IDE's diff dialog. |

### Branch actions (BR)

| ID | Prio | Requirement |
|---|---|---|
| FR-BR-01 | MUST | Create a new branch from any commit (log view, FR-LOG-11) or from an existing branch picked from a list (branch picker). Options: name, check out after creation. The only thing typed is the new branch's name. Invalid or duplicate names are rejected with a clear message. |
| FR-BR-02 | MUST | "Reset current branch to here…" in the log view's context menu on any commit (FR-LOG-11) opens a dialog that names the branch and the target commit (short hash and subject) and lists the modes one below the other, each with one short line in plain words: soft (the undone commits' changes stay staged), mixed (they stay in the files, unstaged; preselected, as in git), hard (they are discarded, and so are all uncommitted changes), keep (FR-BR-07). The user picks the mode each time. |
| FR-BR-03 | MUST | Hard reset asks for confirmation and shows what will be lost (number of uncommitted changed files). |
| FR-BR-04 | MUST | After a successful reset the dialog closes, the log view shows the branch at its new commit at once, and a notice names the previous commit, copyable, so the user can go back. No second dialog. |
| FR-BR-05 | MUST | Check out a local branch in this window's worktree. If git refuses (local changes in the way, branch in use by another worktree), show why. |
| FR-BR-08 | MUST | Check out a commit from the log (detached HEAD). Warn before: commits made in this state belong to no branch. While detached, show it clearly and offer "create branch here". |
| FR-BR-07 | SHOULD | Fourth reset mode "keep" (as in an IDE): like hard, but uncommitted changes stay; refuses if they touch a file the reset would change. |
| FR-BR-06 | SHOULD | Rename and delete local branches. Deleting an unmerged branch needs confirmation. |
| FR-BR-09 | MUST | The toolbar's branch picker shows the current branch and opens a searchable list: "New branch…" first, then local branches (with ahead/behind), remote branches and tags. Clicking an entry checks it out; a local branch checked out in another worktree switches this window to that worktree instead. Each entry has an actions menu: check out (a local branch, FR-BR-05; a remote branch as a new tracking branch; a tag as a detached HEAD, FR-BR-08), new branch from it (FR-BR-01), new worktree from it (FR-WT-06), update from upstream (FR-UPD-01), rename and delete (FR-BR-06). Inspired by an IDE's branch popup, not a copy of it. |

### Worktrees (WT)

| ID | Prio | Requirement |
|---|---|---|
| FR-WT-01 | MUST | List all worktrees. Per worktree: path, branch or detached commit, clean/dirty, flags (main, locked, missing). |
| FR-WT-02 | MUST | Create a worktree at a location the user chooses. Source: an existing branch, or a new branch starting from an existing branch, both picked from a list; created from the log view, the new branch starts at that commit (FR-LOG-11). The only thing typed besides the location is a new branch's name. |
| FR-WT-03 | MUST | The target folder must not exist or must be empty. Otherwise reject with a clear message. |
| FR-WT-04 | MUST | "Switch to worktree" = open that worktree in the configured external editor (ED) with one action. |
| FR-WT-05 | MUST | **A window acts on one selected worktree.** The repo and worktree pickers (FR-WT-13) pick it; the whole window (log view, branch picker, changes view, diff window) follows the selection, and changes, commit, pull, push, reset, checkout, cherry-pick, revert act on the selected worktree only. |
| FR-WT-08 | MUST | Choosing a worktree in the worktree picker switches this window to it. "Open in new window" shows it in a second window; if a window already shows it, focus that one instead of opening another. |
| FR-WT-09 | MUST | The window title shows the selected worktree's name/path and current branch, so windows are easy to tell apart. |
| FR-WT-10 | MUST | Log, branches, tags, and the worktree list are the same in all windows of one repo and stay in sync. |
| FR-WT-11 | SHOULD | Windows for worktrees of different repos at the same time. |
| FR-WT-13 | MUST | The toolbar has three pickers side by side, left to right: repo, worktree, branch. Each is a button naming the current choice that opens a searchable list. The repo picker lists the recent repos; choosing one switches this window to the worktree it last showed of that repo, else the repo's main worktree; it also offers "Open folder…" (FR-REPO-01, FR-REPO-07). The worktree picker lists the current repo's worktrees (FR-WT-01 details); choosing one switches this window to it; each row opens it in a new window (FR-WT-08), opens it in the editor (FR-WT-04) or removes it (FR-WT-07); the list offers "New worktree…" (FR-WT-02). The branch picker is FR-BR-09. A row's actions are buttons that are always visible, not only on hover. A window with no selection shows the repo picker's list as its content. Inspired by the editor's title bar pickers, not a copy of them. |
| FR-WT-12 | SHOULD | After creating a worktree, offer: open in app window, open in editor, both. |
| FR-WT-06 | SHOULD | Create a worktree straight from a commit or branch in the log. |
| FR-WT-07 | SHOULD | Remove a worktree. Confirm first. Refuse if it has uncommitted changes unless the user forces it. Clean up stale entries. |

### Changes and commit (CHG)

| ID | Prio | Requirement |
|---|---|---|
| FR-CHG-01 | MUST | Show changed files of this window's worktree in groups: staged, unstaged, untracked, conflicted. |
| FR-CHG-02 | MUST | Stage and unstage whole files, and all files at once. |
| FR-CHG-03 | MUST | Stage and unstage single hunks and selected lines, in the diff with whitespace shown. A hunk is git's hunk as the diff shows it; a file's mode change belongs to no hunk. In a hunk that replaces lines one for one, a selected line moves as a whole change: its removed and its added version together; in a hunk that replaces lines with a different number of lines, a line selection takes the whole hunk. An action refuses, changing nothing, when the file changed since the diff was shown. |
| FR-CHG-04 | MUST | Show the diff of any changed file, in the diff window (FR-DIFF-07). |
| FR-CHG-05 | MUST | Commit staged changes with a message. Blocked when nothing is staged or the message is empty. |
| FR-CHG-06 | MUST | Amend the last commit. Warn if that commit is already pushed. |
| FR-CHG-07 | MUST | Git hooks run as usual. If a hook fails, show its output. |
| FR-CHG-08 | MUST | Keep the message draft until the commit succeeds. |
| FR-CHG-09 | SHOULD | "Commit and push" in one action. |
| FR-CHG-11 | MUST | In the changes view the change list (FR-CHG-01) scrolls and the commit box below it does not, so committing never needs scrolling. |
| FR-CHG-12 | MUST | The commit message field spans the commit box's full width and shows at least six lines without scrolling. |
| FR-CHG-10 | SHOULD | Discard changes per file, hunk or selected lines, with confirmation. A discard never removes a line the file had before without restoring it: a discarded line of a hunk that replaces lines one for one gets its previous version back, and a line selection in a hunk that replaces lines with a different number of lines discards the whole hunk. A discard refuses, changing nothing, when the file changed since the diff was shown. |

### Fetch, pull, push (SYNC)

| ID | Prio | Requirement |
|---|---|---|
| FR-SYNC-01 | MUST | Fetch from one remote or all remotes. Remove remote branches that no longer exist. |
| FR-SYNC-02 | MUST | Pull the current branch. The user chooses merge or rebase per pull. The last choice is preselected; before the first choice, the mode the git config sets for the branch. |
| FR-SYNC-03 | MUST | Push the current branch. If it has no upstream, offer to set one. |
| FR-SYNC-04 | MUST | If a push is rejected, explain why and offer: pull, or force push. |
| FR-SYNC-05 | MUST | Force push only in the safe form: it fails if the remote has commits the user has not fetched. Plain force push is not offered. |
| FR-SYNC-06 | MUST | Force push asks for confirmation. Protected branches (configurable, default `main`, `master`) get an extra warning. |
| FR-SYNC-07 | MUST | Remote actions show progress and can be cancelled, in the jobs menu (FR-UX-12). |
| FR-SYNC-11 | MUST | Sync controls offer only what can act. Without a remote, Fetch, Pull and Push are disabled and their tooltip says the repo has no remote. A choice of remote appears only when there is more than one. No control shows an empty list. |
| FR-SYNC-08 | MUST | Use the user's existing git login. The app never asks for, stores, or sees credentials. If login fails, say so and show git's message. |
| FR-SYNC-09 | SHOULD | If a pull is blocked by local changes, offer to stash them and restore them after. |
| FR-SYNC-10 | SHOULD | Automatic fetch at a set interval. Off by default. |

### Update another branch in the background (UPD)

Like an IDE's "Update" on a branch that is not checked out.

| ID | Prio | Requirement |
|---|---|---|
| FR-UPD-01 | MUST | Update a local branch that is not checked out in this window's worktree to its upstream. The window's worktree's files are not touched. |
| FR-UPD-02 | MUST | Runs in the background. The app stays usable. |
| FR-UPD-03 | MUST | Fast-forward only. If the branch has diverged, change nothing and report it. |
| FR-UPD-04 | MUST | If the branch is checked out in another worktree: update only if that worktree is clean and a fast-forward is possible. Otherwise report. |
| FR-UPD-05 | MUST | Report the result: updated (old → new, number of commits), already up to date, or failed with reason. |
| FR-UPD-06 | SHOULD | Update several selected branches in one action. |

### Cherry-pick (CP)

| ID | Prio | Requirement |
|---|---|---|
| FR-CP-01 | MUST | Cherry-pick one or more commits selected in the log onto the current branch of this window's worktree. |
| FR-CP-02 | MUST | Several commits are applied oldest first, whatever the selection order. |
| FR-CP-03 | MUST | If a pick stops on conflicts, the conflict flow (CONF) takes over: list files, resolve in editor, continue or abort. |
| FR-CP-04 | MUST | If git refuses to start (local changes in the way, other operation in progress), show why. Nothing is changed. |
| FR-CP-05 | MUST | Picking a merge commit: refuse with a clear message. |
| FR-CP-06 | SHOULD | Options: add a "cherry picked from …" line to the message; apply without committing. |
| FR-CP-07 | SHOULD | Skip the current commit during a stopped multi-commit pick. |
| FR-CP-08 | SHOULD | Merge commits: let the user choose the parent to pick against. |

### Revert (RV)

| ID | Prio | Requirement |
|---|---|---|
| FR-RV-01 | MUST | Revert one or more commits selected in the log. Creates new commits on the current branch of this window's worktree that undo them. |
| FR-RV-02 | MUST | Several commits are reverted newest first. |
| FR-RV-03 | MUST | Conflicts, refusal to start, and merge commits are handled as for cherry-pick (FR-CP-03 to FR-CP-05). |
| FR-RV-04 | SHOULD | Option: apply without committing. |

### Conflicts (CONF)

| ID | Prio | Requirement |
|---|---|---|
| FR-CONF-01 | MUST | Detect when a pull, merge, rebase, cherry-pick, or revert stopped on conflicts. List the conflicted files. |
| FR-CONF-02 | MUST | Open a conflicted file in the editor. |
| FR-CONF-03 | MUST | Mark a file as resolved. |
| FR-CONF-04 | MUST | Continue or abort the running operation. |
| FR-CONF-05 | — | No built-in merge editor. Conflicts are resolved in the editor. |

### External editor (ED)

The app is editor-neutral. the editor is only the default.

| ID | Prio | Requirement |
|---|---|---|
| FR-ED-01 | MUST | Open a worktree folder in the external editor. |
| FR-ED-02 | MUST | Open a file in the external editor from the changes list, the diff, or the conflict list. |
| FR-ED-11 | MUST | The editor setting can differ per environment (Windows vs. each WSL distro), because the command runs where the repo lives. |
| FR-ED-03 | MUST | The user configures which editor is used. Every "open in editor" action in the app uses this one setting. No action is tied to a specific editor. Settings label it as the setting behind "Open in …", per environment. |
| FR-ED-04 | MUST | The user can enter any custom command, with separate forms for "open folder" and "open file" (file form may include line). |
| FR-ED-05 | MUST | Default is the editor. The UI says "Open in editor", or shows the configured editor's name — never a fixed "Open in the editor". A saved change shows at once in every window. |
| FR-ED-06 | MUST | If the editor cannot be started, a notice says so and names the command that failed, and nothing more. |
| FR-ED-07 | SHOULD | Presets for common editors (e.g. the editor, another editor, an IDE, a third editor, a terminal editor in a terminal) so the user only picks from a list. |
| FR-ED-09 | SHOULD | Open the file at the line shown in the diff, if the editor command supports it. |

### Feedback and settings (UX)

| ID | Prio | Requirement |
|---|---|---|
| FR-UX-01 | MUST | Every action ends with a visible result. Errors include git's original message. |
| FR-UX-02 | MUST | The app writes an app log: every git command it runs, with its result and, when it fails, git's output; and the app's own errors. Automatic background reads (refresh, polling) are left out unless the user asks for a detailed log. No credentials are logged (QA-SEC-02). The window itself lists no git commands; Settings name where the app log is. |
| FR-UX-03 | MUST | Confirmation before: hard reset, force push, discard changes, remove worktree, delete unmerged branch. |
| FR-UX-04 | MUST | Settings: external editor (ED), protected branches, default pull mode, diff mode, auto-fetch interval. Reached from a gear button in the toolbar; opens as a dialog over the window. |
| FR-UX-05 | SHOULD | Keyboard shortcuts for the main actions. |
| FR-UX-06 | SHOULD | Light and dark theme. |
| FR-UX-07 | MUST | A window has the toolbar on top and three views below it, each the full height: the log view, the commit detail view (the selected commit, or the comparison) and the changes view (the change list and the commit box). Looking at history and making a commit are separate tasks, so the views may sit behind a switch. File diffs open in the diff window (FR-DIFF-07). |
| FR-UX-08 | MUST | The layout follows the window's width. Wide: the three views side by side, log view first, their widths adjustable. Medium: the log view, and beside it a switch between the commit detail view and the changes view; selecting a commit brings up the commit detail view. Narrow, down to phone width: one view at a time and a switch among all three; a click on a commit keeps the log view, opening the commit shows its commit detail view. The switch shows how many files the changes view lists. |
| FR-UX-09 | SHOULD | In every layout of FR-UX-08, controls fit their panel: a file's actions stay on its row, a toolbar stays on one row, a checkbox stays beside its label. Text too long for its space ends in an ellipsis and shows in full on hover. A control without visible text has a tooltip naming its action. |
| FR-UX-10 | SHOULD | Views show data formatted, not raw: the commit detail view shows the subject in the UI font, author and committer as labelled fields with dates (the committer only when it differs from the author), and parents as short hashes on one line. |
| FR-UX-11 | MUST | Every text the UI shows is translatable, plurals included. English is the only language for now. Git's own messages are shown as git wrote them. |
| FR-UX-12 | MUST | Running jobs (fetch, pull, push, branch updates) are listed in the jobs menu, in the toolbar left of Refresh. It shows that jobs are running; its list shows each job with its progress and a way to abort it. Starting or ending a job never moves the layout. |
| FR-UX-13 | MUST | Every file list (commit detail view, changes view, diff window) shows a file's status as an icon, not a letter: added, modified, deleted, renamed, copied, type changed, untracked, conflicted. The icon's tooltip names the status. |
| FR-UX-14 | MUST | A dropdown, popup or menu closes on the first click outside it and on Escape. |
| FR-UX-15 | MUST | A notice stays until the user closes it, it closes by itself, or a newer result of the same action takes its place (fetch, pull and push count as one action here). A notice never removes the notice of another action, and a closed notice does not come back. |
| FR-UX-16 | MUST | When a git action fails because another program holds one of git's lock files on the repo, the app says that another program is using the repo and offers to run the action again. |
| FR-UX-17 | MUST | Dialogs are not cramped: each field sits on its own row with its label, with room between rows, and the buttons sit together at the bottom. |
| FR-UX-18 | MUST | The app uses the same font on Windows and Linux. |
