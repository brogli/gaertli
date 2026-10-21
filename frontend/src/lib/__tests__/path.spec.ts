// Requirements: FR-UX-09, FR-WT-13, FR-WT-18
import { describe, expect, it } from 'vitest'
import { basename, joinPath, lastSegment, parentPath, repoPath, withLastSegment } from '../path'

describe('basename and parentPath', () => {
  it.each([
    ['POSIX', '/home/u/repo', '/home/u', 'repo'],
    ['POSIX root child', '/repo', '/', 'repo'],
    ['Windows', 'C:\\Users\\u\\repo', 'C:\\Users\\u', 'repo'],
    ['UNC', '\\\\wsl.localhost\\Ubuntu\\home\\x', '\\\\wsl.localhost\\Ubuntu\\home', 'x'],
    ['drive root child', 'C:\\repo', 'C:', 'repo'],
    ['drive root', 'C:\\', '', 'C:'],
    ['POSIX root', '/', '', '/'],
    ['trailing separator', '/home/u/repo/', '/home/u', 'repo'],
    ['Windows trailing separator', 'C:\\Users\\u\\repo\\', 'C:\\Users\\u', 'repo'],
    ['relative', 'docs/readme.md', 'docs', 'readme.md'],
    ['no separator', 'repo', '', 'repo'],
  ])('splits a %s path', (_, path, parent, name) => {
    expect(parentPath(path)).toBe(parent)
    expect(basename(path)).toBe(name)
  })
})

describe('repoPath', () => {
  it('returns a local path unchanged (no distro)', () => {
    expect(repoPath('/home/u/r')).toBe('/home/u/r')
  })

  it('builds the UNC form for a WSL repo', () => {
    expect(repoPath('/home/u/r', 'Ubuntu')).toBe('\\\\wsl.localhost\\Ubuntu\\home\\u\\r')
  })

  it('leaves an already-UNC path unchanged', () => {
    const unc = '\\\\wsl.localhost\\Ubuntu\\home\\u\\r'
    expect(repoPath(unc, 'Ubuntu')).toBe(unc)
  })
})

describe('joinPath', () => {
  it.each([
    ['POSIX', '/home/u', 'wt', '/home/u/wt'],
    ['POSIX trailing separator', '/home/u/', 'wt', '/home/u/wt'],
    ['POSIX root', '/', 'wt', '/wt'],
    ['Windows', 'C:\\Users\\u', 'wt', 'C:\\Users\\u\\wt'],
    ['drive root', 'C:\\', 'wt', 'C:\\wt'],
    ['UNC', '\\\\wsl.localhost\\Ubuntu\\home', 'wt', '\\\\wsl.localhost\\Ubuntu\\home\\wt'],
    ['no folder', '', 'wt', 'wt'],
  ])("joins a %s folder and a name in the folder's style", (_, folder, name, joined) => {
    expect(joinPath(folder, name)).toBe(joined)
  })
})

describe('lastSegment and withLastSegment', () => {
  it.each([
    ['POSIX', '/home/u/wt', 'wt', '/home/u/x'],
    ['Windows', 'C:\\Users\\wt', 'wt', 'C:\\Users\\x'],
    ['trailing separator', '/home/u/', '', '/home/u/x'],
    ['no separator', 'wt', 'wt', 'x'],
    ['empty', '', '', 'x'],
  ])(
    'reads and replaces the raw text after the last separator of a %s path',
    (_, path, last, replaced) => {
      expect(lastSegment(path)).toBe(last)
      expect(withLastSegment(path, 'x')).toBe(replaced)
      expect(withLastSegment(path, lastSegment(path))).toBe(path)
    },
  )
})
