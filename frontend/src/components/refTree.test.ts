import { describe, expect, it } from 'vitest'
import { buildRefTree } from './refTree'

describe('buildRefTree', () => {
  it('FR-LOG-01 folds a shared namespace prefix into one folder node', () => {
    const tree = buildRefTree([
      { name: 'feature/a', sha: 'sha-a' },
      { name: 'feature/b', sha: 'sha-b' },
      { name: 'main', sha: 'sha-main' },
    ])

    expect(tree.map((node) => node.label)).toEqual(['feature', 'main'])

    const feature = tree[0]
    expect(feature?.data).toBeUndefined()
    expect(feature?.children?.map((child) => child.label)).toEqual(['a', 'b'])
    expect(feature?.children?.[0]?.data?.sha).toBe('sha-a')
    expect(feature?.children?.[1]?.data?.sha).toBe('sha-b')

    const main = tree[1]
    expect(main?.children).toBeUndefined()
    expect(main?.data?.sha).toBe('sha-main')
  })

  it('FR-UX-06 marks a childless node as a leaf', () => {
    const tree = buildRefTree([{ name: 'main', sha: 'sha-main' }])

    expect(tree[0]?.leaf).toBe(true)
  })

  it('FR-UX-06 leaves a folder node unmarked, marking only its leaf children', () => {
    const tree = buildRefTree([
      { name: 'feature/a', sha: 'sha-a' },
      { name: 'feature/b', sha: 'sha-b' },
    ])

    const feature = tree[0]!
    expect(feature.leaf).toBeFalsy()
    expect(feature.children?.every((child) => child.leaf)).toBe(true)
  })
})
