# Cherry-pick and revert

## Cherry-pick (CP)

| ID       | Prio   | Requirement                                                                                                                                                                                                       |
| -------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| FR-CP-01 | MUST   | Cherry-pick one or more commits selected in the log onto the current branch of this window's worktree.                                                                                                            |
| FR-CP-02 | MUST   | Several commits are applied oldest first, whatever the selection order.                                                                                                                                           |
| FR-CP-03 | MUST   | If a pick stops on conflicts, the conflict flow takes over (FR-CONF).                                                                                                                                             |
| FR-CP-04 | MUST   | If git refuses to start (local changes in the way, other operation in progress), show why. Nothing is changed.                                                                                                    |
| FR-CP-05 | MUST   | A selection that contains a merge commit is not picked: the app refuses before changing anything and names that commit.                                                                                           |
| FR-CP-06 | SHOULD | Options: add a "cherry picked from …" line to the message; apply without committing.                                                                                                                              |
| FR-CP-07 | SHOULD | Skip the current commit during a stopped multi-commit pick. Skip is offered for a stopped cherry-pick or revert only; asked for in any other state, the app says which operation is in progress, or that none is. |

## Revert (RV)

| ID       | Prio   | Requirement                                                                                                                         |
| -------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| FR-RV-01 | MUST   | Revert one or more commits selected in the log. Creates new commits on the current branch of this window's worktree that undo them. |
| FR-RV-02 | MUST   | Several commits are reverted newest first.                                                                                          |
| FR-RV-03 | MUST   | Conflicts, refusal to start, and merge commits are handled as for cherry-pick (FR-CP-03 to FR-CP-05).                               |
| FR-RV-04 | SHOULD | Option: apply without committing.                                                                                                   |
