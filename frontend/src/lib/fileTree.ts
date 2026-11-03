import type { TreeExpandedKeys } from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename } from './path'

interface Folder {
  folders: Map<string, Folder>
  files: ChangedFile[]
}

const byName = (a: string, b: string) => a.localeCompare(b)

// fileTree turns changed files into OpenVue Tree nodes by folder. A file's
// key is its path and its data the file; a folder's key is its path with a
// trailing slash. Folders come before files, each sorted by name, and a
// folder holding only one folder and no files is one node with it
// ("src/app").
export function fileTree(files: ChangedFile[]): TreeNode[] {
  const root = newFolder()
  for (const file of files) {
    let folder = root
    for (const name of file.path.split('/').slice(0, -1)) {
      const sub = folder.folders.get(name) ?? newFolder()
      folder.folders.set(name, sub)
      folder = sub
    }
    folder.files.push(file)
  }
  return childNodes(root, '')
}

// folderKeys marks every folder in nodes, the expanded keys of a fully
// unfolded tree.
export function folderKeys(nodes: TreeNode[]): TreeExpandedKeys {
  const keys: TreeExpandedKeys = {}
  for (const node of nodes) {
    if (!node.children) continue
    keys[node.key] = true
    Object.assign(keys, folderKeys(node.children))
  }
  return keys
}

function newFolder(): Folder {
  return { folders: new Map(), files: [] }
}

function childNodes(folder: Folder, prefix: string): TreeNode[] {
  const folders = [...folder.folders]
    .sort(([a], [b]) => byName(a, b))
    .map(([name, sub]) => folderNode(prefix, name, sub))
  const files = folder.files
    .map((file) => ({ key: file.path, label: basename(file.path), data: file }))
    .sort((a, b) => byName(a.label, b.label))
  return [...folders, ...files]
}

function folderNode(prefix: string, name: string, folder: Folder): TreeNode {
  const only =
    folder.files.length === 0 && folder.folders.size === 1 ? [...folder.folders][0] : undefined
  if (only) return folderNode(prefix, `${name}/${only[0]}`, only[1])
  const path = `${prefix}${name}/`
  return { key: path, label: name, children: childNodes(folder, path) }
}
