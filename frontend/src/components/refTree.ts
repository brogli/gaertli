// buildRefTree turns a flat list of namespaced refs (e.g. "feature/a",
// "feature/b", "main") into OpenVue's TreeNode[] shape, folding shared
// `/`-separated prefixes into folder nodes — the usual git-GUI convention
// for namespaced branches (FR-LOG-01). No library does this transform
// (TR-17).

export interface RefTreeLeaf {
  name: string
  sha: string
  current?: boolean
  checkedOutIn?: string
}

export interface RefTreeNode {
  key: string
  label: string
  children?: RefTreeNode[]
  data?: { sha: string; current: boolean; checkedOutIn: string }
}

export function buildRefTree(refs: RefTreeLeaf[]): RefTreeNode[] {
  const roots: RefTreeNode[] = []

  for (const ref of refs) {
    const segments = ref.name.split('/')
    let siblings = roots
    let key = ''

    segments.forEach((segment, i) => {
      key = key ? `${key}/${segment}` : segment
      let node = siblings.find((candidate) => candidate.key === key)
      if (!node) {
        node = { key, label: segment }
        siblings.push(node)
      }

      if (i === segments.length - 1) {
        node.data = {
          sha: ref.sha,
          current: ref.current ?? false,
          checkedOutIn: ref.checkedOutIn ?? '',
        }
      } else {
        node.children ??= []
        siblings = node.children
      }
    })
  }

  return roots
}
