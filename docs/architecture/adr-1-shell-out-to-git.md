# ADR-1: Shell out to git

Status: accepted

## Context

nebegit must behave exactly like the git command line (G5, QA-COMP-01): the user's config, hooks,
credential helpers, attributes and line-ending rules all apply. It also needs cherry-pick, revert,
rebase, `apply`, stash and linked worktrees.

go-git, the only mature pure-Go implementation, lacks most of these in v5; v6 is alpha. lazygit, the
best-known Go git client, shells out for the same reasons.

## Decision

Every git operation runs the system `git` binary through `os/exec`. No git library, and no hybrid:
two engines would mean two behaviours.

## Consequences

- Behaviour matches the user's terminal by construction, including hooks and credential helpers.
- Output is parsed from machine-readable formats only (TR-08); the environment and arguments follow
  TR-09 and TR-10.
- Each call costs a process start, which is noticeable on Windows. We keep calls few and large,
  stream the log, and read the sizes of several blobs in one `cat-file --batch-check`.
