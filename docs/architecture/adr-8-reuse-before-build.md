# ADR-8: Reuse before build

Status: accepted

## Context

nebegit is built by a small team of sessions and must stay maintainable as open source (G7). Every
line of own code is code to test and maintain.

## Decision

For every non-trivial problem we look for an established library or proven pattern first, and write
our own code only when nothing fits (TR-17 to TR-19).

Choices with a reason worth keeping:

- File watching: `fsnotify/fsnotify`. It has no recursive watch, so the git dir is watched and the
  worktree status polled (`git-mapping.md`).
- Unified diffs: `bluekeyes/go-gitdiff`, for parsing only; the partial patches for hunk and line
  staging are own code (ADR-7).
- Editor command templates: an own splitter, since `google/shlex` and `kballard/go-shellquote` are
  unmaintained.
- UI components: OpenVue 1.0, the continuation of PrimeVue 4.5 with the same API.
- Preferred when the need arises: `golang.org/x/sync` (`errgroup`, `singleflight`) for parallel
  work, `google/go-cmp` for test assertions, `testing/synctest` for concurrent tests,
  `rogpeppe/go-internal/testscript` for script-style integration tests.

## Consequences

- Own code exists only where no library fits, each for a stated reason: the graph layout and
  `tools/gencore` (no maintained library), the partial-patch builder, the command splitter, and the
  job manager (small; `context` does most of the work).
- Code taken from another project comes only from permissively licensed ones, ships their notice
  (`tools/licensecheck` adds it to `THIRD-PARTY-NOTICES.txt`, OSS-05) and names its source in a
  comment. The graph layout and the partial-patch builder are adapted from lazygit (MIT); the build
  and packaging scripts start from Wails's build template (MIT).
