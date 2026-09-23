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

function splitPath(path: string): { parent: string; name: string } {
  const trimmed = path.replace(/(?<=.)[\\/]+$/, '')
  const i = Math.max(trimmed.lastIndexOf('/'), trimmed.lastIndexOf('\\'))
  if (i < 0 || i === trimmed.length - 1) return { parent: '', name: trimmed }
  return { parent: trimmed.slice(0, i || 1), name: trimmed.slice(i + 1) }
}

// repoPath is the argument a path-routed core call takes: path unchanged for
// a local repo (distro unset), or its shell-side UNC form
// (\\wsl.localhost\<distro>\...) for a WSL repo. A path already in UNC form,
// e.g. one the folder picker returned, is left as-is.
export function repoPath(path: string, distro?: string): string {
  if (!distro || path.startsWith('\\\\')) return path
  return `\\\\wsl.localhost\\${distro}${path.replaceAll('/', '\\')}`
}
