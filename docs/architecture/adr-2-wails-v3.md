# ADR-2: Wails v3

Status: accepted

## Context

The backend is Go (ADR-1, ADR-3). The shell must host a web UI on Windows and Linux, generate TypeScript bindings for Go methods, and open several windows in one process (ADR-9).

## Decision

The shell is Wails v3, pinned to an exact version (TR-02). There is no fallback shell. Wails v3 is the only Go desktop shell with multi-window support, and we accept that it is still in beta.

What we use:

- Bound services: the generated facade and the shell's own services. Their method types must be JSON-serialisable; a Go error reaches TypeScript as a rejected promise.
- TypeScript binding generation for the bound services and the registered events.
- Taskfile-based builds.
- Multi-window, per-window events and the single-instance callback.

## Consequences

- Only `internal/app` imports Wails. Every other package is plain Go and builds with `CGO_ENABLED=0` (TR-07).
- Upgrading Wails is a deliberate step: bump the pin, regenerate bindings, run the manual checklist.
- Beta bugs get worked around in `internal/app` or fixed upstream.
- Wails' server mode (`-tags server`) serves the UI to a browser over HTTP. nebegit uses it only for the real-backend e2e suite and never ships it (TR-31).
