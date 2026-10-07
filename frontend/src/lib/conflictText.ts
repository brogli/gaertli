import type { ConflictFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { LineRange } from './conflictRegions'

export type Take = 'current' | 'incoming' | 'currentFirst' | 'incomingFirst'

type Side = 'base' | 'current' | 'incoming'

// linesIn returns the lines range selects in content.
function linesIn(content: string | undefined, range: LineRange): string[] {
  if (range.lines <= 0 || !content) return []
  return content.split('\n').slice(range.start - 1, range.start - 1 + range.lines)
}

function regionLines(file: ConflictFile, region: number, side: Side): string[] {
  const range = file.regions?.[region]?.[side]
  return range ? linesIn(file[side].content, range) : []
}

// resultFromBase is the working file with each region's lines (a conflict's
// marker block, or what git applied of a change) replaced by the base
// version's lines (none without a base), and the line ranges the regions then
// take.
export function resultFromBase(file: ConflictFile): { text: string; regions: LineRange[] } {
  const lines = (file.result ?? '').split('\n')
  const out: string[] = []
  const regions: LineRange[] = []
  let next = 0
  file.regions?.forEach((region, i) => {
    const before = region.result.lines > 0 ? region.result.start - 1 : region.result.start
    out.push(...lines.slice(next, before))
    const base = regionLines(file, i, 'base')
    regions.push({ start: base.length > 0 ? out.length + 1 : out.length, lines: base.length })
    out.push(...base)
    next = before + region.result.lines
  })
  out.push(...lines.slice(next))
  return { text: out.join('\n'), regions }
}

// takeSides are the sides whose lines a take puts into a region, in order.
const takeSides: Record<Take, Side[]> = {
  current: ['current'],
  incoming: ['incoming'],
  currentFirst: ['current', 'incoming'],
  incomingFirst: ['incoming', 'current'],
}

// takenLines are the lines a take puts into a region.
export function takenLines(file: ConflictFile, region: number, take: Take): string[] {
  return takeSides[take].flatMap((side) => regionLines(file, region, side))
}
