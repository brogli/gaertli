# ADR-6: Write queues by what they lock

- Read commands run in parallel.
- Per-worktree queue: everything that writes that worktree's index or files (stage, commit, reset, checkout, cherry-pick, revert, merge/rebase steps, discard). One at a time per worktree.
- Outside the queues: fetch, push, and "update other branch" case 1 (`git-mapping.md`). They touch refs and objects only; git's ref locks suffice. A slow fetch never blocks a commit (QA-RESP-01, FR-UPD-02).
- Pull = fetch (outside) then merge/rebase (worktree queue).
- FR-UPD case 2 uses the other worktree's queue for `merge --ff-only`.
- Repo-wide one-offs (`worktree add/remove/prune`, branch delete/rename) use a per-repo queue.
- Locks held by other tools make git fail; the app shows the error and offers retry.

