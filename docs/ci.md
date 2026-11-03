# CI

Every pull request runs the `ci` workflow, and it has to be green to merge. `task ci` runs the same
Linux checks locally, offline after `task setup`, except the two e2e suites: `task e2e` and
`task e2e:real`. `task fmt` fixes lint findings and formatting.

`ci` also runs on pushes to `main` and by hand. `release candidate`, started by hand on `main`, runs
`ci`, builds every artifact and leaves a draft release; `docs/releasing.md` has the rest.

## Jobs

| Job             | Proves                                                                                                                                                                    | Locally                                                                                                      |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `linux (check)` | Lint, formatting, type check, traceability and licences; then the component tests                                                                                         | `task ci:lint`, `task test:frontend`                                                                         |
| `linux (go)`    | The Go tests, on real git                                                                                                                                                 | `task ci:go`                                                                                                 |
| `linux (app)`   | The Linux build, the Windows cross-build and installer; then the mock-backend e2e suite (layout and look) and the real-backend suite (git workflows through the real app) | `task ci:build`, `task e2e`, `task e2e:real`                                                                 |
| `git-2-42`      | The git-dependent Go tests on the oldest supported git, with the race detector                                                                                            | git 2.42 first on PATH (the workflow takes it from a pinned nixpkgs), then `task test:go:oldgit FLAGS=-race` |
| `windows`       | The Go tests' Windows-only paths: processes, paths, the WSL core                                                                                                          | On Windows: `go tool task test:go:core`                                                                      |
| `macos`         | The app builds on macOS; nothing runs                                                                                                                                     | On a Mac: `task build`                                                                                       |

`linux (check)` and `linux (app)` run each check as its own step, so a failed lint still shows
whether the component tests passed: read every red step. A failed e2e step attaches its report and
failure files to the run.

## Timing

A Linux job takes a few minutes, Windows about twice that when its caches are warm. A pull request's
first run is slower when `main` has had no recent run to leave caches. A newer push to the same pull
request cancels the run in progress.
