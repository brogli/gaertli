# Docs

- **`docs/` holds the current state, one folder or file per topic**: requirements, architecture,
  conventions, the testing strategy, the CI pipelines, the release process. A folder's `README.md`
  is its index. Plans, research notes, task lists and logs stay out of the repo.
- **Docs are written for people who want the point**: goal first, plain words, nothing a reader does
  not need, no history.
- **Requirements are the only source of truth for what the app does.** Every behaviour a remark, a
  plan or a review settles becomes a row before it is built.
- **A requirement row is one rule**: what the user can do and gets, in at most three sentences. A
  new behaviour gets a new row next to the rows it belongs with, not a sentence added to an existing
  one. A check caps a row's length.
- **IDs are stable.** A row that is split keeps its ID for its core rule and the rules moved out get
  new ones. An ID is never reused.
- **A long area is easy to scan**: grouped by topic under sub-headings, opened by a short paragraph
  giving the flow where there is one. Values per case (a side's name per operation) are a small
  table below their row.
- **Rows leave out what a user would not notice.** UI rows may say how the UI is arranged (a
  toolbar, a picker, a shortcut shown on its button), never library or component names, pixel
  values, log levels, file names, dates or history.
- **Cross-cutting qualities are quality attributes**, in `requirements/quality.md`.
- **Architecture reads like a design doc**: decisions and their reasons, structure, UI design, git
  command shapes. An ADR records only a decision that is costly to reverse, conceptually and
  briefly. No implementation detail (file names below package level, per-method notes, mechanical
  flags, exit codes) and no changelog: those live in code, tests, doc comments and git.
- **Conventions state each convention's intent**, not its details: those live in lint and tool
  configuration and in the code.
