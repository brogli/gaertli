# Architecture — Gärtli

How Gärtli is built. What it does is in `requirements.md`, how its code is written in
`conventions.md`.

## Principles

- **Git does the work.** Every operation runs the user's own git (ADR-1).
- **Git is the only source of truth.** The app keeps no repo data of its own (ADR-4).
- **One core, two placements.** The backend runs in-process for local repos and inside the distro
  for WSL repos, behind one interface (ADR-3).
- **One process, many windows.** Windows on one repo share its log, caches and locks (ADR-8).
- **Heavy lifting in Go.** Go builds the log and graph; the UI fetches only what it shows (ADR-5).

## System context

```mermaid
C4Context
  Person(dev, "Developer", "Works in a terminal and an editor beside Gärtli")
  System(gaertli, "Gärtli", "Desktop git GUI")
  System_Ext(git, "git", "The user's git, with their config, hooks and credentials")
  System_Ext(editor, "External editor", "The user's configured editor")
  SystemDb_Ext(repo, "Repository", "Git dir and worktrees")
  System_Ext(remote, "Git remotes", "Reached only through git")

  Rel(dev, gaertli, "Browses history, stages, commits, syncs")
  Rel(gaertli, git, "Runs")
  Rel(gaertli, editor, "Starts")
  Rel(gaertli, repo, "Watches")
  Rel(git, repo, "Reads, writes")
  Rel(git, remote, "Fetches, pushes")
```

## Containers

```mermaid
C4Container
  UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
  Person(dev, "Developer")

  System_Boundary(gaertli, "Gärtli") {
    Container(ui, "UI", "Vue 3, TypeScript, WebView", "One per window: log, changes, diff, merge editor")
    Container(app, "Desktop app", "Go, Wails v3", "Windows, settings, app log; runs the core for local repos and routes each call to its core")
    ContainerDb(settings, "Settings", "JSON file", "Recent repos, open windows, preferences")
    Container(wslcore, "WSL core", "Go, headless, inside a WSL distro", "The same core, for repos in that distro")
  }

  System_Ext(git, "git", "Runs where the repo lives")
  System_Ext(editor, "External editor")
  SystemDb_Ext(repo, "Repository")

  Rel(dev, ui, "Uses")
  Rel(ui, app, "Calls, events", "Wails bindings")
  Rel(app, settings, "Reads, writes")
  Rel(app, wslcore, "Calls, events", "JSON-RPC over stdio")
  Rel(app, git, "Runs, local repos")
  Rel(wslcore, git, "Runs, WSL repos")
  Rel(app, editor, "Starts, local repos")
  Rel(wslcore, editor, "Starts, WSL repos")
  Rel(git, repo, "Reads, writes")
```

## Components: desktop app

```mermaid
C4Component
  UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
  Container_Boundary(app, "Desktop app") {
    Component(shell, "Shell", "internal/app", "Windows, single instance, settings, app log, event routing; a generated facade picks each call's core")
    Component(remote, "WSL connector", "internal/remote", "Finds distros, deploys and starts a WSL core, the generated RPC client")
    Component(core, "Core", "internal/core, internal/services", "The API and its implementation, one feature area at a time")
    Component(session, "Sessions", "internal/session", "Repo and worktree sessions: caches, write queues, watchers, drafts")
    Component(git, "Git runner", "internal/git", "Runs git, parses its machine-readable output")
    Component(graph, "Graph layout", "internal/graph", "Lanes over the commit stream")
    Component(watch, "Watchers", "internal/watch", "Git-dir watcher, worktree status poller")
    Component(jobs, "Jobs", "internal/jobs", "Long-running tasks with progress and cancel")
    Component(editor, "Editor launcher", "internal/editor", "Editor profiles, starting the editor")
  }
  Rel(shell, core, "Calls, local repos")
  Rel(shell, remote, "Calls, WSL repos")
  Rel(core, session, "Uses")
  Rel(core, git, "Uses")
  Rel(core, graph, "Uses")
  Rel(core, editor, "Uses")
  Rel(session, watch, "Uses")
  Rel(session, jobs, "Uses")
  Rel(session, git, "Uses")
```

The WSL core is the same core, session, runner and job code behind an RPC dispatcher instead of the
shell.

## Documents

- Decisions:
  - [ADR-1: Shell out to git](architecture/adr-1-shell-out-to-git.md)
  - [ADR-2: Wails v3](architecture/adr-2-wails-v3.md)
  - [ADR-3: One core interface, two placements](architecture/adr-3-core-placement.md)
  - [ADR-4: Git is the only source of truth](architecture/adr-4-git-is-the-only-source-of-truth.md)
  - [ADR-5: Log and graph are built in Go](architecture/adr-5-log-and-graph-in-go.md)
  - [ADR-6: Write queues by what they lock](architecture/adr-6-write-queues-by-what-they-lock.md)
  - [ADR-7: Git computes the diff, CodeMirror draws it](architecture/adr-7-diff-ui-with-codemirror-6-merge.md)
  - [ADR-8: One process, one selection per window](architecture/adr-8-one-process-selection-per-window.md)
- [backend.md](architecture/backend.md): shell and core responsibilities, calls, events, errors.
- [frontend.md](architecture/frontend.md): window state, layout, diff view, merge editor.
- [git-mapping.md](architecture/git-mapping.md): which git commands back which feature.
