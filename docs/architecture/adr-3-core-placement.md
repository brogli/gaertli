# ADR-3: One core interface; in-process locally, JSON-RPC into WSL

Status: accepted

## Context

On Windows, the owner's repos usually live inside a WSL 2 distro (G6). Results there must equal what the user gets in their WSL terminal: git, hooks and the editor command run inside the distro (QA-PORT-02), and the window must still be a native Windows window.

Running Windows git against `\\wsl$` paths is slow (every file access crosses the 9p bridge), sees a different git config, and gets no native file-change notifications.

## Decision

The backend is one Go interface, the **core** (`internal/core.API` plus its event stream). `internal/services` implements it. It is pure Go with no GUI.

The **shell** (`internal/app`) binds one generated facade to the WebView. Every call carries the window's worktree id, and the facade forwards it to the core that owns that repo:

| Repo lives on | Core used |
|---|---|
| Windows file system, Linux, macOS | `internal/services`, in-process |
| WSL 2 distro | JSON-RPC client → `wsl.exe -d <distro> -- ~/.cache/nebegit/<version>/nebegit-core --stdio` |

- The shell picks a placement from the path: `\\wsl$\<distro>\…`, `\\wsl.localhost\…`, "Open WSL folder" (FR-REPO-07), or a launch from inside WSL go to that distro's core. Everything else runs in-process.
- One core per distro in use. Sessions (ADR-9) live in the core that owns the repo.
- Events: the in-process core emits them directly; the RPC client turns JSON-RPC notifications into the same events.
- The RPC client, the dispatcher inside `nebegit-core` and the shell's facade are generated from the interface by `tools/gencore`, so a method is written once (TR-28). No established generator exists. Dispatch by reflection was rejected because Wails needs a plain struct with typed methods to generate bindings.
- Transport: `creachadair/jrpc2` with line framing on stdio and server-pushed notifications. Large data moves in bounded chunks (ADR-5).
- Deployment: the Windows download embeds the `linux/amd64` core, so it stays one download (QA-INST-01). On first use of a distro the shell says what it will copy where (QA-PORT-02), then copies it to `~/.cache/nebegit/<version>/`. The protocol version is checked on connect. If the core dies (e.g. `wsl --shutdown`), the shell starts it again and reopens the affected sessions. Nothing is lost, because the core holds no state of its own (ADR-4).
- Launcher (FR-REPO-06): on Windows and Linux, `nebegit [path]` is the GUI binary. Inside WSL, a settings action installs `~/.local/bin/nebegit` as a link to the deployed core; given a path instead of `--stdio`, it starts the Windows app through interop. Nothing goes on any `PATH` without that click.
- The editor command runs in the core, so for a WSL repo the editor starts inside WSL.

Alternatives considered:

- A separate core process for every repo, local ones included: not how Wails is meant to be used, and it would serialise every local call. The WSL-specific work would exist anyway.
- `wsl.exe git …` per command with no core in WSL: one `wsl.exe` start per call, no native file watching across `\\wsl$`, and path translation in every feature.

## Consequences

- The WSL path costs no extra work per feature: a new core method is available over RPC as soon as it is generated.
- The risk moves to drift between the two transports. A slice of the integration suite runs a second time through the RPC client against a real `nebegit-core --stdio` child (`testing.md`), so the WSL transport is tested without WSL.
- The API must stay coarse and JSON-serialisable.
- A spike validated the transport before the WSL work started: core start through `wsl.exe`, round-trip time and throughput, starting the Windows `.exe` from inside WSL, and the cross-compiled Windows shell. WSL 1 is out of scope. The performance targets exclude WSL's own boot time (QA-PERF-01).
