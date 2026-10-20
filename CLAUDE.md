# nebegit

Desktop git GUI: Go core + Wails v3 shell + Vue 3 (OpenVue). Developed spec-driven, test-driven,
hands-off by Claude sessions.

Read in this order: `docs/requirements.md` (what; index of `docs/requirements/`),
`docs/architecture.md` (how; index of `docs/architecture/`), then `HANDOVER.md` in the plans dir
(session protocol, roadmap, task plans; path in `CLAUDE.local.md`). Load only the topic files a task
needs.

## Process

- No feature code without an approved task plan in the plans dir. Task plans are self-approved after
  the template checklist; critical decisions (listed in HANDOVER.md) go to the owner. Otherwise stay
  hands-off.
- A task plan is written only after the research it depends on exists in the plans dir's
  `research/`. Research uses primary sources (official docs, release notes, source code); each
  finding has a link and a date.
- Commit messages name the task plan as `plan NN`.
- PR titles are written for users: GitHub's generated release notes list them.
- Work on a branch off `main` (`dev` for now; later feature branches) and merge `main` into it to
  take in dependency updates. Never commit to `main`. Push only when the owner asks. No release
  until the owner says so.
- "Progress %" means roadmap task plans done out of all task plans, not a session's to-do list.
- Project knowledge goes into this file or a project skill (`.claude/skills/`), not into Claude's
  private memory. Machine-specific facts go into `CLAUDE.local.md`.
- Skills: `agent-work` (spawning agents, merging their worktrees), `desktop-check` (checking the
  real app).

## Docs

- `docs/` holds only requirements and architecture; keep them current. Everything throwaway (task
  plans, research notes, task lists, logs) lives in the plans dir.
- `docs/requirements/` is the only source of truth for what the app does. Rows read as generic,
  goal-oriented requirements written by people: what the user can do and gets, goal first, current
  state. Every behaviour an owner remark, a task plan or a review round settles becomes a row before
  it is built. UI rows (`ui.md` and each area's UI rows) may say how the UI is arranged: a toolbar,
  a picker, a splitter, a shortcut shown on its button. No library or component names, pixel values,
  log levels, file names, dates or history.
- `docs/architecture/` reads like a design doc written by people: decisions and their reasons,
  structure, UI design, git command shapes. No implementation detail (file names below package
  level, per-method notes, mechanical flags, exit codes): that lives in code, tests and doc
  comments. No changelog: history lives in git.
- Traps the code does not make obvious go into the plans dir's `reference/gotchas.md`, one line
  each.

## Code

- TDD: failing test first; a new test must fail on the old code. Test names carry requirement IDs:
  Go `TestFR_LOG_01_...`, Vitest `it('FR-LOG-01 ...')`.
- Frontend tests: Vitest component tests first; Playwright only where a component test cannot cover
  it, and the task plan says why.
- Tests prove behaviour, never restate configuration (a flag, a version, an include list). A tooling
  change is checked once by hand on a planted fault (e.g. `treefmt --ci` on an unformatted file and
  a lint error), recorded in its task plan; no permanent test of the tool config. `tools/repocheck`
  holds working-tree scans only.
- Comments state facts only: no task plan numbers, no requirement, ADR or TR IDs as labels, no
  research or review narration.
- Anything found done ugly gets fixed, also outside the current task, in a separate `simplify:`
  commit; skip files an in-flight worktree edits.
- Rely on OpenVue's standard behaviour. When it misbehaves, remove what of ours defeats it (e.g.
  `@click.stop` inside a Popover); no workaround layers on top. A UI fix's task plan or prompt names
  the expected library behaviour and what of ours blocks it.
- Other git tools are inspiration only, never a template. Never read copyleft (GPL, AGPL, LGPL) or
  source-available source; for such tools only their user docs. Code taken from a permissive project
  ships its notice and names its source in a comment (OSS-05).
- Destructive or history-writing paths (discard, reset, stash, squash, cherry-pick, revert, worktree
  remove) get a correctness review that reproduces with real git in a scratch repo before merging.
- The machine's git may be older than CI's: test anything that rewrites history or parses git output
  against a current git too (`nix build nixpkgs#git --print-out-paths`, its `bin` first on PATH).
  The app needs git 2.42 or newer.

## Checks and commits

- `nix develop --command task ci` is the CI (lint, tests, trace, repo checks); `task e2e` runs the
  mock e2e, `task e2e:real` the real-backend e2e.
- `task fmt` (treefmt) runs every linter with its fixes, then every formatter, over the whole repo;
  `task lint` is the same as a check that changes nothing.
- Run checks without a pipe or under `set -o pipefail`, and chain a commit on the check's own exit
  status (`task ci && git commit …`), never on an echo.
- Run `task ci` before committing CI or Taskfile changes: CI runs the same tasks, and no test
  restates the workflows.
- Regenerate the core's generated code with `cd internal/core && go run ../../tools/gencore` (from
  the repo root it fails).
- Windows build: `nix develop --command task windows:build` (an exported `GOOS=windows` breaks
  Task's PATH lookup).
- Before a push: `git log origin/dev..dev --format='%ae %ce' | sort -u` shows only the GitHub
  noreply address, and `git grep` finds no home paths or personal names.

## Claude's shell

- It exports `GIT_EDITOR=true`: a process started from it resolves git's editor to `true`.
- `pkill -f <pattern>` with the pattern in the same command line kills the calling shell.
- A GUI started from a tool call must not inherit the tool's stdout (`< /dev/null > log 2>&1`), or
  the call hangs until the app exits.
