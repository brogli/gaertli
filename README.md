# nebegit

Swiss German *näbe* ("beside") + git: a desktop git GUI that sits beside your editor.

For developers whose editor lacks strong git tooling. Log viewer with graph, commit compare, worktree handling, safe sync actions. Go core, Wails v3 shell, Vue 3 frontend; every git operation runs the system `git`.

Status: pre-alpha. Windows and Linux are the targets, including repos inside WSL from a native Windows window.

- What it will do: [docs/requirements.md](docs/requirements.md)
- How it is built: [docs/architecture.md](docs/architecture.md)
- Building from source: [CONTRIBUTING.md](CONTRIBUTING.md)

## Install

Download from the project's GitHub Releases page: `nebegit-windows-amd64.zip` for Windows; for Linux, a `.deb` or `.rpm` for the common package managers (`nebegit-linux-amd64.deb`, `nebegit-linux-amd64.rpm`) or `nebegit-linux-amd64.tar.gz` for other distributions; or (best-effort, unsigned, untested) `nebegit-darwin-arm64.zip`. `SHA256SUMS` on the same page lets you verify the download.

- **Windows**: unzip and run `nebegit.exe` — no installer, no auto-update.
- **Linux, deb/rpm**: install with your package manager (e.g. `sudo apt install ./nebegit-linux-amd64.deb`, `sudo dnf install ./nebegit-linux-amd64.rpm`); it also installs the web view. No auto-update — upgrade by installing a newer package.
- **Linux, other distributions**: unpack the tar.gz and run the `nebegit` binary inside it; see Start from a terminal to put it on `PATH`.

Two prerequisites, installed separately unless noted above:

- **git** 2.42 or newer, on `PATH`.
- **The OS web view**: WebView2 on Windows (preinstalled on Windows 11; a small Microsoft installer on Windows 10), or the `webkitgtk` system package on Linux — the deb/rpm install it as a dependency; the tar.gz does not bundle it.

## First start

nebegit checks both prerequisites at launch. Missing git: a message names it and asks you to install it and make sure it's on `PATH`. Missing web view on Windows: a native message box names WebView2. On Linux, a missing WebKitGTK fails before nebegit's own code can show a message, so check the terminal output instead.

## Start from a terminal

`nebegit <path>` opens the window for that repo, or brings it to the front. For the command to exist:

- **WSL**: nebegit sets up `~/.local/bin/nebegit` in a distro when it first opens a repo there (the consent it asks names it). `nebegit` in that distro's terminal then opens the Windows app on the WSL repo; without a path it uses the current folder. If `~/.local/bin` was new, open a new terminal first.
- **Linux, deb/rpm**: the package already put `nebegit` on `PATH`.
- **Linux, tar.gz**: put the extracted `nebegit` binary in `~/.local/bin` (or any folder on your `PATH`).
- **Windows**: add the folder that holds `nebegit.exe` to your user `PATH`.

## Editor setup

In Settings (the toolbar's gear), pick a preset or enter a custom "open folder" / "open file" command. Every "open in editor" action in the app uses this one setting. WSL users can set a different editor per distro, since the command has to run where the repo actually lives.

## Logs

nebegit writes every git command and its own errors to a log file; Settings names its path. Set `NEBEGIT_LOG=debug` before starting it to also log automatic background reads (refresh, polling).

## Known limits

- **WSL**: open a WSL repo with "Open folder…" through the Linux entry (`\\wsl.localhost\<distro>`). Only WSL 2 distros are supported. Before nebegit places its headless core and the `nebegit` command inside a distro the first time, it asks for consent; declining leaves nothing behind.
- **SSH passphrase**: nebegit runs git with interactive prompting disabled, so a passphrase-protected SSH key needs a running `ssh-agent` with the key already added — otherwise the git call fails instead of prompting for the passphrase.
- **Windows SmartScreen**: the Windows binary isn't code-signed (no code signing in v1), so Windows SmartScreen warns about it on first run; choose "More info" then "Run anyway".

Licence: BSD-3-Clause.
