# ADR-4: Git is the only source of truth

- No index, database, or on-disk cache of repo data. The core binary copied into a WSL distro is program code, not repo data.
- In memory: commit list, graph rows, ref map, last status. All droppable and rebuildable.
- On disk: settings only (recent repos, preferences, last pull mode).

