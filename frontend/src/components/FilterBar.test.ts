import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MultiSelect from 'openvue/multiselect'

const mockApplyFilter = vi.fn()
const mockClearFilter = vi.fn()

const mockLogStore: {
  filter: { query?: string; author?: string; branches?: string[]; path?: string }
  filterActive: boolean
  knownAuthors: string[]
  applyFilter: typeof mockApplyFilter
  clearFilter: typeof mockClearFilter
} = {
  filter: {},
  filterActive: false,
  knownAuthors: [],
  applyFilter: mockApplyFilter,
  clearFilter: mockClearFilter,
}

const mockRefsStore: { refs: { branches: unknown[]; remotes: unknown[]; tags: unknown[] } } = {
  refs: { branches: [], remotes: [], tags: [] },
}

vi.mock('../stores/log', () => ({ useLogStore: () => mockLogStore }))
vi.mock('../stores/refs', () => ({ useRefsStore: () => mockRefsStore }))

import FilterBar from './FilterBar.vue'

describe('FilterBar', () => {
  beforeEach(() => {
    mockApplyFilter.mockReset()
    mockClearFilter.mockReset()
    mockLogStore.filter = {}
    mockLogStore.filterActive = false
    mockLogStore.knownAuthors = []
    mockRefsStore.refs = { branches: [], remotes: [], tags: [] }
  })

  it('FR-LOG-05 debounces the Query input, applying only the final text', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = mount(FilterBar)

      const input = wrapper.find('input[placeholder="Search message"]')
      await input.setValue('bu')
      await vi.advanceTimersByTimeAsync(200) // less than the 300ms debounce
      expect(mockApplyFilter).not.toHaveBeenCalled()

      await input.setValue('bug')
      await vi.advanceTimersByTimeAsync(300)

      expect(mockApplyFilter).toHaveBeenCalledTimes(1)
      expect(mockApplyFilter).toHaveBeenCalledWith(expect.objectContaining({ query: 'bug' }))
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-LOG-06 selecting branches in the MultiSelect applies immediately, local and remote names', async () => {
    mockRefsStore.refs = {
      branches: [{ name: 'main' }, { name: 'feature' }],
      remotes: [{ name: 'origin', branches: [{ name: 'main' }] }],
      tags: [],
    }
    const wrapper = mount(FilterBar)

    wrapper.findComponent(MultiSelect).vm.$emit('update:modelValue', ['main', 'origin/main'])
    await wrapper.vm.$nextTick()

    expect(mockApplyFilter).toHaveBeenCalledTimes(1)
    expect(mockApplyFilter).toHaveBeenCalledWith(
      expect.objectContaining({ branches: ['main', 'origin/main'] }),
    )
  })

  it('FR-LOG-08 shows a chip per active filter and clears them all on click', async () => {
    mockLogStore.filter = { query: 'x', path: 'y' }
    mockLogStore.filterActive = true
    const wrapper = mount(FilterBar)

    const chips = wrapper.findAll('.p-chip')
    expect(chips).toHaveLength(2)

    await wrapper.get('[data-test="clear-filters"]').trigger('click')
    expect(mockClearFilter).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-08 Escape inside the bar clears the filter', async () => {
    mockLogStore.filter = { query: 'x' }
    mockLogStore.filterActive = true
    const wrapper = mount(FilterBar)

    await wrapper.get('.filter-bar').trigger('keydown.escape')

    expect(mockClearFilter).toHaveBeenCalledTimes(1)
  })
})
