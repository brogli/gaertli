// The graph draws each lane in one of a small set of colours
// (`GraphCell.vue`'s `.colour-N`). A lane's own colour number only ever
// grows, so once open lanes outnumber the palette it repeats.
export const LANE_PALETTE_SIZE = 6

export function laneColourClass(colour: number): string {
  return `colour-${colour % LANE_PALETTE_SIZE}`
}
