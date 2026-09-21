# ADR-2: Wails v3

- v3 is in beta; the desktop API is declared stable during beta. Pinned to an exact version.
- Services: `application.NewService` / `NewServiceWithOptions`, hooks `ServiceStartup` / `ServiceShutdown`. Bound method types must be JSON-serialisable. Go errors reach TS as rejected promises with name, message, cause.
- Why v3: service model, TS binding generation, Taskfile builds, multi-window (ADR-9).
- Risk: beta bugs, mostly on Linux. Mitigation: pin, keep Wails-specific code in `internal/app`. Fallback: v2, which would only change service registration.

