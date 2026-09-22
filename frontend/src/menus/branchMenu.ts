// buildBranchMenuItems is a branch leaf's right-click menu in RefsSidebar
// (FR-BR-05). current is false for every remote-tracking branch and for any
// local branch not checked out here, so one function serves both leaf
// kinds; the menu is empty for the current branch. isRemote picks
// AddWorktree's mode for "Create worktree here…" (FR-WT-06): an existing
// local branch, or a new branch started from the remote-tracking one.
export function buildBranchMenuItems(
  name: string,
  current: boolean,
  isRemote: boolean,
  checkout: (name: string) => void,
  createWorktree: (name: string, isRemote: boolean) => void,
): { label: string; command: () => void }[] {
  return current
    ? []
    : [
        { label: 'Checkout', command: () => checkout(name) },
        { label: 'Create worktree here…', command: () => createWorktree(name, isRemote) },
      ]
}
