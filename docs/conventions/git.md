# Running git

The app must get exactly what the user's terminal would get (QA-COMP-01), and repo text must never
be read as a command (QA-SEC-03).

- **No shell, no options from repo text.** Git is called with an argument list. Paths come after
  `--`, revisions after `--end-of-options`.
- **Machine output only.** Git's porcelain, `-z` and custom formats are parsed, never its
  human-readable output, and unknown fields are ignored, so a newer git does not break parsing.
- **The user's environment, minus interaction.** Git never prompts (it fails instead), reads never
  take optional locks the user's own commands need (QA-COMP-02), git's messages are English so the
  few that must be matched can be, and the locale hooks and filters run in stays the user's.
- **Never wait for an editor.** Where git would open one for a message, it gets none.
- **Cancellable.** Every call can be stopped the way an interrupt in a terminal stops git.
- **Config that changes the result stays, framing goes.** A parsed patch keeps the user's diff
  algorithm, attributes and filters but drops colour, external diff tools and prefixes.
- **Unambiguous refs.** A ref the user picked reaches git by its full name, except a local branch to
  check out, where a full name would detach.
- **Don't reimplement git.** Where git can derive, validate or refuse something, it does.
