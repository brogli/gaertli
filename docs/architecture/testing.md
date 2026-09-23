# Testing

Every acceptance case in a feature spec becomes one test, written before the code (TR-29). The test name carries the requirement ID (TR-30).

| Level | What | Tools |
|---|---|---|
| Parser unit tests | Recorded git output → structs: odd file names, renames, empty repo, detached HEAD. | Go test, inline fixtures |
| Graph unit tests | Hand-made histories → expected lanes. | Go test |
| Integration tests | Real git against temp repos built by a helper, with a local bare repo as the remote. Every core method. | Go test |
| Protocol tests | A slice of the integration suite runs a second time through the RPC client and a real `nebegit-core --stdio` child. This tests the WSL transport without WSL. | Go test |
| Frontend component tests | Stores and components with mocked bindings. The default for UI cases (TR-31). | Vitest, Vue Test Utils |
| Frontend e2e | Only cases a component test cannot prove, such as virtual scrolling. Vite dev server with a mocked Go layer. | Playwright |
| Performance checks | A generated 100k-commit repo; asserts QA-PERF-02/04/07. On demand and nightly. | Go benchmark + script |
| Manual checklist | What needs real windows, a real WSL distro or a human judgement (below). | owner, per platform |
| Traceability check | Every MUST ID has a test name or a manual entry. | `tools/trace`, in CI |

## Manual test checklist

The owner runs this per platform before a release. `tools/trace` accepts the IDs listed here without an automated test.

| ID | Check | Platform |
|---|---|---|
| FR-REPO-01 | "Open folder…" shows the native dialog, and the picked directory opens as a worktree. | Windows, Linux |
| TR-13, FR-REPO-02 | Quit and relaunch: recent repos and the windows open at quit are restored. | Windows, Linux |
| FR-REPO-04 | Edit a file in the editor, commit in a terminal: the window updates within the stated times, and at once on focus. | Windows, Linux |
| FR-REPO-06 | `nebegit <path>` from a Windows, Linux and WSL terminal opens or focuses the right window. Inside a distro, after "Install nebegit command": `nebegit <path>` and `nebegit` (the current directory) work whether the app is running or not. | Windows, WSL, Linux |
| FR-REPO-07 | "Open WSL folder" with two or more distros, one of them WSL 1: both are listed, WSL 1 disabled; a WSL 2 distro opens at its root and the repo opens in a native window. | Windows |
| ADR-9, FR-WT-05 | Select another worktree in the panel: title and content switch in place, no new window. Switching back shows fresh state. | Windows, Linux |
| FR-WT-02, FR-WT-03 | "+ New worktree" from an existing branch and with a new branch: both appear in the panel. A target folder that has files in it is refused with git's message. | Windows, Linux |
| FR-WT-04, FR-ED-01, FR-ED-02 | Configure the editor, another editor and a custom command, each for a different environment (Windows, a WSL distro). "Open in editor" from a window, from a worktree row in the panel, and for a file from commit details, compare, the compare window and the diff toolbar opens the right folder or file with that environment's profile. | all |
| FR-WT-08, FR-WT-09 | Opening a worktree that already has a window focuses it. Titles show worktree and branch. | all |
| FR-WT-11 | Two repos, one worktree of each in its own window: both work independently at the same time. | Windows, Linux |
| FR-CMP-01, FR-CMP-02 | Compare two commits (Ctrl-click, or "Compare with selected") and double-click a file: a separate window opens with the file list and diff; next/previous change and file work; the main window stays usable; Escape and the close button close it; double-clicking another file of the same comparison reuses the window. | Windows, Linux |
| FR-CONF-05 | Resolve a conflict from a pull, merge, cherry-pick or revert: nebegit shows no merge editor of its own; resolution happens in the external editor. | all |
| FR-ED-06 | Misconfigure the editor command and open something: the error names the problem and its link opens the setting. | all |
| FR-ED-08 | "Test" on a WSL distro's profile with a command that exists only inside the distro: the editor starts inside the distro. | Windows |
| FR-ED-10 | Inside a WSL worktree, misconfigure that distro's profile and choose "Open with system default": `xdg-open` runs inside the distro. Without an opener, `EDITOR_NOT_FOUND` shows again with both fallbacks. | Windows |
| QA-PORT-02 | First use of a distro via "Open WSL folder": the consent dialog names the distro and target path before any copy. Declining leaves no file and shows `DEPLOY_REFUSED`. Accepting opens the repo; git, hooks and the editor run inside WSL. | Windows |
| QA-PORT-02 | With a WSL repo open, run `wsl --shutdown`, then retry an action: the window recovers without being reopened; a brief error is acceptable. | Windows |
| QA-PORT-03 | Across this checklist, no feature exists on one of Windows and Linux but not the other, apart from the documented WSL and macOS exceptions. | Windows, Linux |
| QA-PERF-01 | A fresh launch shows a usable window within 2 s (stopwatch; there is no window-ready signal to automate against). | Windows, Linux |
| QA-PERF-03 | Scrolling the 100k-commit log shows no visible freeze. | Windows, Linux |
| QA-INST-01 | Start without WebView2 (Windows) or WebKitGTK (Linux): Windows shows a native message naming WebView2; on Linux the loader fails before any Go code runs, so check the failure is visible in the terminal. | Windows, Linux |
| QA-INST-01 | On a fresh machine, download the release zip or AppImage and run it: it starts with only git and the webview runtime installed, as the README says. | Windows, Linux |
| QA-USE-01 | An an IDE git user finds commit (Ctrl+K), pull (Ctrl+T), push (Ctrl+Shift+K) and "Compare with working tree" (commit context menu) where they expect them. | all |
| QA-USE-02 | Editor presets, "Test", ignore-whitespace, the theme picker and the editor fallbacks are each at most two steps from the main window. | all |
| QA-MNT-01 | Every feature spec lists the IDs it covers. The specs live outside the repo, so no tool checks this. | — |

## CI

GitHub Actions, on `windows-latest` and `ubuntu-24.04` (with `libgtk-4-dev` and `libwebkitgtk-6.0-dev`). Each job has two steps: install dependencies with network access, then run offline (TR-14): lint (`golangci-lint`, `oxlint`, `eslint`, `vue-tsc`), tests, build. The Linux job runs the Go tests against git 2.40 and against the runner's current git. `macos-latest` only builds (QA-PORT-04).
