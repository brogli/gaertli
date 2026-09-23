// buildRefTree turns a flat list of namespaced refs (e.g. "feature/a",
// "feature/b", "main") into OpenVue's TreeNode[] shape, folding shared
// `/`-separated prefixes into folder nodes.

export interface RefTreeLeaf {
  name: string
  sha: string
  current?: boolean
  checkedOutIn?: string
  upstream?: string
}

export interface RefTreeNode {
  key: string
  label: string
  leaf?: boolean
  children?: RefTreeNode[]
  data?: { sha: string; current: boolean; checkedOutIn: string; upstream: string }
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
          upstream: ref.upstream ?? '',
        }
      } else {
        node.children ??= []
        siblings = node.children
      }
    })
  }

  markLeaves(roots)
  return roots
}

// markLeaves flags every childless node so OpenVue's Tree stops drawing an
// expand chevron it can never use.
function markLeaves(nodes: RefTreeNode[]): void {
  for (const node of nodes) {
    if (node.children) markLeaves(node.children)
    else node.leaf = true
  }
}
