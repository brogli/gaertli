import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockLoadLog, mockLogRows, mockCommitDetails } = vi.hoisted(() => ({
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockCommitDetails: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { LoadLog: mockLoadLog, LogRows: mockLogRows, CommitDetails: mockCommitDetails },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import LogList from './LogList.vue'
import { useLogStore } from '../stores/log'

describe('LogList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockLoadLog.mockReset()
    mockLoadLog.mockResolvedValue(undefined)
    mockLogRows.mockReset()
    mockLogRows.mockResolvedValue({ rows: [], loaded: 0, done: true })
    mockCommitDetails.mockReset()
    mockCommitDetails.mockResolvedValue({
      sha: 'abc123',
      parents: [],
      author: '',
      authorEmail: '',
      authorAt: 0,
      committer: '',
      committerEmail: '',
      committerAt: 0,
      body: '',
      files: [],
    })
  })

  it('FR-LOG-12 copies the selected row hash and subject', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const wrapper = mount(LogList, { props: { wt: '/repo' } })
    await flushPromises()

    const log = useLogStore()
    await log.select({
      sha: 'abc123',
      shortSha: 'abc123',
      author: 'Alice',
      authorAt: 0,
      subject: 'Fix the bug',
      graph: { lane: 0, colour: 0, edges: [] },
    })
    await wrapper.vm.$nextTick()

    await wrapper.get('[data-test="copy-hash"]').trigger('click')
    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith('abc123')

    await wrapper.get('[data-test="copy-subject"]').trigger('click')
    expect(writeText).toHaveBeenCalledTimes(2)
    expect(writeText).toHaveBeenCalledWith('Fix the bug')
  })
})
