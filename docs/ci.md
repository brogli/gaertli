# CI

Every pull request runs the `ci` workflow, and it has to be green to merge. `task ci` runs the same
Linux checks locally, offline after `task setup`, except the two e2e suites: `task e2e` and
`task e2e:real`. `task fmt` fixes lint findings and formatting.

`ci` also runs on pushes to `main` and by hand. `release candidate`, started by hand on `main` once
`ci` has passed on that commit, builds every artifact and leaves a draft release;
`docs/releasing.md` has the rest.

## Jobs

| Job             | Proves                                                                                                                                                                    | Locally                                      |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `linux (check)` | Lint, formatting, type check, traceability and licences; then the component tests                                                                                         | `task ci:lint`, `task test:frontend`         |
| `linux (go)`    | The Go tests, on real git                                                                                                                                                 | `task ci:go`                                 |
| `linux (app)`   | The Linux build, the Windows cross-build and installer; then the mock-backend e2e suite (layout and look) and the real-backend suite (git workflows through the real app) | `task ci:build`, `task e2e`, `task e2e:real` |
| `windows`       | The Go tests' Windows-only paths: processes, paths, the WSL core                                                                                                          | On Windows: `go tool task test:go:core`      |
| `macos`         | The app builds on macOS; nothing runs                                                                                                                                     | On a Mac: `task build`                       |

`linux (check)` and `linux (app)` run each check as its own step, so a failed lint still shows
whether the component tests passed: read every red step. A failed e2e step attaches its report and
failure files to the run.

A lint failure "export data version N is greater than maximum supported version" means the Nix
shell's golangci-lint predates the Go in `go.mod`. The next `flake.lock` update brings a newer one.

## Timing

A Linux job takes a few minutes, Windows about twice that when its caches are warm. Only `main`'s
runs save caches, and pull requests restore them, so a run is slower when `main` has had no recent
run. A newer push to the same pull request cancels the run in progress.
