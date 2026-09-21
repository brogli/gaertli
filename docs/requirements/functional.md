# Functional requirements (FR)

### Repository (REPO)

| ID | Prio | Requirement |
|---|---|---|
| FR-REPO-01 | MUST | Open a local repo by picking a folder. Any folder inside a repo or inside one of its worktrees works. |
| FR-REPO-02 | MUST | Remember recent repos. Started without a path: reopen the windows that were open at last quit. Started with a path (FR-REPO-06): open that one only. |
| FR-REPO-03 | MUST | Always show: repo, window's worktree, current branch, ahead/behind counts vs upstream, and any operation in progress (merge, rebase, cherry-pick, revert), and detached HEAD. |
| FR-REPO-04 | MUST | Pick up changes made outside the app (terminal, editor) while the window has focus: git-state changes (commit, checkout, staging, new branch) within 2 seconds; edits to working files within 3 seconds. On very large working trees this may take longer; then the app shows that it is refreshing. Immediate refresh when the window gains focus. Manual refresh is also possible. |
| FR-REPO-06 | MUST | Start from a terminal: `nebegit [path]` opens (or focuses) the window for that worktree. From a WSL terminal this opens the app on Windows, showing the WSL repo — like the `editor` command does. The app offers a one-click action to make the `nebegit` command available in a terminal (incl. inside WSL). |
| FR-REPO-07 | MUST | On Windows, two ways to open a repo: a folder on the Windows file system, or "Open WSL folder": pick a WSL distro, then browse folders inside it. Both behave the same afterwards. Recent repos show which kind each one is. |
| FR-REPO-05 | MUST | If the folder is not a git repo, say so clearly. Do nothing else. |

### Log viewer (LOG)

| ID | Prio | Requirement |
|---|---|---|
| FR-LOG-01 | MUST | List commits of all branches by default. Per commit: graph, subject, author, date, short hash, ref labels (local branches, remote branches, tags, HEAD). |
| FR-LOG-02 | MUST | Show a branch graph with merges and forks. While a text search (message, author) is active, a flat list without graph lines is acceptable. |
| FR-LOG-03 | MUST | Select a commit to see details: full message, author, committer, dates, parents, list of changed files. |
| FR-LOG-04 | MUST | Open the diff of any changed file of a commit (vs its parent). |
| FR-LOG-05 | MUST | Search by message text, author, or hash. |
| FR-LOG-06 | MUST | Filter by one or more branches. |
| FR-LOG-07 | MUST | Filter by file or folder path. |
| FR-LOG-08 | MUST | Filters can be combined. Active filters are visible. One action clears them all. |
| FR-LOG-09 | MUST | The log is usable before the full history is loaded. |
| FR-LOG-10 | MUST | Mark which commits are checked out, and in which worktree. |
| FR-LOG-11 | MUST | Commit actions are reachable from a context menu on the commit: compare, new branch from here, reset current branch to here, check out commit, cherry-pick, revert, create worktree, copy hash. |
| FR-LOG-12 | SHOULD | Copy the hash or the subject of a commit. |

### Compare commits (CMP)

| ID | Prio | Requirement |
|---|---|---|
| FR-CMP-01 | MUST | Select two commits and compare them. Show the list of changed files with their type: added, modified, deleted, renamed. |
| FR-CMP-02 | MUST | Open the diff of any file in that list. |
| FR-CMP-03 | MUST | Show the direction clearly (from → to). Allow swapping. |
| FR-CMP-04 | SHOULD | Compare a commit with the current working files. |

### Diff view (DIFF)

| ID | Prio | Requirement |
|---|---|---|
| FR-DIFF-01 | MUST | Side-by-side and unified view. Toggle between them. Remember the choice. |
| FR-DIFF-02 | MUST | Jump to next/previous change and next/previous file. |
| FR-DIFF-03 | MUST | Binary files are shown as "binary", without content. |
| FR-DIFF-04 | MUST | Very large files do not freeze the app. Warn and load on demand. |
| FR-DIFF-05 | SHOULD | Syntax highlighting. |
| FR-DIFF-06 | SHOULD | Toggle "ignore whitespace". |

### Branch actions (BR)

| ID | Prio | Requirement |
|---|---|---|
| FR-BR-01 | MUST | Create a new branch from any commit. Options: name, check out after creation. Invalid or duplicate names are rejected with a clear message. |
| FR-BR-02 | MUST | "Reset current branch to here" in the log's context menu on any commit, like an IDE. The dialog names the branch and the target commit. The user picks the mode each time: soft, mixed, or hard. Each mode is explained in one line. |
| FR-BR-03 | MUST | Hard reset asks for confirmation and shows what will be lost (number of uncommitted changed files). |
| FR-BR-04 | MUST | After a reset, show the previous commit hash so the user can go back. |
| FR-BR-05 | MUST | Check out a local branch in this window's worktree. If git refuses (local changes in the way, branch in use by another worktree), show why. |
| FR-BR-08 | MUST | Check out a commit from the log (detached HEAD). Warn before: commits made in this state belong to no branch. While detached, show it clearly and offer "create branch here". |
| FR-BR-07 | SHOULD | Fourth reset mode "keep" (as in an IDE): like hard, but refuses if local changes would be lost. |
| FR-BR-06 | SHOULD | Rename and delete local branches. Deleting an unmerged branch needs confirmation. |

### Worktrees (WT)

| ID | Prio | Requirement |
|---|---|---|
| FR-WT-01 | MUST | List all worktrees. Per worktree: path, branch or detached commit, clean/dirty, flags (main, locked, missing). |
| FR-WT-02 | MUST | Create a worktree at a location the user chooses. Source: an existing branch, or a new branch from a branch or commit. |
| FR-WT-03 | MUST | The target folder must not exist or must be empty. Otherwise reject with a clear message. |
| FR-WT-04 | MUST | "Switch to worktree" = open that worktree in the configured external editor (ED) with one action. |
| FR-WT-05 | MUST | **One window per worktree.** Each app window is bound to one worktree. Changes, commit, pull, push, reset, checkout, cherry-pick, revert act on that worktree only. There is no worktree switcher inside a window. |
| FR-WT-08 | MUST | From the worktree list: open a worktree in its own app window. If that worktree already has a window, focus it instead of opening a second one. |
| FR-WT-09 | MUST | The window title shows worktree name/path and current branch, so windows are easy to tell apart. |
| FR-WT-10 | MUST | Log, branches, tags, and the worktree list are the same in all windows of one repo and stay in sync. |
| FR-WT-11 | SHOULD | Windows for worktrees of different repos at the same time. |
| FR-WT-12 | SHOULD | After creating a worktree, offer: open in app window, open in editor, both. |
| FR-WT-06 | SHOULD | Create a worktree straight from a commit or branch in the log. |
| FR-WT-07 | SHOULD | Remove a worktree. Confirm first. Refuse if it has uncommitted changes unless the user forces it. Clean up stale entries. |

### Changes and commit (CHG)

| ID | Prio | Requirement |
|---|---|---|
| FR-CHG-01 | MUST | Show changed files of this window's worktree in groups: staged, unstaged, untracked, conflicted. |
| FR-CHG-02 | MUST | Stage and unstage whole files, and all files at once. |
| FR-CHG-03 | MUST | Stage and unstage single hunks and selected lines. |
| FR-CHG-04 | MUST | Show the diff of any changed file. |
| FR-CHG-05 | MUST | Commit staged changes with a message. Blocked when nothing is staged or the message is empty. |
| FR-CHG-06 | MUST | Amend the last commit. Warn if that commit is already pushed. |
| FR-CHG-07 | MUST | Git hooks run as usual. If a hook fails, show its output. |
| FR-CHG-08 | MUST | Keep the message draft until the commit succeeds. |
| FR-CHG-09 | SHOULD | "Commit and push" in one action. |
| FR-CHG-10 | SHOULD | Discard changes per file or hunk, with confirmation. |

### Fetch, pull, push (SYNC)

| ID | Prio | Requirement |
|---|---|---|
| FR-SYNC-01 | MUST | Fetch from one remote or all remotes. Remove remote branches that no longer exist. |
| FR-SYNC-02 | MUST | Pull the current branch. The user chooses merge or rebase per pull. The last choice is preselected. |
| FR-SYNC-03 | MUST | Push the current branch. If it has no upstream, offer to set one. |
| FR-SYNC-04 | MUST | If a push is rejected, explain why and offer: pull, or force push. |
| FR-SYNC-05 | MUST | Force push only in the safe form: it fails if the remote has commits the user has not fetched. Plain force push is not offered. |
| FR-SYNC-06 | MUST | Force push asks for confirmation. Protected branches (configurable, default `main`, `master`) get an extra warning. |
| FR-SYNC-07 | MUST | Remote actions show progress and can be cancelled. |
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
| FR-ED-03 | MUST | The user configures which editor is used. Every "open in editor" action in the app uses this one setting. No action is tied to a specific editor. |
| FR-ED-04 | MUST | The user can enter any custom command, with separate forms for "open folder" and "open file" (file form may include line). |
| FR-ED-05 | MUST | Default is the editor. The UI says "Open in editor", or shows the configured editor's name — never a fixed "Open in the editor". |
| FR-ED-06 | MUST | If the editor cannot be started, show a clear error with a direct way to the setting. |
| FR-ED-07 | SHOULD | Presets for common editors (e.g. the editor, another editor, an IDE, a third editor, a terminal editor in a terminal) so the user only picks from a list. |
| FR-ED-08 | SHOULD | A "test" action in settings that tries the command. |
| FR-ED-09 | SHOULD | Open the file at the line shown in the diff, if the editor command supports it. |
| FR-ED-10 | SHOULD | If the configured editor (including the editor default) cannot be started, offer as fallback the user's git editor setting or the system default for opening files. |

### Feedback and settings (UX)

| ID | Prio | Requirement |
|---|---|---|
| FR-UX-01 | MUST | Every action ends with a visible result. Errors include git's original message. |
| FR-UX-02 | MUST | A command log shows every git operation the app ran, with its output. Operations started by the user are shown by default. Automatic background reads (refresh, polling) are hidden by default and can be switched on. |
| FR-UX-03 | MUST | Confirmation before: hard reset, force push, discard changes, remove worktree, delete unmerged branch. |
| FR-UX-04 | MUST | Settings: external editor (ED), protected branches, default pull mode, diff mode, auto-fetch interval. |
| FR-UX-05 | SHOULD | Keyboard shortcuts for the main actions. |
| FR-UX-06 | SHOULD | Light and dark theme. |

---
