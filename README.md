# nebegit

[![ci](https://github.com/brogli/nebegit/actions/workflows/ci.yml/badge.svg)](https://github.com/brogli/nebegit/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/brogli/nebegit)](https://github.com/brogli/nebegit/releases)
[![licence](https://img.shields.io/github/license/brogli/nebegit)](LICENSE)
[![go](https://img.shields.io/github/go-mod/go-version/brogli/nebegit)](go.mod)

Swiss German _näbe_ ("beside") + git: a desktop git GUI that sits beside your editor.

For developers whose editor lacks strong git tooling. Log viewer with graph, commit compare,
worktree handling, safe sync actions. Go core, Wails v3 shell, Vue 3 frontend; every git operation
runs the system `git`.

Status: pre-alpha. Windows and Linux are the targets, including repos inside WSL from a native
Windows window.

- What it will do: [docs/requirements.md](docs/requirements.md)
- How it is built: [docs/architecture.md](docs/architecture.md)
- Developing nebegit: [CONTRIBUTING.md](CONTRIBUTING.md)

## Install

Download from the project's GitHub Releases page; `SHA256SUMS` there verifies the download. There is
no auto-update: installing a newer version over the old one keeps your settings.

- **Windows, installer** (recommended): run `nebegit-windows-amd64-setup.exe`; no administrator
  rights needed. It installs under your user profile, adds a Start menu entry, and puts the
  `nebegit` command on `PATH` for Windows and WSL terminals opened afterwards. Uninstalling from
  Windows' "Installed apps" removes the app and that command.
- **Windows, zip**: unzip `nebegit-windows-amd64.zip` and run `nebegit.exe`; nothing is installed.
  To use the `nebegit` command, add the unzipped `bin` folder to your user `PATH` by hand (Windows'
  "Edit environment variables for your account").
- **Windows, winget** (once published): `winget install brogli.nebegit`; `winget upgrade` and
  `winget uninstall` give the same result as the installer.
- **Linux, deb/rpm**: install with your package manager, e.g.
  `sudo apt install ./nebegit-linux-amd64.deb` or `sudo dnf install ./nebegit-linux-amd64.rpm`.
- **Linux, other distributions**: unpack `nebegit-linux-amd64.tar.gz` and run the `nebegit` binary
  inside it; see Start from a terminal to put it on `PATH`.
- **macOS** (best-effort, unsigned, untested): `nebegit-darwin-arm64.zip`.

You also need:

- **git** 2.42 or newer, on `PATH`.
- **The OS web view**: WebView2 on Windows (preinstalled on Windows 11; a small Microsoft installer
  on Windows 10), or WebKitGTK 6.0 on Linux, which the deb/rpm install.

## Build from source

You need git, [Go](https://go.dev/dl) 1.26 or newer and [Nix](https://nixos.org/download) with
flakes enabled; on Windows, all three inside WSL. The Nix shell brings everything else.

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

`nebegit [path]` opens or focuses the window for that repo (`.` for the current folder); without a
path it reopens the windows that were open at last quit. This needs the `nebegit` command on `PATH`
(see Install):

- **Windows, cmd or PowerShell**: works from any folder; the command returns at once and leaves the
  app running.
- **WSL**: the same command, once a new terminal has picked up the PATH change (only needed the
  first time). `nebegit .` opens the app in a native Windows window on the WSL repo; nothing is
  installed inside the distro for the command itself.
- **Linux, deb/rpm**: the package already put `nebegit` on `PATH`.
- **Linux, tar.gz**: put the extracted `nebegit` binary in `~/.local/bin` (or any folder on your
  `PATH`).

## Editor setup

In Settings (the toolbar's gear), pick a preset or enter a custom "open folder" / "open file"
command. Each WSL distro has its own editor setting, since the command runs where the repo lives.

## Logs

nebegit logs its own errors and every failed git or editor command to a file; Settings names its
path. Debug mode in Settings, or `NEBEGIT_LOG=debug` set before starting it, logs every git and
editor command it runs.

## Known limits

- **WSL**: open a WSL repo with "Open folder…" through the Linux entry (`\\wsl.localhost\<distro>`).
  Only WSL 2 distros are supported. Before nebegit places its helper inside a distro, the first time
  and after each update, it asks for consent, naming the distro; declining leaves nothing behind.
- **SSH passphrase**: nebegit runs git with interactive prompting disabled, so a
  passphrase-protected SSH key needs a running `ssh-agent` with the key already added; otherwise the
  git call fails.
- **Windows SmartScreen**: nebegit isn't code-signed in v1, so SmartScreen warns on first run;
  choose "More info", then "Run anyway".

Licence: BSD-3-Clause.
