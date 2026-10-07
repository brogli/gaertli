// Git's conflict marker lines. A marker line is a run of "<" or ">" of the
// marker size, alone or followed by a space; a trailing "\r" ends it like the
// line end does.

// DEFAULT_MARKER_SIZE is the size of git's markers unless the
// conflict-marker-size attribute says otherwise.
export const DEFAULT_MARKER_SIZE = 7

export function isMarkerLine(line: string, size: number): boolean {
  return isRun(line, '<', size) || isRun(line, '>', size)
}

function isRun(line: string, ch: string, size: number): boolean {
  if (line.length < size || line.slice(0, size) !== ch.repeat(size)) return false
  const next = line[size]
  return next === undefined || next === ' ' || next === '\r'
}
