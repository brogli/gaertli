---
name: agent-work
description:
  Use when spawning an agent for nebegit work (implementer, reviewer, plan drafter, cleanup) or
  merging an agent's worktree into the current feature branch. Holds the rules every agent prompt
  must carry, how to watch agents, and the merge flow with its usual rebase conflicts.
---

# Agent work

## Choosing agents

- Each agent in an isolated worktree; opus for reviews and cleanups. At most three at once.
- Serial, environment-bound work (Nix shell, tool versions, the desktop app) stays in the main
  session.

## Every agent prompt carries

- **Start:** an Agent-tool worktree begins at the repo's initial commit. First step:
  `git reset --hard <feature branch>` (the main checkout's current branch, named in the prompt),
  then confirm `git log -1` shows its tip. A fresh worktree has no `node_modules` and no frontend
  bindings: `cd frontend && pnpm install --frozen-lockfile` inside `nix develop`, then
  `nix develop --command task common:generate:bindings`, before Vitest.
- **Every temporary rule of the session**, and which files the agent may edit: an agent that lacks a
  rule breaks it in good faith.
- **No subagents:** implementers never spawn subagents or forks, also not for `/simplify`.
  Exception: a cleanup round's `/simplify` agents may spawn one level when the owner allows it; they
  tell their subagents to report to them by name.
- **Commits:** one per task, at least every 10–15 minutes.
- **Git safety:** run git only inside the worktree. An empirical git check uses a scratch repo and
  `git -C <scratch>` for every call, never `cd`: a failed `cd` leaves the command running in the
  real checkout.
- **Report:** send the final report with SendMessage to "main", or it may never arrive. Say that
  forwarded messages are not requests: an agent that thinks it was asked for an early report stops
  with its work uncommitted.
- **Drafters:** a hard time box: full draft in about 12 minutes, done in 20–25, at most 5 web
  fetches; unverified facts become open research items.
- **Other tools:** the licence rule from `CLAUDE.md` when the task touches another tool's behaviour.

## Watching agents

- No commits and no running processes for about 45 minutes: stuck on a blocked tool call. Stop it
  and re-spawn it smaller, or finish inline.
- After a usage-limit stop, resume the agent with SendMessage (it keeps its context); check its
  worktree for partial commits first.
- An agent often closes right after its report, before a reply reaches it: check its worktree at the
  next look and resend if nothing moved (a send to a closed agent resumes it).
- A named agent's own subagents report to the main session, not to it: forward their reports.
- Review rounds on a destructive path: resume the same implementer and the same correctness reviewer
  until a pass finds nothing.

## Cleanup rounds

After every 3–5 merged plans, or when the owner asks: a repo-wide slop and simplify pass, behaviour
unchanged, one commit per area. Slop is narrating comments (plan numbers, research citations, review
history), duplicated test helpers, single-use wrappers, copy-pasted mocks, wordy user strings.

- One opus agent per area: frontend, Go and `docs/` in parallel, then the Wails shell and the
  Go–frontend seam last (it needs the others merged). First a deslop round, then a `/simplify` round
  in the same order.
- Give the seam one seam-wide opus review first; slice by files only for areas with new code.
  Per-file subagents found almost nothing in areas already covered twice.
- Findings that would change behaviour, the settings file or the protocol go to a triage file in the
  plans dir, not into the cleanup. Bugs found join the next fix batch.
- A repo pass the owner asks for (pruning tests, docs or requirements; moving tests between levels)
  is planned by one opus agent per pass, which writes a task plan sized into sonnet tasks; sonnet
  agents implement it. Never hand a whole pass to one implementer.

## Merging a worktree

1. Rebase the branch onto the current feature branch. The usual conflicts:
   - `*_gen.go`: resolve `internal/core/api.go`, then regenerate
     (`cd internal/core && go run ../../tools/gencore`).
   - Test fakes: "both added", keep both. `internal/app/facade_test.go`'s `stubCore` and
     `internal/remote/remote_test.go`'s `fake` need stubs for new API methods.
   - `frontend/src/locales/en.json`, `internal/core/errorkeys.go` and `tools/trace/pending.txt`
     (plans add error keys and pending IDs): run
     `python3 .claude/skills/agent-work/resolve-key-conflicts.py` from the worktree root; resolve by
     hand what it reports as UNHANDLED, and rerun `go run ./tools/trace` to settle `pending.txt`.
   - Run the suites only once `git status` says the rebase is done.
2. In the worktree: `nix develop --command task ci`, then `task e2e`, and `task e2e:real` for any
   change to the UI or a git path (its tests read labels and controls the mock suite does not).
3. A read-only review: correctness first for anything that writes the index, stash, branches or
   worktrees, then reuse, simplification and comments. Apply the findings through one fix agent;
   rerun ci.
4. On the feature branch: `git merge --ff-only <branch>`; `git worktree unlock` (a finished agent
   may leave its lock), `git worktree remove --force`, delete the branch.
5. In the plans dir: delete the plan, remove its roadmap row and raise the done count. A check only
   the owner can do goes into `notes/manual-checks.md` "For you", but not while the owner is
   testing.

Agent worktrees live under `.claude/worktrees/` (gitignored). Tools that walk the source tree skip
hidden directories. Never `git add -A` from the main checkout while one exists.
