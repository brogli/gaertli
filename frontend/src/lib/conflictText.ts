import type {
  ConflictFile,
  LineRange,
} from '../../bindings/github.com/brogli/gaertli/internal/core'

export type Take = 'current' | 'incoming' | 'currentFirst' | 'incomingFirst'

type Side = 'base' | 'current' | 'incoming'

// linesIn returns the lines range selects in content.
function linesIn(content: string | undefined, range: LineRange): string[] {
  if (range.lines <= 0 || !content) return []
  return content.split('\n').slice(range.start - 1, range.start - 1 + range.lines)
}

function changeLines(file: ConflictFile, change: number, side: Side): string[] {
  const range = file.changes?.[change]?.[side]
  return range ? linesIn(file[side].content, range) : []
}

// resultFromBase is the working file with each change's lines (a conflict's
// marker block, or what git applied of a change) replaced by the base
// version's lines (none without a base), and the line ranges the changes then
// take.
export function resultFromBase(file: ConflictFile): { text: string; changes: LineRange[] } {
  const lines = (file.result ?? '').split('\n')
  const out: string[] = []
  const changes: LineRange[] = []
  let next = 0
  file.changes?.forEach((change, i) => {
    const before = change.result.lines > 0 ? change.result.start - 1 : change.result.start
    out.push(...lines.slice(next, before))
    const base = changeLines(file, i, 'base')
    changes.push({ start: base.length > 0 ? out.length + 1 : out.length, lines: base.length })
    out.push(...base)
    next = before + change.result.lines
  })
  out.push(...lines.slice(next))
  return { text: out.join('\n'), changes }
}

// takeSides are the sides whose lines a take puts into a change, in order.
const takeSides: Record<Take, Side[]> = {
  current: ['current'],
  incoming: ['incoming'],
  currentFirst: ['current', 'incoming'],
  incomingFirst: ['incoming', 'current'],
}

// takenLines are the lines a take puts into a change.
export function takenLines(file: ConflictFile, change: number, take: Take): string[] {
  return takeSides[take].flatMap((side) => changeLines(file, change, side))
}
