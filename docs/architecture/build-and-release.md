# Build, packaging and releases

## Build

- Builds use Wails v3's Taskfiles, run through `go tool task` (TR-25).
- The Windows build embeds the `linux/amd64` core for WSL (ADR-3), so the app stays one download.
  `task build` cross-compiles the core first, then the GUI.
- Version and commit are stamped into the binary at release build time; Settings shows the version
  (DIST-03).
- `task build:server` builds the server-mode app the real-backend e2e suite runs (ADR-2).

## Packaging and releases

- Windows: a single `.exe`, zipped.
- Linux: a `.deb` and an `.rpm` (nfpm, via `wails3 tool package`), installing to `/usr/bin` and
  declaring GTK4, WebKitGTK and git as package dependencies; a `.tar.gz` (binary, `LICENSE`,
  `THIRD-PARTY-NOTICES.txt`) for other distributions.
- macOS: an unsigned `.app`, zipped, labelled untested (QA-PORT-04).
- Run-time prerequisites are git and the platform's web view (DIST-01, TR-04 to TR-06); the tar.gz
  bundles neither.
- No installer, no auto-update, no code signing (DIST-03). Unsigned Windows binaries trigger
  SmartScreen.
- Releases: GitHub Actions builds from a version tag and publishes the artifacts,
  `THIRD-PARTY-NOTICES.txt` and `SHA256SUMS` as a GitHub Release (DIST-02).
- A licence check runs in CI (OSS-01, TR-19).
