# Repository

| ID | Prio | Requirement |
|---|---|---|
| FR-REPO-01 | MUST | Open a local repo by picking a folder. Any folder inside a repo or inside one of its worktrees works. |
| FR-REPO-02 | MUST | Remember recent repos. Started without a path: reopen the windows that were open at last quit, each on its last selected worktree. Started with a path (FR-REPO-06): open that one only. |
| FR-REPO-03 | MUST | Always show: repo, window's worktree, current branch, ahead/behind counts vs upstream, any operation in progress (merge, rebase, cherry-pick, revert), and detached HEAD. |
| FR-REPO-04 | MUST | Pick up changes made outside the app (terminal, editor) while the window has focus: git-state changes (commit, checkout, staging, new branch) within 2 seconds, edits to working files within 3 seconds. Very large working trees may take longer; the app then shows that it is refreshing. The window refreshes at once when it gains focus, and on demand. |
| FR-REPO-06 | MUST | Start from a terminal: `nebegit [path]` opens or focuses the window for that worktree; from a WSL terminal it opens the app on Windows, showing the WSL repo. Inside a WSL distro the command is set up when the app first opens a repo there, under the consent it asks then (QA-PORT-02). When the app is moved, the command starts the moved app once it has opened a repo there, also when the app lies inside that distro's file system. If the command's folder is new, the command works from the next login shell, and the consent says so. The Linux package installs the command; for the Windows download and the Linux archive the user docs say how. |
| FR-REPO-07 | MUST | On Windows, "Open folder…" opens a repo on the Windows file system or inside a WSL distro from the same folder dialog; the app tells the two apart by itself, and both behave the same afterwards. Recent repos show which kind each is. A repo in a WSL distro that is not installed says so. |
| FR-REPO-05 | MUST | If the folder is not a git repo, say so clearly. Do nothing else. |
