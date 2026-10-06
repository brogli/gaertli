// Git's conflict marker blocks in a text: merge, diff3 and zdiff3 styles,
// any marker size. Lines split at "\n" only; a trailing "\r" ends a marker
// line like the line end does.

export interface MarkerBlock {
  // 1-based, inclusive: the line of the opening and of the closing marker.
  startLine: number
  endLine: number
  current: string[]
  base: string[] | null
  incoming: string[]
}

// DEFAULT_MARKER_SIZE is the size of git's markers unless the
// conflict-marker-size attribute says otherwise.
export const DEFAULT_MARKER_SIZE = 7

export type Take = 'current' | 'incoming' | 'currentFirst' | 'incomingFirst'

// isRun reports whether line is exactly size times ch, then a space or the end.
function isRun(line: string, ch: string, size: number): boolean {
  if (line.length < size || line.slice(0, size) !== ch.repeat(size)) return false
  const next = line[size]
  return next === undefined || next === ' ' || next === '\r'
}

export function isMarkerLine(line: string, size: number): boolean {
  return isRun(line, '<', size) || isRun(line, '>', size)
}

export function hasMarkerLines(text: string, size: number): boolean {
  return text.split('\n').some((line) => isMarkerLine(line, size))
}

export function findBlocks(text: string, size: number): MarkerBlock[] {
  const lines = text.split('\n')
  const blocks: MarkerBlock[] = []
  let open = -1
  let current: string[] = []
  let base: string[] | null = null
  let incoming: string[] | null = null

  lines.forEach((line, i) => {
    if (isRun(line, '<', size)) {
      open = i
      current = []
      base = null
      incoming = null
    } else if (open < 0) {
      return
    } else if (incoming === null && base === null && isRun(line, '|', size)) {
      base = []
    } else if (incoming === null && isRun(line, '=', size)) {
      incoming = []
    } else if (incoming !== null && isRun(line, '>', size)) {
      blocks.push({ startLine: open + 1, endLine: i + 1, current, base, incoming })
      open = -1
    } else {
      const section = incoming ?? base ?? current
      section.push(line)
    }
  })
  return blocks
}

export function takeLines(block: MarkerBlock, take: Take): string[] {
  switch (take) {
    case 'current':
      return block.current
    case 'incoming':
      return block.incoming
    case 'currentFirst':
      return [...block.current, ...block.incoming]
    case 'incomingFirst':
      return [...block.incoming, ...block.current]
  }
}
