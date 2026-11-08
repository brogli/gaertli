# ADR-9: One package of bounded contexts

**Context.** The core API had grown to one interface spanning several areas, with one namespace and
no stated owner of a type or a rule (QA-USE-03, QA-MNT-05). A domain-driven split could go two ways:
a package per area, or one package with an interface per area.

**Decision.** The core is one Go package holding one interface per bounded context (log, changes,
branches, sync, operations, conflicts, worktrees, editor), composed into what a window may call and
what only the shell calls. Each context owns its calls, its types and its session state and reads
another's only through that context's types. The implementation mirrors it: one package, one file
group per context, each method in its context's files.

Rejected: a package per context. Every type two contexts share would need a kernel package below
them and the composed API a package above, the generator would route across packages, the UI
bindings would come as one module per package, and no rule would be enforced that the interfaces do
not already state.

**Consequences.** The contexts are visible in the file layout and the glossary, and the frontend's
stores can follow them. They are cheap to rename or regroup. A package boundary is not available to
enforce them; review is. Splitting into packages stays open once the shared logic has moved out.
