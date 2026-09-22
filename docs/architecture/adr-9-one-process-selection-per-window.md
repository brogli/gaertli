# ADR-9: One process, one selection per window

Requirements: FR-WT-05, FR-WT-08 to FR-WT-11, FR-WT-13. Revised 2026-09-22 (owner decision): a window shows one *selected* worktree and can switch, instead of being bound to one worktree for its lifetime.

- One shell process, many windows. Each window has one selection: a worktree id. The selector panel (FR-WT-13) changes it; the whole window then reloads for the new worktree.
- Two session levels, unchanged:
  - Repo session, one per repo, shared by every window whose selection is one of its worktrees: commit list, graph rows, refs map, worktree list, repo-level queue, git-dir watcher, remote jobs.
  - Worktree session, one per worktree in use (not per window): status, index state, conflict state, worktree write queue, status poller, message draft. Two windows selecting the same worktree share it.
- One process so the log of a 100k-commit repo is loaded once (QA-PERF-07), queues are shared, and FR-WT-10 holds by construction.
- Every service call carries a worktree id; the core resolves it to a session and runs git with `-C <worktree path>`. The UI never sends raw paths for worktree actions. Switching the selection is a shell concern: the window swaps the id its stores use and reloads; the core sees an `OpenWorktree` for the new selection and a `CloseWorktree` for the old one once no window shows it.
- Events go to the windows they concern: repo-level to every window currently selecting one of the repo's worktrees, worktree-level to the windows currently selecting that worktree. The shell keeps the routing table (`Registry`) keyed by the current selection, not by window identity.
- Single instance: a second launch hands its path to the running process, which focuses a window already showing that worktree or, if none, opens a new window on it (FR-REPO-06, FR-WT-08). Wails provides the single-instance callback, per-window events (`window.EmitEvent`), and broadcast events (`app.Event.Emit`).
- A repo session is dropped when no window selects any of its worktrees. Closing the last window quits.
- The panel's own data (recent repos, each repo's worktrees and branches) comes from the settings file (recents) and the repo sessions of the repos it shows; a repo not yet opened in this process gets a session on first expansion.

Consequences: the home window becomes the empty state of the panel, not a separate view. `Registry` maps selections to windows rather than windows to fixed paths. Nothing changes in `core.API`.

---
