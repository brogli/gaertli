# ADR-9: One process, one window per worktree

Requirements: FR-WT-05, FR-WT-08 to FR-WT-11.

- One shell process, many windows. Each window is bound to one worktree path for its lifetime.
- Two session levels:
  - Repo session, one per repo, shared by its windows: commit list, graph rows, refs map, worktree list, repo-level queue, git-dir watcher, remote jobs.
  - Worktree session, one per window: status, index state, conflict state, worktree write queue, status poller, message draft.
- One process so the log of a 100k-commit repo is loaded once (QA-PERF-07), queues are shared, and FR-WT-10 holds by construction.
- Every service call carries the window's worktree id; the core resolves it to a session and runs git with `-C <worktree path>`. The UI never sends raw paths for worktree actions.
- Events go to the windows they concern: repo-level to all windows of the repo, worktree-level to one.
- Single instance: a second launch hands its path to the running process, which opens or focuses the window (FR-WT-08). Wails provides the single-instance callback, per-window events (`window.EmitEvent`), and broadcast events (`app.Event.Emit`).
- Closing the last window of a repo drops its repo session. Closing the last window quits.

---
