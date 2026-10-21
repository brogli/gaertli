# ADR-6: Write queues by what they lock

**Context.** Git protects the index and refs with lock files: two writes to one index at once make
one fail. One global queue would let a slow fetch block a commit (QA-RESP-01).

**Decision.** Writes queue by what git locks. Reads never wait. Writes to a worktree's index and
files queue per worktree, writes to a repo's shared state (worktrees, branch rename and delete) per
repo. Ref-only writes (fetch, push, updating a branch checked out nowhere) do not queue; git's ref
locks suffice. A pull is a fetch outside the queues and then a merge or rebase in the worktree's.

**Consequences.** Network actions never block local work. Reads can see a write half-done, so views
reload after every own write. A lock held by another tool fails as git fails, and the app offers a
retry.
