import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareCommits, mockOpenCompareDiff, mockOpenFile } = vi.hoisted(() => ({
  mockCompareCommits: vi.fn(),
  mockOpenCompareDiff: vi.fn(),
  mockOpenFile: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareCommits: mockCompareCommits, OpenFile: mockOpenFile },
  AppService: { OpenCompareDiff: mockOpenCompareDiff },
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
    mockOpenCompareDiff.mockReset()
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
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

  it('FR-CMP-02 double-clicking a file opens its own diff window', async () => {
    mockCompareCommits.mockResolvedValue([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
    ])
    const compare = useCompareStore()
    compare.toggleSelect(row('sha-a', 'aaa1111'))
    compare.toggleSelect(row('sha-b', 'bbb2222'))

    const wrapper = mount(CompareView, { props })
    await flushPromises()

    await wrapper.findAll('.files li')[1]!.trigger('dblclick')

    expect(mockOpenCompareDiff).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'b.txt', 'a2.txt')
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile with wt, the path, and line 0', async () => {
    mockCompareCommits.mockResolvedValue([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])
    const compare = useCompareStore()
    compare.toggleSelect(row('sha-a', 'aaa1111'))
    compare.toggleSelect(row('sha-b', 'bbb2222'))

    const wrapper = mount(CompareView, { props })
    await flushPromises()

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'b.txt', 0)
  })
})
