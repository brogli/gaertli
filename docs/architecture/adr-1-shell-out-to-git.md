# ADR-1: Shell out to git

**Context.** Gärtli must behave exactly like the git command line (G5, QA-COMP-01): the user's
config, hooks, credential helpers and attributes all apply. No pure-Go git library covers rebase,
cherry-pick, `apply` and linked worktrees.

**Decision.** Every operation runs the system `git` binary. No git library, and no hybrid: two
engines would mean two behaviours.

**Consequences.** Behaviour matches the user's terminal by construction. Output is parsed from git's
machine-readable formats. Each call costs a process start, notable on Windows, so calls are few and
large.
