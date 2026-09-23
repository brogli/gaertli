import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareWithWorking, mockOpenFile } = vi.hoisted(() => ({
  mockCompareWithWorking: vi.fn(),
  mockOpenFile: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareWithWorking: mockCompareWithWorking, OpenFile: mockOpenFile },
}))

import CompareWorkingView from './CompareWorkingView.vue'
import { useCompareStore } from '../stores/compare'
import { commitRow } from '../testing/fixtures/log-rows'

const props = { wt: '/repo' }

describe('CompareWorkingView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCompareWithWorking.mockReset()
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
  })

  it('FR-CMP-04 renders the changed files, with no swap button', async () => {
    mockCompareWithWorking.mockResolvedValue([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])
    const compare = useCompareStore()
    compare.startAgainstWorking(commitRow({ sha: 'sha-a', shortSha: 'aaa1111' }))

    const wrapper = mount(CompareWorkingView, { props })
    await flushPromises()

    expect(mockCompareWithWorking).toHaveBeenCalledWith('/repo', 'sha-a')
    expect(wrapper.findAll('.files li')).toHaveLength(2)
    expect(wrapper.find('[data-test="swap"]').exists()).toBe(false)
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile', async () => {
    mockCompareWithWorking.mockResolvedValue([{ path: 'a.txt', status: 'M' }])
    const compare = useCompareStore()
    compare.startAgainstWorking(commitRow({ sha: 'sha-a', shortSha: 'aaa1111' }))

    const wrapper = mount(CompareWorkingView, { props })
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
  })

  it('FR-CMP-04 notes that new, uncommitted files are not part of this comparison', async () => {
    mockCompareWithWorking.mockResolvedValue([])
    const compare = useCompareStore()
    compare.startAgainstWorking(commitRow({ sha: 'sha-a', shortSha: 'aaa1111' }))

    const wrapper = mount(CompareWorkingView, { props })
    await flushPromises()

    expect(wrapper.text()).toContain('untracked')
  })

  it('FR-CMP-04 clicking exit leaves the working-tree compare', async () => {
    mockCompareWithWorking.mockResolvedValue([])
    const compare = useCompareStore()
    compare.startAgainstWorking(commitRow({ sha: 'sha-a', shortSha: 'aaa1111' }))

    const wrapper = mount(CompareWorkingView, { props })
    await flushPromises()

    await wrapper.get('[data-test="exit"]').trigger('click')

    expect(compare.workingCommit).toBeNull()
  })
})
