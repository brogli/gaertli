# nebegit

[![ci](https://github.com/brogli/nebegit/actions/workflows/ci.yml/badge.svg)](https://github.com/brogli/nebegit/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/brogli/nebegit)](https://github.com/brogli/nebegit/releases)
[![licence](https://img.shields.io/github/license/brogli/nebegit)](LICENSE)
[![go](https://img.shields.io/github/go-mod/go-version/brogli/nebegit)](go.mod)

Swiss German _näbe_ ("beside") + git: a desktop git GUI that sits beside your editor.

For developers whose editor lacks strong git tooling. Log viewer with graph, commit compare,
worktree handling, safe sync actions, merge, rebase, cherry-pick and a three-pane merge editor. Go
core, Wails v3 shell, Vue 3 frontend. Every git operation runs the system `git`.

Status: pre-alpha. Windows and Linux are the targets, including repos inside WSL from a native
Windows window.

- What it does: [docs/requirements.md](docs/requirements.md)
- How it is built: [docs/architecture.md](docs/architecture.md), its code conventions:
  [docs/conventions.md](docs/conventions.md)
- Developing nebegit: [CONTRIBUTING.md](CONTRIBUTING.md)

## AI assisted coding disclosure

This is developed as an experiment and learning experience to try spec-driven development. While
opinionated directions were given, expect some heavy AI slop.

## Install (yes, there's an exe)

Download from the project's GitHub Releases page. `SHA256SUMS` there verifies the download. There is
no auto-update. Installing a newer version over the old one keeps your settings.

- **Windows, installer** (recommended): run `nebegit-windows-amd64-setup.exe`. No administrator
  rights needed. It installs under your user profile, adds a Start menu entry, and puts the
  `nebegit` command on `PATH` for Windows and WSL terminals opened afterwards. Uninstalling from
  Windows' "Installed apps" removes the app and that command.
- **Windows, zip**: unzip `nebegit-windows-amd64.zip` and run `nebegit.exe`. Nothing is installed.
  For the `nebegit` command, add the unzipped `bin` folder to your user `PATH` by hand (Windows'
  "Edit environment variables for your account").
- **WSL**: install the Windows app (any of the above). Nothing is installed inside the distro by
  hand. The Linux packages below are for native Linux desktops, not for WSL. See Known limits.
- **Linux, deb/rpm**: install with your package manager, e.g.
  `sudo apt install ./nebegit-linux-amd64.deb` or `sudo dnf install ./nebegit-linux-amd64.rpm`.
- **Linux, other distributions**: unpack `nebegit-linux-amd64.tar.gz` and run the `nebegit` binary
  inside it. Start from a terminal says how to put it on `PATH`.
- **macOS** (best-effort, unsigned, untested): `nebegit-darwin-arm64.zip`.

You also need:

- **git** 2.42 or newer, on `PATH`.
- **The OS web view**: WebView2 on Windows (preinstalled on Windows 11, a small Microsoft installer
  on Windows 10), or WebKitGTK 6.0 on Linux, which the deb/rpm install.

## Build from source

You need git, [Go](https://go.dev/dl) 1.26 or newer and [Nix](https://nixos.org/download) with
flakes enabled. On Windows, all three inside WSL. [direnv](https://direnv.net) is optional, but
recommended. The Nix shell brings everything else.

```sh
git clone https://github.com/brogli/nebegit
cd nebegit
nix develop --command task setup   # downloads Go modules and frontend packages
```

- **Windows**: `nix develop --command task windows:build` builds `bin/nebegit.exe`, which runs on
  Windows as it is.
- **Linux**: `nix develop --command task build` builds `bin/nebegit`. It uses the Nix shell's
  libraries, so start it from there: `nix develop --command bin/nebegit`. For an app that runs on
  its own, install a release package.

## First start

nebegit checks both prerequisites at launch and says which one is missing or too old: on Windows in
a message box, on Linux only in the terminal output. If it doesn't open on Linux, start it from a
terminal.

## Start from a terminal

`nebegit [path]` opens or focuses the window for that repo (`.` for the current folder). Without a
path it reopens the windows that were open at last quit. This needs the `nebegit` command on `PATH`
(see Install):

- **Windows, cmd or PowerShell**: works from any folder. The command returns at once and leaves the
  app running.
- **WSL**: the same command, once a new terminal has picked up the PATH change (only needed the
  first time). `nebegit .` opens the app in a native Windows window on the WSL repo.
- **Linux, deb/rpm**: the package already put `nebegit` on `PATH`.
- **Linux, tar.gz**: put the extracted `nebegit` binary in `~/.local/bin` (or any folder on your
  `PATH`).

## Editor setup

In Settings (the toolbar's gear), pick a preset or enter a custom "open folder" / "open file"
command. Each WSL distro has its own editor setting, since the command runs where the repo lives.

## Logs

- **Windows, including WSL repos**: `%LOCALAPPDATA%\nebegit\logs\nebegit.log`
- **Linux**: `~/.local/state/nebegit/nebegit.log`, or under `$XDG_STATE_HOME` if that is set

## Known limits

- **WSL**: WSL 2 only. Open a repo with "Open folder…" through the Linux entry
  (`\\wsl.localhost\<distro>`) or with `nebegit .` from a WSL terminal. Git runs inside the distro
  through a small helper that nebegit copies there, after asking you.
- **SSH passphrase**: nebegit cannot ask you for a passphrase. A key that has one must already be
  unlocked in a running `ssh-agent`, or the git call fails.
- **Windows SmartScreen**: nebegit isn't code-signed, so SmartScreen warns on first run. Choose
  "More info", then "Run anyway".

Licence: BSD-3-Clause.
