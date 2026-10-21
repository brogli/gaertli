# Build and release

## Build

- The Windows app needs no cgo, so it is cross-compiled on Linux everywhere, and it carries the
  Linux core for WSL repos (ADR-3).
- Release binaries are built on stock CI runners, since binaries built in the Nix shell run only
  inside it.
- A release build stamps its version and commit, shown in Settings; local builds are `dev`.

## Packages

The app needs only git and the platform's web view, and no package bundles either.

- **Windows**: an installer (no administrator rights, per user) and a portable zip. Both carry a
  `nebegit` command for Windows and WSL terminals that starts the app and returns at once
  (FR-REPO-06); the installer puts it on the user PATH. The app's own exe edits the PATH, since the
  installer's script language cannot hold a long one.
- **Linux**: `.deb` and `.rpm` that depend on GTK, WebKitGTK and git, and a `.tar.gz`. The `.deb`
  brings an AppArmor profile, since Ubuntu otherwise denies WebKitGTK's sandbox the user namespaces
  it needs (DIST-06).
- **macOS**: an unsigned, untested app (QA-PORT-04).
- No auto-update and no code signing (DIST-03).

## Versions and releases

- Versions are plain `X.Y.Z`, set by hand before each release, one string in every artifact; tags
  are `vX.Y.Z`.
- A release is started by hand on `main` (DIST-02). The workflow runs the full CI, builds every
  artifact with checksums and third-party notices, and creates a draft release on the built commit,
  replacing older drafts. A failed macOS build does not hold it back.
- Release notes are the PR titles merged since the last release, edited in the draft.
- Publishing the draft tags the built commit, so the published files are the checked ones. A bad
  release is superseded, never deleted, since a deleted immutable release's tag cannot be reused.
