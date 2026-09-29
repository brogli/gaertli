# Build, packaging and releases

## Build

- Builds use Wails v3's Taskfiles, run through `go tool task` (TR-25).
- The Windows build embeds the `linux/amd64` core for WSL (ADR-3), so the app stays one download.
  `task build` cross-compiles the core first, then the GUI.
- A release build stamps its version and commit into the binary; Settings shows both, e.g.
  `0.1.0 (abc1234)` (DIST-03). Local builds are `dev`.
- `task build:server` builds the server-mode app the real-backend e2e suite runs (ADR-2).

## Packaging and releases

- Windows: an NSIS installer (the primary download and the file winget installs) and a portable zip.
  - The installer runs without administrator rights, into `%LOCALAPPDATA%\Programs\nebegit` by
    default (the user can pick another folder), adds a Start menu entry and no desktop icon, and
    supports a silent install (`/S`) for package managers; it shows up like any other installed app,
    with its version and publisher. It does not install WebView2: the app's own start-up check
    (DIST-01) covers a missing one, and a bundled bootstrapper would redistribute Microsoft's own
    binary under Microsoft's terms, not a permissive licence (OSS-05).
  - Both the installer and the zip carry a `bin` folder with `nebegit` (POSIX sh, for WSL and Git
    Bash) and `nebegit.cmd` (cmd, PowerShell): the `nebegit` command a terminal finds, once that
    folder is on the user PATH (FR-REPO-06). Each script starts the exe beside it and returns at
    once, without waiting for the app; in WSL, `nebegit`'s non-flag arguments are converted to
    Windows paths (`wslpath -w`) first, so `nebegit .` opens the WSL repo. The installer puts
    `<install>\bin` on the user PATH itself; the zip is a plain unpack, and the user docs say how to
    add its `bin` folder to PATH by hand.
  - The PATH edit is made by the app's own exe, called by the installer as
    `nebegit.exe --add-to-path` and by the uninstaller as `nebegit.exe --remove-from-path`: an NSIS
    script's strings stop at 1024 characters, too short for a long user PATH, so NSIS itself never
    reads or writes it. The exe appends or removes exactly its `bin` folder, once, keeping every
    other entry, its order and its `%VAR%` text; it writes the value back only when it changed, as
    `REG_EXPAND_SZ` (its original type), and then broadcasts `WM_SETTINGCHANGE` so a terminal
    already open in Explorer's session sees the change without a new login. The app itself never
    touches PATH outside these two calls. If a call fails, the installer or uninstaller leaves PATH
    as it was and names the folder to add or remove by hand.
  - Installing over an existing installation replaces the files and keeps the settings, which live
    outside the install folder (DIST-03); a package manager upgrade and replacing the archive's
    files do the same.
  - A running nebegit locks its exe. The installer and the uninstaller check for that first, before
    they change anything: interactively they ask the user to close it and retry; a silent run exits
    with 32 (Windows' sharing-violation code) instead of skipping the locked file and reporting
    success, which is what NSIS does by default. The check catches the exe in the install folder
    only, the one file that would be skipped.
  - winget: `brogli.nebegit`, `InstallerType: nullsoft`, `Scope: user`, installed from the published
    release asset with its `SHA256SUMS` hash, using NSIS's own silent switches for install and
    uninstall (DIST-06); its `ExpectedReturnCodes` map 32 to `packageInUse`, so winget tells the
    user to close the app. No manifest lives in this repo: it would only restate each release's
    version and hash, and cannot be validated on Linux. The owner submits it to
    `microsoft/winget-pkgs` by hand, for published releases.
- Linux: a `.deb` and an `.rpm` (nfpm, via `wails3 tool package`), installing to `/usr/bin` and
  declaring GTK4, WebKitGTK and git as package dependencies; a `.tar.gz` (binary, `LICENSE`,
  `THIRD-PARTY-NOTICES.txt`) for other distributions.
- macOS: an unsigned `.app`, zipped, labelled untested (QA-PORT-04).
- Run-time prerequisites are git and the platform's web view (DIST-01, TR-04 to TR-06); the tar.gz
  and the Windows zip bundle neither.
- No auto-update, no code signing (DIST-03). Unsigned Windows binaries trigger SmartScreen.
- Versions are plain semantic versions `X.Y.Z`, set by hand in `build/config.yml` and raised before
  each release; tags are `vX.Y.Z`. One string everywhere: the app, the exe's version resource, the
  installer's `DisplayVersion` (its numeric version is `X.Y.Z.0`), the deb and rpm, and winget's
  `PackageVersion`. No pre-release suffix and no build number: "release candidate" is the draft's
  state, not part of the files, and two candidates of one version never both ship, since only one
  draft is ever published. The commit tells builds apart.
- Releases (DIST-02) are started by hand; nothing starts on a merge.
  - Release candidate: a GitHub Actions workflow the maintainer starts on `main` (it refuses any
    other branch, and a version whose tag exists, i.e. one already released). It runs the full CI
    workflow on the commit, then builds every artifact (the Windows installer is cross-built on
    Linux, like the rest of the Windows build, TR-26), writes `THIRD-PARTY-NOTICES.txt` and
    `SHA256SUMS`, attests each file's build provenance once the repo is public, and creates a draft
    release `vX.Y.Z` targeting the built commit's SHA, never a branch, titled with the version and
    commit. It first deletes older drafts, so the next candidate of the same version replaces a
    failed one.
  - Release notes are GitHub's generated list of the pull requests merged since the last release,
    bots excluded, edited by the maintainer in the draft. PR titles are therefore written for users.
  - Publish: the maintainer checks the draft's files and publishes that same draft. A draft has no
    tag; publishing creates `vX.Y.Z` on the built commit, so the published files are the checked
    ones. With immutable releases enabled, tag and files stay fixed after that.
  - After a failed candidate the fix goes through a pull request into `main` and a new run, with the
    same version. A bad published release is marked pre-release with a warning and superseded, not
    deleted: a deleted immutable release's tag can never be reused.
  - `task release:dry-run` builds this platform's artifacts the same way, locally.
- A licence check runs in CI (OSS-01, TR-19).
