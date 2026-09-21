# Risks

| Risk | Impact | Plan |
|---|---|---|
| Generated RPC adapter drifts from in-process behaviour | WSL-only bugs | The same integration cases run through both implementations (protocol tests). Keep the API coarse. |
| WSL shuts down or the WSL core dies while a window is open | Dead window | The shell detects pipe close, shows "reconnect", restarts the core. No state is lost (ADR-4). |
| Native Go and Nix C libraries do not link cleanly | Linux dev-setup friction | gcc from Nix; `LD_LIBRARY_PATH` and `PKG_CONFIG_PATH` exported by the shell. Fallback: distro packages for GTK4/WebKitGTK 6.0, Nix only for Node/pnpm. |
| Multi-window in Wails v3 misbehaves on WebKitGTK | One window per worktree is shaky on Linux | M0 spike. Fallback: one process per window; sessions unchanged, shared cache lost. |
| macOS build is untested | Broken macOS releases go unnoticed | Labelled untested. A failing macOS build does not block a release (QA-PORT-04). |
| Wails v3 beta bugs | Blockers on one platform | Pin version, thin Wails layer, v2 fallback (ADR-2). |
| Process start cost on Windows | Sluggish UI | Batch calls, stream the log, `cat-file --batch`. Measured in M1. |
| Partial-patch bugs | Wrong content staged | Follow lazygit's pattern, heavy unit tests, verify with `git diff --cached` after apply. |
| Status polling on huge work trees | CPU use | Adaptive interval, only while focused. Honour the user's `core.fsmonitor` / `untrackedCache`; never enable them (QA-SAFE-03). |
| OpenVue is a young fork | Bugs, slow fixes | API equals PrimeVue 4. Pin version. Wrap OpenVue components where cheap. |
| Parsing breaks on a new git version | Wrong data | Machine formats only (TR-08). CI tests against git 2.40 and current. |

---
