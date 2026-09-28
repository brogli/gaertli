# ADR-3: One core interface; in-process locally, JSON-RPC into WSL

Status: accepted

## Context

On Windows, the owner's repos usually live inside a WSL 2 distro (G6). Results there must equal what
the user gets in their WSL terminal: git, hooks and the editor command run inside the distro
(QA-PORT-02), and the window must still be a native Windows window.

Running Windows git against `\\wsl$` paths is slow (every file access crosses the 9p bridge), sees a
different git config, and gets no native file-change notifications.

## Decision

The backend is one Go interface, the **core** (`internal/core.API` plus its event stream).
`internal/services` implements it. It is pure Go with no GUI.

The **shell** (`internal/app`) binds one generated facade to the WebView. Every call on a repo
carries a worktree id or a repo path, and the facade forwards it to the core that owns that repo.
The id or path alone decides, not the window making the call:

| Repo lives on                     | Core used                                                                                  |
| --------------------------------- | ------------------------------------------------------------------------------------------ |
| Windows file system, Linux, macOS | `internal/services`, in-process                                                            |
| WSL 2 distro                      | JSON-RPC client → `wsl.exe -d <distro> -- ~/.cache/nebegit/<version>/nebegit-core --stdio` |

- The shell picks the placement from the path: `\\wsl$\<distro>\…` and `\\wsl.localhost\<distro>\…`
  go to that distro's core, whether they came from the native folder dialog (FR-REPO-07) or from a
  launch inside WSL. Everything else runs in-process.
- One core per distro in use. Sessions (ADR-9) live in the core that owns the repo.
- Events: the in-process core emits them directly; the RPC client turns JSON-RPC notifications into
  the same events.
- The RPC client, the dispatcher and the facade are generated from the interface (TR-28). No
  established generator exists, and dispatch by reflection was rejected because Wails needs a plain
  struct with typed methods to generate bindings.
- Transport: `creachadair/jrpc2` with line framing on stdio and server-pushed notifications. Large
  data moves in bounded pieces (ADR-5).
- Deployment: the Windows download embeds the `linux/amd64` core, so it stays one download
  (DIST-01). On first use of a distro the shell asks a short yes-or-no question (QA-PORT-02), then
  copies the core to `~/.cache/nebegit/<version>/`. The protocol version is checked on connect. If
  the core dies (e.g. `wsl --shutdown`), the next call to that distro starts it again; the shell
  reopens the sessions of that distro's windows, and each window reloads everything it shows.
  Nothing is lost: the core holds no repo state of its own (ADR-4), and a window hands the new core
  the commit message being typed before it reloads.
- Launcher (FR-REPO-06): on Windows and Linux, `nebegit [path]` is the GUI binary. Inside WSL, the
  command is a script beside the Windows install (`build-and-release.md`), reached through the
  Windows user PATH that WSL appends to its own; it converts its argument to a Windows path and
  starts the Windows app through interop, then returns at once. Nothing of the command is placed in
  a distro: the helper core (above) is the only thing the app deploys there, and the consent that
  first-use asks for names only the helper. That consent, and the deploy it guards, come up whenever
  a distro's core is needed: at start for a restored window or a path on that distro, not only from
  "Open folder…". The shell routes a `\\wsl$\<distro>\…` or `\\wsl.localhost\<distro>\…` argument,
  and a UNC working directory from a WSL terminal, to that distro's core the same way it routes a
  path chosen in the folder dialog.
- The editor command runs in the core, so for a WSL repo the editor starts inside WSL.

Alternatives considered:

- A separate core process for every repo, local ones included: not how Wails is meant to be used,
  and it would serialise every local call. The WSL-specific work would exist anyway.
- `wsl.exe git …` per command with no core in WSL: one `wsl.exe` start per call, no native file
  watching across `\\wsl$`, and path translation in every feature.

## Consequences

- The WSL path costs no extra work per feature: a new core method is available over RPC as soon as
  it is generated.
- The risk moves to drift between the two transports. The protocol tests (`testing.md`) cover the
  WSL transport without WSL.
- The API must stay coarse and JSON-serialisable.
- A WSL repo has two forms of every path. In the shell and the UI, a worktree id and every path
  handed to a path-based call are the UNC form (`\\wsl.localhost\<distro>\…`), so each names its
  core. On the way to the distro's core they are translated to its POSIX form, the only form the
  core sees; paths the core returns stay POSIX.
