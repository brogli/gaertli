---
name: agent-work
description: Use when spawning an agent for nebegit work (implementer, reviewer, spec drafter, cleanup) or merging an agent's worktree into dev. Holds the rules every agent prompt must carry, how to watch agents, and the merge flow with its usual rebase conflicts.
---

# Agent work

## Choosing agents

- Sonnet by default, each in an isolated worktree; opus for reviews and cleanups. At most three at once.
- Small tasks (about 7 minutes of work); split bigger ones or promote them to opus.
- Serial, environment-bound work (Nix shell, tool versions, the desktop app) stays in the main session.

## Every agent prompt carries

- **Start:** an Agent-tool worktree begins at the repo's initial commit. First step: `git reset --hard dev`, then confirm `git log -1` shows the dev tip.
- **Every temporary rule of the session**, and which files the agent may edit. An agent that lacks a rule breaks it in good faith (one edited `docs/` while another session rewrote it).
- **No subagents:** implementers never spawn subagents or forks, also not for `/simplify` (one did, and its subagents edited the worktree). Exception: a cleanup round's `/simplify` agents may spawn one level when the owner allows it; they tell their subagents to report to them by name.
- **Commits:** one per task, at least every 10–15 minutes. Checks run without a pipe; the commit is chained on the check's own exit status.
- **Git safety:** run git only inside the worktree. An empirical git check uses a scratch repo and `git -C <scratch>` for every call, never `cd` (a failed `cd` once ran a cherry-pick in the real checkout).
- **Report:** send the final report with SendMessage to "main", or it may never arrive. Say that forwarded messages are not requests: an agent that thinks it was asked for an early report stops with its work uncommitted.
- **Drafters:** a hard time box: full draft in about 12 minutes, done in 20–25, at most 5 web fetches; unverified facts become open research items. An unboxed drafter researched for hours and wrote nothing.
- **Other tools:** the licence rule from `CLAUDE.md` when the task touches another tool's behaviour.

## Watching agents

- No commits and no running processes for about 45 minutes: stuck on a blocked tool call. Stop it and re-spawn it smaller, or finish inline.
- After a usage-limit stop, resume the agent with SendMessage (it keeps its context); check its worktree for partial commits first.
- An agent often closes right after its report, before a reply reaches it: check its worktree at the next look and resend if nothing moved (a send to a closed agent resumes it).
- A named agent's own subagents report to the main session, not to it: forward their reports.
- Review rounds on a destructive path: resume the same implementer and the same correctness reviewer until a pass finds nothing. A behaviour a review settles becomes a requirement row before the fix round.

## Cleanup rounds

After every 3–5 merged specs, or when the owner asks: a repo-wide slop and simplify pass, behaviour unchanged, one commit per area. Slop is narrating comments (spec numbers, research citations, review history), duplicated test helpers, single-use wrappers, copy-pasted mocks, wordy user strings.

- One opus agent per area: frontend, Go and `docs/` in parallel, then the Wails shell and the Go–frontend seam last (it needs the others merged). First a deslop round, then a `/simplify` round in the same order.
- Give the seam one seam-wide opus review first; slice by files only for areas with new code. Per-file subagents found almost nothing in areas already covered twice.
- Skip files an in-flight worktree edits; do them in a follow-up pass.
- Findings that would change behaviour, the settings file or the protocol go to a triage file in the plans dir, not into the cleanup. Bugs found join the next fix batch.

## Merging a worktree

1. Rebase the branch onto `dev`. The usual conflicts:
   - `*_gen.go`: resolve `internal/core/api.go`, then regenerate (`cd internal/core && go run ../../tools/gencore`).
   - Test fakes: "both added", keep both. `internal/app/facade_test.go`'s `stubCore` and `internal/remote/remote_test.go`'s `fake` need stubs for new API methods.
   - `frontend/src/locales/en.json`, `internal/core/errorkeys.go` and `tools/trace/pending.txt` (specs add error keys and pending IDs): run `python3 .claude/skills/agent-work/resolve-key-conflicts.py` from the worktree root; resolve by hand what it reports as UNHANDLED, and rerun `go run ./tools/trace` to settle `pending.txt`.
   - Run the suites only once `git status` says the rebase is done.
2. In the worktree: `nix develop --command task ci`, then `task e2e`.
3. A read-only review: correctness first for anything that writes the index, stash, branches or worktrees (reproduced with real git in a scratch repo), then reuse, simplification and comments. Apply the findings through one fix agent; rerun ci.
4. On `dev`: `git merge --ff-only <branch>`; `git worktree unlock` (a finished agent may leave its lock), `git worktree remove --force`, delete the branch.
5. In the plans dir: delete the spec, remove its roadmap row and raise the done count. A check only the owner can do goes into `notes/manual-checks.md` "For you", but not while the owner is testing.

Agent worktrees live under `.claude/worktrees/` (gitignored). Tools that walk the source tree skip hidden directories. Never `git add -A` from the main checkout while one exists.
