// buildBranchMenuItems is a branch leaf's right-click menu in RefsSidebar
// (FR-BR-05), sibling to commitMenu.ts's own row menu: current is false for
// every remote-tracking branch (never itself HEAD) as well as any local
// branch that isn't checked out here, so the same function serves both a
// local and a remote branch leaf. Empty when current — nothing to offer.
// Later specs append their own items to this same builder: 91 (rename,
// delete).
export function buildBranchMenuItems(
  name: string,
  current: boolean,
  checkout: (name: string) => void,
): { label: string; command: () => void }[] {
  return current ? [] : [{ label: 'Checkout', command: () => checkout(name) }]
}
