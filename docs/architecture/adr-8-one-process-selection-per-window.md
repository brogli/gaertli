# ADR-8: One process, one selection per window

**Status.** Approved.

**Context.** Users work in several worktrees of one repo at once (FR-WT-05, FR-WT-08 to FR-WT-11).
Loading a 100k-commit log per window would break the memory budget (QA-PERF-07), and two windows
writing one worktree must not race.

**Decision.** One process, many windows. Each main window selects one worktree and can switch. State
lives in sessions in the core: a repo session shared by every window on the repo (the log, the repo
queue, the watcher, remote jobs) and a worktree session shared by the windows on that worktree (its
queue, status polling, the commit message draft, unsaved merge results). A session closes when no
window needs it. A second launch hands its path to the running process.

**Consequences.** A repo's log loads once however many windows show it. Windows on one worktree
share its queue, so their writes never race. Events go only to the windows they concern.
