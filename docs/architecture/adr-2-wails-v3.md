# ADR-2: Wails v3

Status: accepted

## Context

The backend is Go (ADR-1, ADR-3). The shell must host a web UI on Windows and Linux, generate TypeScript bindings for Go methods, and open several windows in one process (ADR-9).

## Decision

The shell is Wails v3, pinned to an exact version (TR-02). There is no fallback shell. Wails v3 is the only Go desktop shell with multi-window support, and we accept that it is still in beta.

What we use:

- Services: one bound facade struct (`application.NewService`) with the `ServiceStartup`/`ServiceShutdown` hooks. Bound method types must be JSON-serialisable. Go errors reach TypeScript as rejected promises carrying name, message and cause.
- TypeScript binding generation for the facade.
- Taskfile-based builds.
- Multi-window, per-window events and the single-instance callback.

## Consequences

- Only `internal/app` imports Wails. Every other package is plain Go and builds with `CGO_ENABLED=0` (TR-07).
- Upgrading Wails is a deliberate step: bump the pin, regenerate bindings, run the manual checklist.
- Beta bugs get worked around in `internal/app` or fixed upstream.
- Wails has no headless or browser mode. Frontend end-to-end tests run against the Vite dev server with a mocked Go layer (TR-31).
