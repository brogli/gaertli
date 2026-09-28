# Requirements — nebegit

What the app must do. How it is built is in `architecture.md`.

Priorities: **MUST** = needed for v1. **SHOULD** = wanted, can slip. Every requirement has an ID.
Task plans and tests refer to these IDs.

---

## Purpose

**nebegit** = Swiss German _näbe_ ("beside") + git: the git tool that sits beside your editor.

A desktop GUI for daily git work, used next to a code editor that lacks strong git tooling (first
case: the editor). It fills the gap left after moving from an IDE to the editor: a strong log view, commit
comparison, worktree handling, and safe sync actions.

## Goals

- G1: Cover the daily git tasks that an IDE made easy and the editor lacks.
- G2: Make worktrees easy to see, create, and open in the editor.
- G3: Be fast on large repos (about 100k commits).
- G4: Be safe. No silent data loss.
- G5: Behave exactly like the git command line. No surprises.
- G6: Run on Windows and Linux. On Windows, handle both repos on the Windows file system and repos
  inside WSL (the owner's common case) equally well.
- G7: Open source. Others can download, use, and contribute.

## Non-goals

- Not an editor. Not a code review tool.
- Not a replacement for every git command.
- No hosting features (pull requests, issues).
- No own credential handling.

## Users and context

- Users: developers who know git well. The first user is the owner, who uses the editor; others may use any
  editor (FR-ED).
- Single user, single machine.
- Git is installed and on the path. The user is already logged in to their remotes through the git
  command line.
- A window shows one selected worktree at a time (FR-WT-05); several windows can be open.
- UI language: English; the UI is translatable (FR-UX-11).
- Development is spec-driven and test-driven, done hands-off by Claude sessions. The owner handles
  session handovers and critical decisions only. Process: the repo's `CLAUDE.md`. Tooling:
  `architecture.md`.

---

## Requirements

- Functional requirements (FR-*), one file per area:
  - [repository.md](requirements/functional/repository.md): REPO
  - [log.md](requirements/functional/log.md): LOG, CMP
  - [diff.md](requirements/functional/diff.md): DIFF
  - [branches.md](requirements/functional/branches.md): BR, UPD
  - [worktrees.md](requirements/functional/worktrees.md): WT
  - [changes.md](requirements/functional/changes.md): CHG, CONF
  - [sync.md](requirements/functional/sync.md): SYNC
  - [cherry-pick-revert.md](requirements/functional/cherry-pick-revert.md): CP, RV
  - [editor.md](requirements/functional/editor.md): ED
  - [ui.md](requirements/functional/ui.md): UX
- [quality.md](requirements/quality.md): QA-*.
- [distribution.md](requirements/distribution.md): `DIST-*`, `OSS-*`.
- [glossary.md](requirements/glossary.md): the domain terms every document, the code and the UI use
  (QA-USE-03).

## Out of scope for v1

Clone · built-in merge editor · interactive rebase (beyond squash and fixup, FR-LOG-14) · stash
management (beyond FR-SYNC-09) · tag management · blame · file history view · submodules ·
LFS-specific features · auto-update · code signing.
