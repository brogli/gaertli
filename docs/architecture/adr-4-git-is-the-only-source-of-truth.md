# ADR-4: Git is the only source of truth

Status: accepted

## Context

nebegit runs beside a terminal and an editor that change the repo at any time. Any state the app keeps about a repo can go stale, and a stale view in a git tool leads to wrong actions (G4).

## Decision

- No index, database or on-disk cache of repo data.
- In memory only: the commit list, graph rows, ref map and last status. All of it can be dropped and rebuilt.
- On disk, only the settings (recent repos, the windows open at last quit, the Settings dialog's choices) and the UI's remembered choices in the WebView's local storage (the last pull mode, panel widths).
- The core binary copied into a WSL distro is program code, not repo data.

One exception: git's own rebuildable maintenance files do not count as app state. On the first open of a large repo, nebegit may run `git commit-graph write --reachable`. The file lives in `.git/objects/info/`, git maintains it itself, and its absence changes nothing but speed. On a 100k-commit repo it cut time to the first log row from 280 ms to 63 ms and peak memory by 35 %.

## Consequences

- After any outside change, a reload from git is always correct. Change detection (`git-mapping.md`) only has to decide *when* to reload.
- A crashed or restarted core, such as a WSL core after `wsl --shutdown`, loses nothing.
- Every view pays for its first load. The log is streamed so the first rows show at once (ADR-5).
