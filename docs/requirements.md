# Requirements — nebegit


This doc says **what** the app must do. It says nothing about **how**. See `architecture.md` for that.

Priorities: **MUST** = needed for v1. **SHOULD** = wanted, can slip.
Every requirement has an ID. Feature specs and tests refer to these IDs.

---

## Purpose

**nebegit** = Swiss German *näbe* ("beside") + git: the git tool that sits beside your editor.

A desktop GUI for daily git work, used next to a code editor that lacks strong git tooling (first case: the editor).
It fills the gap left after moving from an IDE to the editor: a strong log viewer, commit compare, worktree handling, and safe sync actions.

## Goals

- G1: Cover the daily git tasks that an IDE made easy and the editor lacks.
- G2: Make worktrees easy to see, create, and open in the editor.
- G3: Be fast on large repos (about 100k commits).
- G4: Be safe. No silent data loss.
- G5: Behave exactly like the git command line. No surprises.
- G6: Run on Windows and Linux. On Windows, handle both repos on the Windows file system and repos inside WSL (the owner's common case) equally well.
- G7: Open source. Others can download, use, and contribute.

## Non-goals

- Not an editor. Not a code review tool.
- Not a replacement for every git command.
- No hosting features (pull requests, issues).
- No own credential handling.

## Users and context

- Users: developers who know git well. First user is the owner; the app is published as open source for others.
- Git is installed. The user is already logged in to their remotes through the git command line.
- The owner uses the editor. Other users may use any editor (FR-ED). The app runs beside it.
- A window shows one selected worktree at a time; a selector panel in the window switches it. Several windows can be open.

---

## Requirements

- [functional.md](requirements/functional.md): FR-*, one section per area (REPO, LOG, CMP, DIFF, BR, WT, CHG, SYNC, UPD, CP, RV, CONF, ED, UX).
- [quality.md](requirements/quality.md): QA-*.

## Constraints and assumptions

- Git is installed and on the path.
- The user is already authenticated for their remotes.
- Single user, single machine. One worktree per window.
- UI language: English.
- Development is spec-driven and test-driven, done hands-off by Claude sessions. The owner handles session handovers and critical decisions only. Process and tooling: `architecture.md`, TR-15 to TR-33.

## Out of scope for v1

Clone · built-in merge editor · interactive rebase · stash management (beyond FR-SYNC-09) · tag management · blame · file history view · submodules · LFS-specific features · pull requests / hosting integration · installers · auto-update · code signing.

## Decisions

| Topic | Decision |
|---|---|
| Name | nebegit |
| Licence | BSD-3-Clause |
| Hosting / CI | GitHub + GitHub Actions |
| macOS | Best effort: build only, untested |
| Worktree handling | A selector panel in the window (repos, worktrees, branches); the rest of the window follows the selection. |
| Audience | Open source with release downloads |
| Log actions in v1 | Cherry-pick, revert, check out commit. Reset mode "keep" stays SHOULD. |
| Process | Hands-off by Claude sessions; critical decisions to the owner. |

No open questions. Technical unknowns are spike items in `architecture.md`.
