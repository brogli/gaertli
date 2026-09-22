# Testing

Flow: requirement → feature spec (plans dir) → failing test → code (TR-29). Each acceptance case becomes one test whose name carries the ID (TR-30).

| Level | What | Tools |
|---|---|---|
| Parser unit tests | Recorded git output → structs. Odd file names, renames, empty repo, detached HEAD. | Go test, golden files |
| Graph unit tests | Hand-made histories → expected lanes. | Go test |
| Integration tests | Real git, temp repos built by a helper DSL, local bare repo as remote. Every service call. | Go test |
| Protocol tests | A slice of the integration suite runs a second time through the RPC client and a real `nebegit-core --stdio` child, covering the WSL transport without WSL. | Go test |
| Frontend component tests | Stores and components with mocked bindings. Default for UI cases (TR-31). | Vitest, Vue Test Utils |
| Frontend e2e | Only cases a component test cannot prove. Vite dev server with a mocked Go layer. | Playwright |
| Performance checks | Generated 100k-commit repo. Asserts QA-PERF-02/04/07. On demand and nightly. | Go benchmark + script |
| Manual checklist | The QA-MNT-02 exceptions (below). | owner, per platform |
| Traceability check | Every MUST ID has a test name or a manual entry. | `tools/trace`, in CI |

### Manual test checklist

Run by the owner per platform before a release. `tools/trace` accepts these IDs without an automated test.

| ID | Check | Platform |
|---|---|---|
| FR-REPO-04 | Edit a file in the editor, commit in a terminal: window updates within the stated times; immediately on focus. | Windows, Linux |
| FR-REPO-06 | `nebegit <path>` from a Windows, Linux, and WSL terminal opens or focuses the right window. From inside a WSL distro's own shell, after clicking "Install nebegit command" for it: `nebegit <path>` and `nebegit` (no argument, opens the cwd) both open or focus the right window, whether the app is already running or closed. | Windows, WSL, Linux |
| FR-REPO-07 | "Open WSL folder" on a machine with ≥ 2 distros, one WSL 1: both list, the WSL 1 one visibly disabled; picking a WSL 2 one and browsing opens the right folder at the UNC root, and the repo opens in a native window. | Windows |
| FR-WT-04, FR-ED-01, FR-ED-02 | Open folder and file in the configured editor from Windows, WSL, and Linux. Configure the editor, another editor, and a custom command, each for a different environment (a local Windows install and a WSL distro, FR-ED-11): "Open in editor" from each environment's own window, and from a selector-panel worktree row (FR-WT-04, no window needed), opens the right folder, matching that environment's own profile; "open in editor" from the commit-details, compare, and compare-diff-window file lists, and from the diff pane's own toolbar (FR-ED-02), opens the right file the same way. | all |
| FR-ED-06 | Misconfigured editor: error with a link to the setting. Misconfigure an editor command, click "Open in editor": the error names the problem and the "Open editor setting" link opens the settings panel. | all |
| FR-WT-08, FR-WT-09 | A second window for the same worktree focuses the existing one; titles show worktree and branch. | all |
| QA-PORT-02 | First use of a distro with no core deployed yet, opened via "Open WSL folder": the consent dialog names the exact distro and target path before any copy; declining leaves no file behind and shows `DEPLOY_REFUSED`'s message; accepting opens the repo normally thereafter, same as a local one; git and hooks run inside WSL. With that distro's own editor profile configured, "Open in editor" from that window starts the editor process inside the distro too, not on Windows. | Windows |
| QA-PORT-02 | A WSL repo already open in a window: `wsl --shutdown` (or `wsl --terminate <distro>`) from a Windows terminal, then retry an action (e.g. Refresh) in the still-open window: it recovers without the user closing/reopening it — a brief error is acceptable, but a retry succeeds and the log/summary reload correctly. | Windows |
| QA-PERF-03 | Scroll the 100k-commit log: no visible freeze. | Windows, Linux |
| QA-INST-01 | Start without WebView2 (Windows) or the webkitgtk package (Linux): Windows shows a native message naming WebView2; on Linux WebKitGTK links before any Go code runs, so no in-app message is possible — confirm the failure is visible in the terminal, not silent. Git presence is covered by `TestQA_INST_01_GitPresent`. | Windows, Linux |
| QA-USE-01, QA-USE-02 | An an IDE git user finds each action within two steps, under familiar names. | all |
| QA-MNT-01 | Every spec in the plans dir lists the IDs it covers; the plans dir is outside the repo, so no tool can check it. | — |
| FR-REPO-01 | The native "Open folder…" dialog opens and a picked directory opens as a worktree; component tests only prove the binding is called. | Windows, Linux |
| TR-13, FR-REPO-02 | Quit and relaunch nebegit: recent repos and the windows open at last quit persist across an actual process restart, not just a mocked `Store`. | Windows, Linux |
| QA-PERF-01 | Fresh `task dev` / `bin/nebegit` launch, stopwatch: a usable window appears within 2s. No window-ready signal exists in `internal/app` to automate against; building one is a bigger, separate piece of plumbing. | Windows, Linux |
| FR-CMP-01, FR-CMP-02 | Compare two commits (click one, Ctrl-click another, or right-click a commit and choose "Compare with selected"), then double-click a file: a separate OS window opens, file list left, diff right; next/previous change and file work; the main window stays usable behind it; Escape and its own close button both close it; double-clicking a second file of the same comparison while its window is open focuses and switches it, not a second window. | Windows, Linux |
| ADR-9, FR-WT-05 | With a repo open in a window, select a different worktree in the panel: the window's title and content swap in place, no new OS window; switching away and back shows the previous worktree's state was reloaded fresh, not stale. | Windows, Linux |
| FR-WT-11 | Open two different repos, each once, one worktree of each in its own window: both windows work independently at the same time (log, refs, worktree list all stay correct per window). | Windows, Linux |
| FR-WT-02, FR-WT-03 | The panel's "+ New worktree": create one from an existing branch, then one with a new branch — both succeed and show up in the panel; try a target folder that already has files in it — rejected with git's own clear message. | Windows, Linux |

CI: GitHub Actions (`actions/setup-go`, `actions/setup-node` with pnpm cache, golangci-lint's action). Matrix windows-latest and ubuntu-24.04 (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`). Two steps per job: install dependencies with network; then lint (`golangci-lint`, `oxlint`, `eslint`, `vue-tsc`), tests, build, offline (TR-14). The Linux job runs the Go tests against git 2.40 and the runner's current git. macos-latest: build only (QA-PORT-04).

---
