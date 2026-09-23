# ADR-9: One process, one selection per window

Status: accepted

## Context

Users work with several worktrees of the same repo at once (FR-WT-05, FR-WT-08 to FR-WT-11), and move between them from a selector panel (FR-WT-13). Loading the log of a 100k-commit repo once per window would break the memory budget (QA-PERF-07). Two windows writing the same worktree must not race (FR-WT-10).

## Decision

- One shell process, many windows. Each window has one selection, a worktree id. The selector panel changes it and the whole window then reloads for the new worktree.
- Sessions live in the core, at two levels:
  - A **repo session**, one per repo, shared by every window that selects one of its worktrees: commit list, graph rows, ref map, worktree list, the repo-level write queue, the git-dir watcher, remote jobs.
  - A **worktree session**, one per worktree in use (not per window): status, conflict state, the worktree write queue, the status poller, the commit message draft. Two windows selecting the same worktree share it.
- Every core call carries a worktree id. The core resolves it to a session and runs git with `-C <worktree path>`. The UI never sends raw paths for worktree actions.
- Switching the selection is a shell concern. The window swaps the id its stores use and reloads. The core sees an open for the new worktree, and a close for the old one once no window shows it.
- Events go only to the windows they concern: repo-level events to every window selecting one of the repo's worktrees, worktree-level events to the windows selecting that worktree.
- Single instance: a second launch hands its path to the running process, which focuses a window already showing that worktree or opens a new one (FR-REPO-06, FR-WT-08).
- A repo session is dropped when no window selects any of its worktrees. Closing the last window quits.
- The selector panel reads recent repos from settings and each repo's worktrees and branches from its repo session. A repo not yet open in this process gets a session when the panel first expands it.

## Consequences

- A repo's log is loaded once, however many windows show it.
- FR-WT-10 holds by construction: all windows share one write queue per worktree.
- There is no separate home window; the empty state of the selector panel takes its place.
