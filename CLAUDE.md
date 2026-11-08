# Gärtli

Desktop git GUI: Go core + Wails v3 shell + Vue 3 (OpenVue). Developed spec-driven, test-driven,
hands-off by Claude sessions.

Read `docs/requirements.md` (what), `docs/architecture.md` (how it is built) and
`docs/conventions.md` (how its code is written) first, each an index of its folder, then only the
topic files a task needs. `docs/testing.md` is the testing strategy, `docs/ci.md` what the CI
pipelines do, `docs/releasing.md` how builds and releases work.

## Process

- Design decisions rest on research from primary sources (official docs, release notes, source
  code), each finding with a link and a date.
- Work on a branch off `main` and rebase it onto `main` for updates; never merge `main` into it.
  Never commit to `main`. Pushing branches and opening PRs is allowed; releases only when the user
  says so.
- Owner decisions (a new or changed ADR, a new MUST requirement, a release) go to the user.
- PR titles are Conventional Commits, written for users: they become the release notes
  (`docs/conventions/tooling.md`).
- Project knowledge goes into this file or a project skill (`.claude/skills/`), not into Claude's
  private memory. Machine-specific facts and personal workflow go into `CLAUDE.local.md`.

## Docs

- `docs/` holds requirements, architecture, conventions, the testing strategy, the CI pipelines and
  the release process, kept current. Plans, research notes, task lists and logs stay out of the
  repo.
- `docs/requirements/` is the only source of truth for what the app does. Every behaviour a remark,
  a plan or a review settles becomes a row before it is built. Rows read as requirements written by
  people: what the user can do and gets, goal first, current state. UI rows may say how the UI is
  arranged (a toolbar, a picker, a shortcut shown on its button), never library or component names,
  pixel values, log levels, file names, dates or history.
- Cross-cutting qualities are quality attributes in `docs/requirements/quality.md`.
- `docs/architecture/` reads like a design doc written by people: decisions and their reasons,
  structure, UI design, git command shapes. An ADR records only a decision that is costly to
  reverse, conceptually and briefly. No implementation detail (file names below package level,
  per-method notes, mechanical flags, exit codes) and no changelog. Those live in code, tests, doc
  comments and git.
- `docs/conventions/` states each implementation convention's intent, not its details: those live in
  lint and tool configuration and in the code.

## Code

- TDD: failing test first, and a new test must fail on the old code. A test file names the
  requirements its tests prove in one header line, `// Requirements: FR-LOG-01, QA-COMP-01`; test
  names say what the test checks, without IDs.
- Frontend: Vitest component tests first. Playwright only where a component test cannot cover it,
  and the PR says why.
- Tests prove behaviour, never configuration (a flag, a version, an include list). A tooling change
  is checked once by hand on a planted fault, and the PR records the check. `tools/repocheck` holds
  working-tree scans only.
- Comments state facts only: no ticket or plan numbers, no requirement or ADR IDs as labels (the
  test file header aside), no research or review narration.
- Anything found done ugly gets fixed, also outside the current task, in a separate `simplify:`
  commit. Skip files an in-flight worktree edits.
- Rely on OpenVue's standard behaviour. When it misbehaves, remove what of ours defeats it (e.g.
  `@click.stop` inside a Popover) instead of adding a workaround. A UI fix names the expected
  library behaviour and what of ours blocked it.
- Other git tools are inspiration only. Never read copyleft (GPL, AGPL, LGPL) or source-available
  source, only their user docs. Code taken from a permissive project ships its notice and names its
  source in a comment (OSS-05).
- Destructive or history-writing paths (discard, reset, stash, squash, cherry-pick, revert, worktree
  remove) get a correctness review that reproduces with real git in a scratch repo before merging.
- The app supports only the latest git release; the Nix shell provides it, so tests run on it.

## Checks and commits

- `nix develop --command task ci` is the CI (lint, tests, trace, repo checks). `task e2e` runs the
  mock e2e, `task e2e:real` the real-backend e2e.
- `task fmt` fixes lint findings and formatting over the whole repo. `task lint` is the same as a
  check.
- Run checks without a pipe or under `set -o pipefail`, and chain a commit on the check's exit
  status (`task ci && git commit …`), never on an echo.
- Run `task ci` before committing CI or Taskfile changes: CI runs the same tasks, and no test
  restates the workflows.
- Regenerate the core's generated code from `internal/core`: `go run ../../tools/gencore` (from the
  repo root it fails).
- Windows build: `nix develop --command task windows:build`. An exported `GOOS=windows` breaks
  Task's PATH lookup.
- Before a push, `git grep` finds no home paths or personal names, and
  `git log origin/main..HEAD --format='%ae %ce' | sort -u` shows only the GitHub noreply address.

## Claude's shell

- It exports `GIT_EDITOR=true`: a process started from it resolves git's editor to `true`.
- `pkill -f <pattern>` with the pattern in the same command line kills the calling shell.
- A GUI started from a tool call must not inherit the tool's stdout (`< /dev/null > log 2>&1`), or
  the call hangs until the app exits.
