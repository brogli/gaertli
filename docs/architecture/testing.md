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
| FR-REPO-06 | `nebegit <path>` from a Windows, Linux, and WSL terminal opens or focuses the right window. | Windows, WSL, Linux |
| FR-REPO-07 | "Open WSL folder" lists distros and folders; the repo opens in a native window. | Windows |
| FR-WT-04, FR-ED-01, FR-ED-02 | Open folder and file in the configured editor from Windows, WSL, and Linux. | all |
| FR-ED-06 | Misconfigured editor: error with a link to the setting. | all |
| FR-WT-08, FR-WT-09 | A second window for the same worktree focuses the existing one; titles show worktree and branch. | all |
| QA-PORT-02 | First use of a distro: consent dialog before the core is copied; git and hooks run inside WSL. | Windows |
| QA-PERF-03 | Scroll the 100k-commit log: no visible freeze. | Windows, Linux |
| QA-INST-01 | Start without WebView2 (Windows) or the webkitgtk package (Linux): Windows shows a native message naming WebView2; on Linux WebKitGTK links before any Go code runs, so no in-app message is possible — confirm the failure is visible in the terminal, not silent. Git presence is covered by `TestQA_INST_01_GitPresent` (spec 04). | Windows, Linux |
| QA-USE-01, QA-USE-02 | An an IDE git user finds each action within two steps, under familiar names. | all |
| QA-MNT-01 | Every spec in the plans dir lists the IDs it covers; the plans dir is outside the repo, so no tool can check it. | — |
| FR-REPO-01 | The native "Open folder…" dialog opens and a picked directory opens as a worktree; component tests only prove the binding is called. | Windows, Linux |
| TR-13, FR-REPO-02 | Quit and relaunch nebegit: recent repos and the windows open at last quit persist across an actual process restart, not just a mocked `Store`. | Windows, Linux |

CI: GitHub Actions (`actions/setup-go`, `actions/setup-node` with pnpm cache, golangci-lint's action). Matrix windows-latest and ubuntu-24.04 (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`). Two steps per job: install dependencies with network; then lint (`golangci-lint`, `eslint`, `vue-tsc`), tests, build, offline (TR-14). The Linux job runs the Go tests against git 2.40 and the runner's current git. macos-latest: build only (QA-PORT-04).

---
