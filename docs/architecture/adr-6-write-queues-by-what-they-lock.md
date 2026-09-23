# ADR-6: Write queues by what they lock

Status: accepted

## Context

Git protects the index and refs with lock files. Two app actions writing the same index at once would make one of them fail, but a single global queue would let a slow fetch block a commit (QA-RESP-01, FR-UPD-02).

## Decision

Writes are serialised per scope, matching what git itself locks:

| Scope | Serialised | Examples |
|---|---|---|
| Reads | never, they run in parallel | status, log, diff |
| Worktree | one at a time per worktree | stage, commit, reset, checkout, cherry-pick, revert, merge/rebase steps, discard |
| Repo | one at a time per repo | `worktree add/remove/prune`, branch delete/rename |
| Refs and objects only | not queued; git's ref locks suffice | fetch, push, updating a branch that is not checked out |

- Pull is a fetch outside the queues, followed by a merge or rebase in the worktree queue.
- Updating a branch that is checked out in another worktree takes that worktree's queue for its `merge --ff-only`.
- Jobs are not a bypass. A job's work takes the same lock as any other write of its kind.
- If another tool holds a git lock, git fails. The app shows the error and offers a retry.

## Consequences

- Network actions never block local work.
- Reads can observe a write half-done. Views reload after every own write, and the watcher catches the rest (`git-mapping.md`).
