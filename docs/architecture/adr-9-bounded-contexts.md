# ADR-9: The core is split into bounded contexts

**Context.** The core API had grown to one interface spanning several areas, with one namespace and
no stated owner of a type or a rule (QA-USE-03, QA-MNT-05).

**Decision.** The core is organised as bounded contexts, each an interface in one Go package (log,
changes, branches, sync, operations, conflicts, worktrees, editor), composed into what a window may
call and what only the shell calls. Each context owns its calls, its types and its session state and
reads another's only through that context's types. The implementation mirrors it: one file group per
context, each method in its context's files.

One package keeps the shared types in one place, the generator and the UI bindings one module, and
the interfaces already state the boundaries, so no package line is needed to draw them.

**Consequences.** The contexts are visible in the file layout and the glossary, and the frontend's
stores can follow them. They are cheap to rename or regroup. Review, not the compiler, keeps a
context from reaching into another's internals.
