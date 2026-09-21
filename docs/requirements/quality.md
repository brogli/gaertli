# Quality attributes (QA)

### Performance (reference repo: 100k commits, normal dev machine)

| ID | Requirement |
|---|---|
| QA-PERF-01 | App start to usable window: ≤ 2 s. |
| QA-PERF-02 | First log rows visible: ≤ 1 s after opening the repo. |
| QA-PERF-03 | Log scrolling: no freeze longer than 100 ms. |
| QA-PERF-04 | Search or filter: first results ≤ 2 s. |
| QA-PERF-05 | Diff of a file < 1 MB opens in ≤ 500 ms. |
| QA-PERF-06 | Change list refresh: ≤ 1 s. |
| QA-PERF-07 | Memory: ≤ 500 MB with the reference repo open. |

### Responsiveness

| ID | Requirement |
|---|---|
| QA-RESP-01 | The UI never blocks. Anything longer than 300 ms shows a busy state. |
| QA-RESP-02 | Remote actions show progress and can be cancelled. |

### Safety

| ID | Requirement |
|---|---|
| QA-SAFE-01 | No action that can lose work runs without confirmation. |
| QA-SAFE-02 | The app never leaves the repo in a state the git command line cannot handle. |
| QA-SAFE-03 | The app never changes the user's git configuration without being asked. |
| QA-SAFE-04 | A cancelled or crashed action leaves the repo consistent, or the app tells the user how to recover. |

### Compatibility

| ID | Requirement |
|---|---|
| QA-COMP-01 | Results equal what the git command line would produce. The user's git config, hooks, ignore rules, attributes, and line-ending settings are respected. |
| QA-COMP-02 | Works while other git tools (terminal, the editor) use the same repo. |

### Portability

| ID | Requirement |
|---|---|
| QA-PORT-01 | Runs on Windows 10/11 (x64) and mainstream Linux desktops (x64). These are the supported, tested platforms. |
| QA-PORT-04 | macOS: best effort. A build is published with a release if it builds; a failing macOS build does not hold back a release. It is not tested, neither by hand nor automatically. Bugs are fixed when reported, without a deadline. No macOS-only features. |
| QA-PORT-02 | Works with repos stored inside WSL 2. The window is a normal Windows window. Git, hooks, and the editor command run inside WSL, so results equal what the user gets in their WSL terminal (QA-COMP-01). Speed targets (QA-PERF) apply here too, not counting the time WSL itself needs to boot when it was not running. Before the app places its helper program inside the WSL distro, it tells the user. |
| QA-PORT-03 | Same features on all supported platforms. |

### Security and privacy

| ID | Requirement |
|---|---|
| QA-SEC-01 | No telemetry. No network traffic except git talking to the user's remotes. |
| QA-SEC-02 | No credentials are stored or logged. |
| QA-SEC-03 | Text from the repo (branch names, paths, messages) can never trigger unintended commands. |

### Reliability

| ID | Requirement |
|---|---|
| QA-REL-01 | A failed git action never crashes the app. |
| QA-REL-02 | Unexpected repo states (detached HEAD, empty repo, missing worktree folder, shallow clone) are shown, not hidden, and do not break the app. |

### Usability

| ID | Requirement |
|---|---|
| QA-USE-01 | Feels familiar to an an IDE git user: same terms, similar flows. |
| QA-USE-02 | Each wishlist action is at most two steps away from the main window. |

### Maintainability and testability (spec-driven)

| ID | Requirement |
|---|---|
| QA-MNT-01 | Every feature has a spec that lists the requirement IDs it covers. |
| QA-MNT-02 | Every MUST functional requirement is covered by at least one automated test that names its ID. Exceptions are allowed only where automation is not practical (real WSL, real external editor, window focus, scrolling smoothness). Each exception is listed by ID in a manual test checklist. An automated check enforces this. |
| QA-MNT-03 | A test run needs no network and never touches the developer's own repos or git config. Installing dependencies beforehand may use the network; that is setup, not the test run. Tests never contact a real git remote or any external service. |

### Installation and distribution (open source)

| ID | Requirement |
|---|---|
| QA-INST-01 | One download per platform from the project's release page. Prerequisites: git, and the operating system's web view component (preinstalled on Windows 11; a system package on Windows 10 and Linux). The app says clearly at start if one is missing. |
| QA-INST-02 | Releases are built by an automated pipeline (GitHub Actions) from a tagged commit and published as GitHub Releases, with checksums. No hand-built binaries. |
| QA-INST-03 | No installer and no auto-update in v1. The app shows its version. |
| QA-OSS-01 | Source is public on GitHub under **BSD-3-Clause**. All dependencies have compatible licences. |
| QA-OSS-02 | A new contributor can build and run tests with the documented steps only. |
| QA-OSS-03 | User docs: install, first start, editor setup, known limits (WSL, SSH passphrase without agent). |
| QA-OSS-04 | Nothing in the app assumes the owner's setup: no fixed paths, editor, remote host, or branch names. |

---
