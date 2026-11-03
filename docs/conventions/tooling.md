# Tooling

- **One development shell.** Everything except Go comes from a pinned Nix shell, so every checkout
  and CI's Linux jobs use the same tools (QA-MNT-04). Go and its tools come from `go.mod`. Binaries
  built in the shell use its libraries and run only inside it. Native Windows development is not
  covered; develop inside WSL.
- **One package manager per language**, with one committed lockfile.
- **One command formats and lints everything**, the same way locally, in the editor and in CI.
- **Every language is linted.** A rule a linter can express is a lint rule, checked once on a
  planted violation when added, not a permanent test.
- **Checks run locally first.** `task ci` runs the checks a commit must pass; CI runs the same tasks
  plus the e2e suites and the oldest supported git, all offline after setup (QA-MNT-03).
- **Pull request titles are [Conventional Commits](https://www.conventionalcommits.org)**, the form
  Renovate's take, such as `fix(log): filters survive a reload`. They become the release notes, so
  the description is written for users. A PR no user would notice carries the `no-release-notes`
  label, which keeps it out of them.
