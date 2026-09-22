// buildBranchMenuItems is a branch leaf's right-click menu in RefsSidebar,
// empty for the current branch. isRemote picks AddWorktree's mode for
// "Create worktree here…": an existing local branch, or a new branch
// started from the remote-tracking one. "Update from upstream" is offered
// only for a local branch with an upstream.
export function buildBranchMenuItems(
  name: string,
  current: boolean,
  isRemote: boolean,
  hasUpstream: boolean,
  checkout: (name: string) => void,
  createWorktree: (name: string, isRemote: boolean) => void,
  updateBranch: (name: string) => void,
): { label: string; command: () => void }[] {
  return current
    ? []
    : [
        { label: 'Checkout', command: () => checkout(name) },
        { label: 'Create worktree here…', command: () => createWorktree(name, isRemote) },
        ...(isRemote || !hasUpstream
          ? []
          : [{ label: 'Update from upstream', command: () => updateBranch(name) }]),
      ]
}
