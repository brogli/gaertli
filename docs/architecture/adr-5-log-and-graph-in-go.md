# ADR-5: Log and graph are built in Go

**Status.** Approved.

**Context.** The log must stay fast on repos of about 100k commits (G3, QA-PERF-02/03/07), WSL repos
too, where every byte crosses a pipe (ADR-3).

**Decision.** Go streams `git log`, lays out the graph as the commits arrive, and keeps the rows in
memory. The UI's virtual list asks for the window of rows it shows; each row carries the graph lines
inside it, so rows drawn one by one join into one graph.

**Consequences.** The UI never holds the whole history, and the first rows show while the log still
loads. Windows on one repo share one loaded log (ADR-8).
