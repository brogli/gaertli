// buildBranchMenuItems is a branch leaf's right-click menu in RefsSidebar,
// empty for the current branch. For a remote branch, "Create worktree
// here…" starts a new local branch from it. "Update from upstream" needs a
// local branch with an upstream; Rename… and Delete need a local branch.
export function buildBranchMenuItems(
  { name, current, isRemote, hasUpstream }: {
    name: string
    current: boolean
    isRemote: boolean
    hasUpstream: boolean
  },
  actions: {
    checkout: (name: string) => void
    createWorktree: (name: string, isRemote: boolean) => void
    updateBranch: (name: string) => void
    openRename: (name: string) => void
    deleteBranch: (name: string) => void
  },
): { label: string; command: () => void }[] {
  if (current) return []
  return [
    { label: 'Checkout', command: () => actions.checkout(name) },
    { label: 'Create worktree here…', command: () => actions.createWorktree(name, isRemote) },
    ...(isRemote || !hasUpstream
      ? []
      : [{ label: 'Update from upstream', command: () => actions.updateBranch(name) }]),
    ...(isRemote
      ? []
      : [
          { label: 'Rename…', command: () => actions.openRename(name) },
          { label: 'Delete', command: () => actions.deleteBranch(name) },
        ]),
  ]
}
