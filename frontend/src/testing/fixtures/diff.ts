import type { DiffFile, DiffHunk } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// diffFileFixture is a DiffFile answer for a text file within the size cap.
export function diffFileFixture(overrides: Partial<DiffFile> = {}): DiffFile {
  return { path: 'a.txt', binary: false, tooLarge: false, ...overrides }
}

// hunk is one git hunk header.
export function hunk(
  oldStart: number,
  oldLines: number,
  newStart: number,
  newLines: number,
): DiffHunk {
  return { oldStart, oldLines, newStart, newLines }
}
