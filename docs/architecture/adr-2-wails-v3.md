# ADR-2: Wails v3

**Context.** The backend is Go (ADR-1). The shell must host a web UI on Windows and Linux, generate
TypeScript bindings for Go, and open several windows in one process (ADR-8).

**Decision.** The shell is Wails v3, the only Go desktop shell with multi-window support, pinned to
an exact version. We accept that it is in beta; there is no fallback shell.

**Consequences.** Only the shell package depends on Wails and cgo, so its beta bugs stay contained
there and the core builds as plain Go. A Wails upgrade is a deliberate step.
