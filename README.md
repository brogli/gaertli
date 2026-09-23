# nebegit

Swiss German *näbe* ("beside") + git: a desktop git GUI that sits beside your editor.

For developers whose editor lacks strong git tooling. Log viewer with graph, commit compare, worktree handling, safe sync actions. Go core, Wails v3 shell, Vue 3 frontend; every git operation runs the system `git`.

Status: pre-alpha. Windows and Linux are the targets, including repos inside WSL from a native Windows window.

- What it will do: [docs/requirements.md](docs/requirements.md)
- How it is built: [docs/architecture.md](docs/architecture.md)
- Building from source: [CONTRIBUTING.md](CONTRIBUTING.md)

## Install

Download the archive for your platform from the project's GitHub Releases page: `nebegit-windows-amd64.zip`, `nebegit-linux-amd64.AppImage`, or (best-effort, unsigned, untested) `nebegit-darwin-arm64.zip`. `SHA256SUMS` on the same page lets you verify the download. Unzip (Windows) or mark the AppImage executable (`chmod +x`, Linux) and run it — no installer, no auto-update.

Two prerequisites, installed separately:

- **git**, on `PATH`.
- **The OS web view**: WebView2 on Windows (preinstalled on Windows 11; a small Microsoft installer on Windows 10), or the `webkitgtk` system package on Linux — the AppImage does not bundle it.

## First start

nebegit checks both prerequisites at launch. Missing git: a message names it and asks you to install it and make sure it's on `PATH`. Missing web view on Windows: a native message box names WebView2. On Linux, a missing WebKitGTK fails before nebegit's own code can show a message, so check the terminal output instead.

## Editor setup

Settings panel: pick a preset or enter a custom "open folder" / "open file" command. Every "open in editor" action in the app uses this one setting. WSL users can set a different editor per distro, since the command has to run where the repo actually lives.

## Known limits

- **WSL**: only WSL 2 distros work; a listed WSL 1 distro is shown disabled. Before nebegit places its headless core inside a distro the first time, it asks for consent; declining leaves nothing behind.
- **SSH passphrase**: nebegit runs git with interactive prompting disabled, so a passphrase-protected SSH key needs a running `ssh-agent` with the key already added — otherwise the git call fails instead of prompting for the passphrase.

Licence: BSD-3-Clause.
