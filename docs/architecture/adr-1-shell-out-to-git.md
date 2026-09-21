# ADR-1: Shell out to git

go-git cannot cover the requirements: no cherry-pick, rebase, `apply`, stash, credential helpers, or linked worktrees (v5); v6 is alpha. lazygit, the best-known Go git client, also shells out. No hybrid: two engines would mean two behaviours.

Cost: a process start per call, noticeable on Windows. Mitigation: few, larger calls; streaming for the log; `cat-file --batch`.

