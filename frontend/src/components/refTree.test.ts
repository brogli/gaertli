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
})
