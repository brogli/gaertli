# Code

Code is read far more often than it is written, mostly by someone without its author's context.

- **Readable, then simple, then short.** In that order: a clever line that saves three loses.
- **The simplest thing that works now.** No abstraction, option or extension point before a second
  real use asks for it; tolerate a duplicate once before extracting it.
- **Loose coupling, high cohesion.** A package, component or store owns one area and shows a small
  surface. Areas talk through their public surface (the core API, events, store actions), never
  through another's internals. Dependencies point one way.
- **Step-down.** A file reads top to bottom: each function is followed by the ones it calls, and
  stays at one level of abstraction.
- **Names in the domain's words.** Each concept has its glossary term (QA-USE-03); no `data`,
  `info`, `manager`, `util`.
- **Check at the edge, trust inside.** Outside data (git output, JSON, user input) is parsed and
  validated once where it enters; fixed sets and domain values are types, so a wrong value fails to
  compile or is refused where it enters.
- **Errors are never swallowed.** Handle an error or return it with context; the user sees what
  failed (QA-REL-01).
- **Idiomatic for the stack, newest stable idiom first.** Go reads like Effective Go, Vue like the
  Vue style guide.
- **Delete freely.** Dead code, unused options and commented-out code go; git keeps the history.
- **Pragmatic.** A rule bends where following it makes the code worse, and the change says why.
