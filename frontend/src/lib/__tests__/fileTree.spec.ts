import { describe, expect, it } from 'vitest'
import type { TreeNode } from 'openvue/treenode'
import { fileTree, folderKeys } from '../fileTree'

const file = (path: string) => ({ path, status: 'M' })

// shape reduces nodes to their keys and labels, children nested.
function shape(nodes: TreeNode[]): unknown[] {
  return nodes.map((n) => (n.children ? [n.key, n.label, shape(n.children)] : [n.key, n.label]))
}

describe('fileTree', () => {
  it('FR-LOG-16 lists folders before files, each by name; a folder holding only one folder is one node with it', () => {
    const files = [
      file('src/app/a.ts'),
      file('README.md'),
      file('src/app/b/c.ts'),
      file('docs/x.md'),
    ]

    expect(shape(fileTree(files))).toEqual([
      ['docs/', 'docs', [['docs/x.md', 'x.md']]],
      [
        'src/app/',
        'src/app',
        [
          ['src/app/b/', 'b', [['src/app/b/c.ts', 'c.ts']]],
          ['src/app/a.ts', 'a.ts'],
        ],
      ],
      ['README.md', 'README.md'],
    ])
  })

  it('FR-LOG-16 a file node carries its changed file; a folder holding a file and a folder stays its own node', () => {
    const a = file('src/a.ts')
    const nodes = fileTree([a, file('src/lib/b.ts')])

    expect(nodes.map((n) => n.key)).toEqual(['src/'])
    expect(nodes[0]!.children!.find((n) => n.key === 'src/a.ts')!.data).toBe(a)
  })

  it('FR-LOG-16 folderKeys lists every folder, nested ones too', () => {
    const nodes = fileTree([file('src/app/b/c.ts'), file('src/app/a.ts'), file('d/e.ts')])

    expect(folderKeys(nodes)).toEqual({ 'd/': true, 'src/app/': true, 'src/app/b/': true })
  })
})
