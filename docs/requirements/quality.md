# Quality attributes (QA)

## Performance (reference repo: 100k commits, normal dev machine)

| ID         | Requirement                                                                                                                                                                                                                                                                          |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| QA-PERF-01 | App start to usable window: ≤ 2 s.                                                                                                                                                                                                                                                   |
| QA-PERF-02 | First log rows visible: ≤ 1 s after opening the repo.                                                                                                                                                                                                                                |
| QA-PERF-03 | Log scrolling: no freeze longer than 100 ms.                                                                                                                                                                                                                                         |
| QA-PERF-04 | Search or filter: first results ≤ 2 s.                                                                                                                                                                                                                                               |
| QA-PERF-05 | Diff of a file < 1 MB opens in ≤ 500 ms.                                                                                                                                                                                                                                             |
| QA-PERF-06 | Change list refresh: ≤ 1 s.                                                                                                                                                                                                                                                          |
| QA-PERF-07 | Memory: ≤ 500 MB with the reference repo open.                                                                                                                                                                                                                                       |
| QA-PERF-08 | After the log of a repo with 20,000 commits or more has loaded for the first time, the app has git write its commit-graph file (git's own cache for reading history) unless the repo already has one. It does so without asking, after the log shows, and says nothing if git fails. |

## Responsiveness

| ID         | Requirement                                                          |
| ---------- | -------------------------------------------------------------------- |
| QA-RESP-01 | The UI never blocks. Anything longer than 300 ms shows a busy state. |

## Safety

| ID         | Requirement                                                                                         |
| ---------- | --------------------------------------------------------------------------------------------------- |
| QA-SAFE-01 | No action that can lose work runs without confirmation.                                             |
| QA-SAFE-02 | The app never leaves the repo in a state the git command line cannot handle.                        |
| QA-SAFE-03 | The app never changes the user's git configuration without being asked.                             |
| QA-SAFE-04 | A cancelled or crashed action leaves the repo consistent, or the app tells the user how to recover. |

## Compatibility

| ID         | Requirement                                                                                                                                            |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| QA-COMP-01 | Results equal what the git command line would produce. The user's git config, hooks, ignore rules, attributes, and line-ending settings are respected. |
| QA-COMP-02 | Works while other git tools (a terminal, an editor) use the same repo.                                                                                 |
| QA-COMP-03 | The automated tests run against the oldest supported git version and against a current one.                                                            |

## Portability

| ID         | Requirement                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QA-PORT-01 | Runs on Windows 10/11 (x64) and mainstream Linux desktops (x64). These are the supported, tested platforms.                                                                                                                                                                                                                                                                                                                                              |
| QA-PORT-02 | Works with repos stored inside WSL 2, in a normal Windows window: git, hooks and the editor run inside WSL, so results equal the user's WSL terminal (QA-COMP-01), at the QA-PERF speeds once WSL has booted. A distro on WSL 1 is not supported: the app says so and changes nothing there. Before placing its helper in a distro, or a new version of it, the app asks; declining says that repos there need the helper, and the next open asks again. |
| QA-PORT-03 | Same features on all supported platforms.                                                                                                                                                                                                                                                                                                                                                                                                                |
| QA-PORT-04 | macOS: best effort. A release includes a macOS build if it builds; a failing one does not hold back the release. It is not tested, by hand or automatically. Reported bugs are fixed without a deadline. No macOS-only features.                                                                                                                                                                                                                         |

## Security and privacy

| ID        | Requirement                                                                               |
| --------- | ----------------------------------------------------------------------------------------- |
| QA-SEC-01 | No telemetry. No network traffic except git talking to the user's remotes.                |
| QA-SEC-02 | No credentials are stored or logged.                                                      |
| QA-SEC-03 | Text from the repo (branch names, paths, messages) can never trigger unintended commands. |

## Reliability

| ID        | Requirement                                                                                                                                 |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| QA-REL-01 | A failed git action never crashes the app.                                                                                                  |
| QA-REL-02 | Unexpected repo states (detached HEAD, empty repo, missing worktree folder, shallow clone) are shown, not hidden, and do not break the app. |

## Usability

| ID        | Requirement                                                                                                                                                                           |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QA-USE-01 | Feels familiar to a user of an IDE's git tooling: same terms, similar flows.                                                                                                          |
| QA-USE-02 | The everyday actions (open a repo, switch worktree or branch, stage, commit, pull, push, fetch, compare two commits, open in editor) are at most two steps away from the main window. |
| QA-USE-03 | Requirements, architecture, code and UI share one ubiquitous language: each domain concept has one name, its glossary term.                                                           |

## Maintainability and testability

| ID        | Requirement                                                                                                                                                                                                                                                                                                                                                                                        |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QA-MNT-01 | The requirements are the specification: every behaviour is written here before it is built, and every test names the requirement it proves.                                                                                                                                                                                                                                                        |
| QA-MNT-02 | Every MUST functional requirement is covered by at least one automated test that names its ID. Exceptions are allowed only where automation is not practical (real WSL, real external editor, window focus, scrolling smoothness), each listed by ID in a manual test checklist. A requirement written ahead of its test is listed as owed until the test lands. An automated check enforces this. |
| QA-MNT-03 | A test run needs no network: it never contacts a real git remote or any external service, and never touches the developer's own repos or git config. Installing dependencies beforehand may use the network.                                                                                                                                                                                       |
