# ADR-3: One core interface, two placements

**Context.** On Windows, repos often live inside a WSL 2 distro (G6), and results must equal the
user's WSL terminal: git, hooks and the editor run inside the distro (QA-PORT-02). Windows git on
`\\wsl$` paths is slow, sees another config and gets no file notifications.

**Decision.** The backend is one Go interface, the _core_. For a local repo it runs inside the
desktop app; for a repo in a WSL distro the same code runs headless in that distro, one per distro,
reached over JSON-RPC on stdio. A repo's path alone decides which core serves a call. The RPC client
and dispatcher are generated from the interface, so a method is written once. The Windows app
carries the Linux core and places it in a distro after asking once.

Rejected: a core process for every repo (serialises local calls for no gain) and `wsl.exe git` per
command (a process start per call, no file watching, path translation in every feature).

**Consequences.** WSL support costs no work per feature. The risk moves to drift between the two
transports, covered by protocol tests without WSL. The API must stay coarse and serialisable. A WSL
path has two forms; the UI uses the Windows form, the core sees only the Linux one.
