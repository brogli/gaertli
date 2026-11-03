// The graph draws each lane in one of a small set of colours, distinct from
// the diff's green/red and the comparison's from/to blue/orange. A lane's own
// colour number only ever grows, so once open lanes outnumber the palette it
// repeats. Whole class names, so Tailwind finds them in the source.
const LANE_COLOURS = [
  'text-tone-violet',
  'text-tone-teal',
  'text-tone-pink',
  'text-tone-cyan',
  'text-tone-purple',
  'text-tone-indigo',
]

export const LANE_PALETTE_SIZE = LANE_COLOURS.length

export function laneColourClass(colour: number): string {
  return LANE_COLOURS[colour % LANE_PALETTE_SIZE] ?? ''
}
