# Contributing

Thanks for your interest. nebegit is pre-alpha. Bug reports and small fixes are welcome as issues
and pull requests. For a new feature or a behaviour change, open an issue first: every behaviour is
a requirement row in `docs/requirements/` before it is built, and the row is where we agree on it.

An issue needs the nebegit version, the OS, what you did and what happened, and the lines from the
app log (the README says where it is) when there are any.

## Set up

Build from source as in the [README](README.md#build-from-source), then add
[direnv](https://direnv.net) with [nix-direnv](https://github.com/nix-community/nix-direnv), so the
Nix shell opens on `cd` and `task` works without the `nix develop --command` prefix. On Windows,
develop inside WSL.

```sh
task setup           # once: Go modules and frontend packages
task test            # Go and Vitest tests
task fmt             # fixes lint findings and formatting
task lint            # the same as a check
task e2e             # Playwright against a mock backend
task build           # -> bin/nebegit
task windows:build   # -> bin/nebegit.exe
task dev             # hot reload
task ci              # what CI runs on Linux
```

Binaries built in the shell use its libraries and run only inside it, e.g.
`nix develop --command bin/nebegit`.

### Ubuntu 24.04

Ubuntu grants user namespaces only to programs with an AppArmor profile, and WebKitGTK's sandbox
needs them. Ubuntu 25.04 and later ship a profile the shell can use. 24.04 ships none, so the app
built in the shell aborts at start with `bwrap: setting up uid map: Permission denied`, and the
shell warns at entry. Once, with root:

```sh
sudo tee /etc/apparmor.d/nix-bwrap >/dev/null <<'EOT'
abi <abi/4.0>,
include <tunables/global>

profile nix-bwrap /nix/store/*-bubblewrap-*/bin/bwrap flags=(unconfined) {
  userns,
}
EOT
sudo apparmor_parser -r /etc/apparmor.d/nix-bwrap
```

## Pull requests

- Branch off `main`. One fix or feature per PR.
- Requirement row first, then a failing test, then the code. Test names carry the requirement ID:
  `TestFR_LOG_01_...` in Go, `it('FR-LOG-01 ...')` in Vitest.
- `task ci` passes. Every check runs offline after `task setup`.
- Commit messages: one imperative line, a body when the why isn't obvious.
- The PR title becomes a line of the release notes, so write it for users: what they can do now, not
  what you changed in the code.
- Dependencies need a permissive licence (MIT, BSD, ISC, Apache-2.0, MPL-2.0). `task licence-check`
  verifies that and writes the third-party notices. pnpm is the only JS package manager, so commit
  `frontend/pnpm-lock.yaml`.
- Nothing copied from copyleft (GPL, AGPL, LGPL) or source-available projects. Code taken from a
  permissive project ships its notice and names its source in a comment.

## Licence

BSD-3-Clause, no CLA, no sign-off. Opening a PR confirms that may submit it under this licence.
