// basename is the last segment of a Windows or POSIX path; the path itself
// when it has no separator.
export function basename(path: string): string {
  return path.split(/[\\/]/).pop() || path
}
