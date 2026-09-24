# Requirements — nebegit

What the app must do. How it is built is in `architecture.md`.

Priorities: **MUST** = needed for v1. **SHOULD** = wanted, can slip.
Every requirement has an ID. Feature specs and tests refer to these IDs.

---

## Purpose

**nebegit** = Swiss German *näbe* ("beside") + git: the git tool that sits beside your editor.

A desktop GUI for daily git work, used next to a code editor that lacks strong git tooling (first case: the editor).
It fills the gap left after moving from an IDE to the editor: a strong log view, commit comparison, worktree handling, and safe sync actions.

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

- Users: developers who know git well. The first user is the owner, who uses the editor; others may use any editor (FR-ED).
- Single user, single machine.
- Git is installed and on the path. The user is already logged in to their remotes through the git command line.
- A window shows one selected worktree at a time; the repo and worktree pickers in its toolbar switch it. Several windows can be open.
- UI language: English; the UI is translatable (FR-UX-11).
- Development is spec-driven and test-driven, done hands-off by Claude sessions. The owner handles session handovers and critical decisions only. Process and tooling: `architecture.md`, TR-15 to TR-33.

---

## Requirements

- [functional.md](requirements/functional.md): FR-*, one section per area (REPO, LOG, CMP, DIFF, BR, WT, CHG, SYNC, UPD, CP, RV, CONF, ED, UX).
- [quality.md](requirements/quality.md): QA-*.
- [glossary.md](requirements/glossary.md): the domain terms every document, the code and the UI use (QA-USE-03).

## Out of scope for v1

Clone · built-in merge editor · interactive rebase (beyond squash and fixup, FR-LOG-14) · stash management (beyond FR-SYNC-09) · tag management · blame · file history view · submodules · LFS-specific features · installer programs · auto-update · code signing.
