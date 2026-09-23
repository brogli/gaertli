# ADR-5: Log and graph are built in Go, the UI renders a window

Status: accepted

## Context

The log must stay fast on repos with about 100k commits (G3, QA-PERF-02/03/07), for WSL repos too, where every byte crosses a pipe (ADR-3).

## Decision

- Go streams `git log`, builds commit records and graph rows, and keeps them in memory. 100k rows of a few hundred bytes each fit the memory budget.
- The UI's virtual list asks Go for rows `[from, to)`. Each row carries its lane data, and the UI draws one small graph strip per row.
- The graph layout is a streaming algorithm over the commit stream (`internal/graph`), so rows are usable while the log is still loading.

## Consequences

- The UI never holds the whole history. Scrolling costs one small call per window of rows.
- Windows that show worktrees of the same repo share one loaded log (ADR-9).
- Filters run as a separate `git log` per window, beside the shared unfiltered log.
