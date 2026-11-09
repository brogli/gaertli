# ADR-4: Git is the only source of truth

**Status.** Approved.

**Context.** Gärtli runs beside a terminal and an editor that change the repo at any time. Repo
state the app keeps can go stale, and a stale view in a git tool leads to wrong actions (G4).

**Decision.** No database or on-disk cache of repo data. What the app reads lives in memory and can
be dropped and rebuilt at any time. On disk there are only the settings and the UI's remembered
layout. Git's own rebuildable caches, such as its commit-graph file, are git's state, not the app's.

**Consequences.** After any outside change a reload is correct, so change detection only decides
when to reload. A restarted core loses nothing (QA-REL-03). Every view pays for its first load.
