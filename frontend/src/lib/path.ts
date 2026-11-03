// basename is a POSIX, Windows or UNC path's last segment, ignoring a
// trailing separator; a root is its own basename, a drive root without its
// separator ("C:").
export function basename(path: string): string {
  return splitPath(path).name
}

// parentPath is path up to its last segment: "/" for a child of the POSIX
// root, "C:" for a child of a drive root, "" for a root or a single segment.
export function parentPath(path: string): string {
  return splitPath(path).parent
}

// joinPath is name inside folder, joined with folder's own separator: "/"
// unless folder uses only "\".
export function joinPath(folder: string, name: string): string {
  if (!folder) return name
  const separator = folder.includes('\\') && !folder.includes('/') ? '\\' : '/'
  return folder.replace(/[\\/]+$/, '') + separator + name
}

// lastSegment is path's raw text after its last separator: "" for a path
// ending in one. Unlike basename, withLastSegment(path, lastSegment(path))
// is path again, so a field typed into keeps what was typed.
export function lastSegment(path: string): string {
  return path.slice(lastSegmentStart(path))
}

// withLastSegment is path with its last segment replaced by name.
export function withLastSegment(path: string, name: string): string {
  return path.slice(0, lastSegmentStart(path)) + name
}

function lastSegmentStart(path: string): number {
  return Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1
}

function splitPath(path: string): { parent: string; name: string } {
  const trimmed = path.replace(/(?<=.)[\\/]+$/, '')
  const i = Math.max(trimmed.lastIndexOf('/'), trimmed.lastIndexOf('\\'))
  if (i < 0 || i === trimmed.length - 1) return { parent: '', name: trimmed }
  return { parent: trimmed.slice(0, i || 1), name: trimmed.slice(i + 1) }
}

// repoPath is the argument a core call takes, path- or worktree-routed:
// path unchanged for a local repo (distro unset), or its shell-side UNC form
// (\\wsl.localhost\<distro>\...) for a WSL repo. A path already in UNC form,
// e.g. one the folder picker returned, is left as-is.
export function repoPath(path: string, distro?: string): string {
  if (!distro || path.startsWith('\\\\')) return path
  return `\\\\wsl.localhost\\${distro}${path.replaceAll('/', '\\')}`
}
