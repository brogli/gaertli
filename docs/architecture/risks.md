# Risks

| Risk | Impact | Plan |
|---|---|---|
| Wails v3 is in beta | Blocking bugs on one platform, API changes between betas | Exact version pin; Wails code only in `internal/app`; bugs worked around there or fixed upstream. No fallback shell (ADR-2). |
| The RPC path drifts from the in-process path | Bugs that only show for WSL repos | The same integration cases run through both (protocol tests). Keep the API coarse. |
| WSL shuts down or the WSL core dies while a window is open | A dead window | The shell notices the closed pipe, restarts the core and reopens the sessions. Nothing is lost (ADR-4). |
| Process start cost on Windows | A sluggish UI | Few, large calls; stream the log; `cat-file --batch`. |
| Partial-patch bugs | Wrong content staged or discarded | lazygit's pattern, heavy tests against real git; destructive paths reviewed against real repos. |
| Status polling on huge worktrees | CPU use | Poll only while focused, back off when status is slow. Honour the user's `core.fsmonitor` and `core.untrackedCache`, never enable them (QA-SAFE-03). |
| Parsing breaks on a new git version | Wrong data | Machine formats only (TR-08); CI tests against git 2.40 and the current release. |
| OpenVue is a young fork | Bugs, slow fixes | Same API as PrimeVue 4; exact pin; wrap components where cheap. |
| Native Go and Nix C libraries do not link cleanly | Friction in the Linux dev setup | gcc comes from Nix; the shell exports `LD_LIBRARY_PATH` and `PKG_CONFIG_PATH`. Fallback: distro packages for GTK4/WebKitGTK, Nix only for Node and pnpm. |
| The macOS build is untested | Broken macOS releases go unnoticed | Labelled untested. A failing macOS build does not block a release (QA-PORT-04). |
