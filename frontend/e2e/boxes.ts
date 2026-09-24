import { expect } from '@playwright/test'

// expectNoOverlap: sorted left to right, each box's x-range ends at or
// before the next one's begins.
export function expectNoOverlap(boxes: { x: number; width: number }[]): void {
  const sorted = [...boxes].sort((a, b) => a.x - b.x)
  for (let i = 1; i < sorted.length; i++) {
    expect(sorted[i]!.x).toBeGreaterThanOrEqual(sorted[i - 1]!.x + sorted[i - 1]!.width)
  }
}
