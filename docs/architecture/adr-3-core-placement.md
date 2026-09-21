# ADR-3: One core interface; in-process locally, JSON-RPC into WSL

- **Core** = `internal/core.API` plus its event stream. Implementation `internal/services`: sessions, git runner, watcher, jobs, editor launcher. Pure Go, no GUI.
- **Shell** = `internal/app`. It binds one generated facade to the WebView. Each call carries the window's worktree id; the facade forwards it to the core that owns the repo:

| Repo lives | Core used |
|---|---|
| Windows file system, Linux, macOS | `internal/services`, in-process |
| WSL 2 distro | generated RPC client → `wsl.exe -d <distro> -- ~/.cache/nebegit/<version>/nebegit-core --stdio` |

- Selection by path: `\\wsl$\<distro>\…`, `\\wsl.localhost\…`, "Open WSL folder" (FR-REPO-07), or a launch from inside WSL → RPC core in that distro. Everything else → in-process.
- One RPC core per distro in use. Sessions (ADR-9) live in the core that owns the repo.
- Git, file watching, hooks, and the editor command run where the repo lives. inotify works inside WSL.
- Events: the in-process core emits directly; the RPC client turns notifications into the same events.
- Why not a separate core process for every repo: not the Wails pattern, serialises every local call, and the WSL-only work exists either way.
- Why not `wsl.exe git …` per command without a core in WSL: one `wsl.exe` start per call, no native file watching across `\\wsl$`, path translation in every feature.

Generated layers (TR-28): `tools/gencore` is a small `go generate` program on `golang.org/x/tools/go/packages`; no established generator exists. Reflection-based dispatch was rejected because Wails binding generation needs a plain struct with typed methods.

JSON-RPC: `github.com/creachadair/jrpc2` (`channel.Line` for stdio, `Server.Notify` with `AllowPush`). Large data moves in bounded chunks (ADR-5).

Shipping the WSL core: the Windows download embeds the `linux/amd64` `nebegit-core` via `go:embed`, one download (QA-INST-01). On first use of a distro the shell tells the user what it will place where (QA-PORT-02), then copies the core to `~/.cache/nebegit/<version>/`. Version is checked on connect. Linux and macOS builds do not embed it.

Launcher (FR-REPO-06): on Windows and Linux, `nebegit [path]` is the GUI binary. Inside WSL, a settings action "Install `nebegit` command in this distro" links `~/.local/bin/nebegit` to the deployed core, which, given a path instead of `--stdio`, starts the Windows app through interop with distro name and Linux path. Nothing goes on any PATH without that click.

Editor: the core starts the editor command, so for a WSL repo it runs inside WSL (`editor <path>`).

QA-PERF-01 excludes WSL's own boot time.

**M0 spike items**, each with a pass criterion in the spike spec; a failed item is a critical decision for the owner (TR-32):
- `wails3 generate bindings` on the generated facade struct.
- The RPC core started through `wsl.exe`: `git status` call and a watcher event.
- Round-trip time and throughput for a 5k-row log window over the pipe.
- Start the Windows `.exe` from inside WSL.
- Cross-compile the Windows shell from the WSL Nix shell and run it on the host.
- GTK4 + WebKitGTK 6.0 under WSLg (escape hatch: `WEBKIT_DISABLE_DMABUF_RENDERER=1`).
- Multi-window on WebKitGTK (ADR-9).

WSL 1 is out of scope. Wails has no headless or browser mode; frontend e2e runs against the Vite dev server (TR-31).

