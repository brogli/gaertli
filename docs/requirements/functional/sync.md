# Fetch, pull, push

| ID | Prio | Requirement |
|---|---|---|
| FR-SYNC-01 | MUST | Fetch from one remote or all remotes. Remove remote branches that no longer exist. |
| FR-SYNC-02 | MUST | Pull the current branch with the default pull mode set in Settings (FR-UX-04), or once with merge or rebase, chosen from Pull's menu. |
| FR-SYNC-03 | MUST | Push the current branch. If it has no upstream, offer to set one. |
| FR-SYNC-04 | MUST | If a push is rejected, explain why and offer: pull, or force push. |
| FR-SYNC-05 | MUST | Force push only in the safe form: it fails if the remote has commits the user has not fetched. Plain force push is not offered. |
| FR-SYNC-06 | MUST | Force push asks for confirmation. Protected branches (configurable, default `main`, `master`) get an extra warning. Settings take them one name at a time: typing suggests the current repo's matching branch names, and a chosen suggestion or a typed name becomes its own entry, which can be removed. |
| FR-SYNC-07 | MUST | Remote actions show progress and can be cancelled, in the jobs menu (FR-UX-12). Cancelling leaves the repo as git leaves it when interrupted: no lock file of git's blocks the next action, and an operation stopped mid-way can be continued or aborted. |
| FR-SYNC-11 | MUST | Sync controls offer only what can act. Without a remote, Fetch, Pull and Push are disabled and their tooltip says the repo has no remote. A choice of remote appears only when there is more than one. No control shows an empty list. |
| FR-SYNC-08 | MUST | Use the user's existing git login; the app never asks for or sees credentials. If login fails, say so and show git's message. |
| FR-SYNC-09 | SHOULD | If a pull is blocked by local changes, offer to stash them and restore them after. |
| FR-SYNC-10 | SHOULD | Automatic fetch at a set interval. Off by default. |
