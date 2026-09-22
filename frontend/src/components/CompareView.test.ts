import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareCommits } = vi.hoisted(() => ({ mockCompareCommits: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareCommits: mockCompareCommits },
}))

import CompareView from './CompareView.vue'
import { useCompareStore } from '../stores/compare'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

function row(sha: string, shortSha: string): LogCommit {
  return { sha, shortSha, author: '', authorAt: 0, subject: '', graph: { lane: 0, colour: 0, edges: [] } }
}

const props = { wt: '/repo' }

describe('CompareView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCompareCommits.mockReset()
  })

  it('FR-CMP-01, FR-CMP-03 shows both commits short SHAs in from->to order and the changed files', async () => {
    mockCompareCommits.mockResolvedValue([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])
    const compare = useCompareStore()
    compare.toggleSelect(row('sha-a', 'aaa1111'))
    compare.toggleSelect(row('sha-b', 'bbb2222'))

    const wrapper = mount(CompareView, { props })
    await flushPromises()

    expect(mockCompareCommits).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b')
    const text = wrapper.text()
    expect(text.indexOf('aaa1111')).toBeLessThan(text.indexOf('bbb2222'))
    expect(wrapper.findAll('.files li')).toHaveLength(2)
  })

  it('FR-CMP-03 clicking swap reverses the comparison direction', async () => {
    mockCompareCommits.mockResolvedValue([])
    const compare = useCompareStore()
    compare.toggleSelect(row('sha-a', 'aaa1111'))
    compare.toggleSelect(row('sha-b', 'bbb2222'))

    const wrapper = mount(CompareView, { props })
    await flushPromises()
    mockCompareCommits.mockClear()

    await wrapper.get('[data-test="swap"]').trigger('click')
    await flushPromises()

    expect(mockCompareCommits).toHaveBeenCalledWith('/repo', 'sha-b', 'sha-a')
    const text = wrapper.text()
    expect(text.indexOf('bbb2222')).toBeLessThan(text.indexOf('aaa1111'))
  })

  it('FR-CMP-01 pressing Escape exits compare mode', async () => {
    mockCompareCommits.mockResolvedValue([])
    const compare = useCompareStore()
    compare.toggleSelect(row('sha-a', 'aaa1111'))
    compare.toggleSelect(row('sha-b', 'bbb2222'))

    mount(CompareView, { props })
    await flushPromises()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(compare.selection).toEqual([])
  })
})
