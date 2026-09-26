# Contributing

## Prerequisites

- Go 1.26 or newer, installed natively (not through Nix).
- Git 2.42 or newer.
- Linux or WSL: [Nix](https://nixos.org/download) with flakes enabled, and
  [direnv](https://direnv.net) (with [nix-direnv](https://github.com/nix-community/nix-direnv) to
  cache the shell). The shell provides Node 24, pnpm, golangci-lint, GTK4, WebKitGTK and a matching
  C compiler.
- Windows without WSL: see below.

## Build and test

```sh
git clone https://github.com/brogli/nebegit
cd nebegit
direnv allow          # enters the Nix shell; first time downloads GTK and WebKitGTK
task setup            # go mod download + pnpm install (the only step that needs the network)
task test             # Go and Vitest tests, offline
task fmt              # linters with fixes (golangci-lint, oxlint, ESLint), then formatters (treefmt)
task lint             # the same as a check that changes nothing (treefmt --ci), vue-tsc
task build            # -> bin/nebegit
task dev              # hot reload: Vite dev server + app window
```

`task` and `wails3` are pinned in `go.mod` as `tool` directives; the shell puts thin wrappers on
`PATH`. Outside the shell run `go tool task <name>`.

Binaries built in the shell link against the shell's GTK and WebKitGTK and run only inside it.
Release binaries come from CI on stock runners, never from a developer machine.

Under WSLg the shell sets `WEBKIT_DISABLE_DMABUF_RENDERER=1`; without it the window may stay blank.

### Windows

Recommended: develop inside WSL with the steps above and cross-compile the Windows binary from the
shell:

```sh
task windows:build            # -> bin/nebegit.exe, no Docker, no extra tools
```

Native Windows without Nix: install Go, Node 24 and pnpm 11 (the
[pnpm installer](https://pnpm.io/installation); the Nix shell pins the exact versions), and the
[golangci-lint binary](https://golangci-lint.run/docs/welcome/install/). Then:

```powershell
go tool task setup
go install tool       # puts task and wails3 on PATH (GOPATH\bin)
task test
task build
```

## How the project is developed

Spec-driven and test-driven: every behaviour is a requirement in `docs/requirements/` before it is
built, and every test name carries the ID it proves (`TestFR_LOG_01_...`, `it('FR-LOG-01 ...')`).
Read `docs/requirements.md` and `docs/architecture.md` before changing behaviour; `CLAUDE.md`
describes the working rules.

Work happens on branches off `main`; `main` receives finished work, releases and dependency updates.
CI runs `task ci` and the real-backend e2e on Linux, the Go tests on Windows and against git 2.42,
and `task build` on macOS. Every check runs offline after the install step, so it fails if a test or
build needs the network.

Dependencies: anything linked into the app or its tests needs a permissive licence (MIT, BSD,
Apache-2.0, MPL-2.0). pnpm is the only JS package manager; commit `frontend/pnpm-lock.yaml`, never a
`package-lock.json`.
