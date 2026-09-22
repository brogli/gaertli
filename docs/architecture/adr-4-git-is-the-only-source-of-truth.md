# ADR-4: Git is the only source of truth

- No index, database, or on-disk cache of repo data. The core binary copied into a WSL distro is program code, not repo data.
- In memory: commit list, graph rows, ref map, last status. All droppable and rebuildable.
- On disk: settings only (recent repos, preferences, last pull mode).

- Exception (2026-09-21, owner decision): git's own rebuildable maintenance files are not app state. nebegit may run `git commit-graph write --reachable` on the first open of a large repo; the file lives in `.git/objects/info/`, git maintains and rebuilds it itself, and its absence changes nothing but speed (measured: first log row 280 ms → 63 ms, peak RSS −35 % on a 100k-commit repo).
