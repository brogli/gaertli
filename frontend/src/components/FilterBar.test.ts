import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AutoComplete from 'openvue/autocomplete'
import MultiSelect from 'openvue/multiselect'

const mockApplyFilter = vi.fn()

const mockLogStore: {
  filter: { query?: string; author?: string; branches?: string[]; path?: string }
  filterActive: boolean
  knownAuthors: string[]
  applyFilter: typeof mockApplyFilter
} = {
  filter: {},
  filterActive: false,
  knownAuthors: [],
  applyFilter: mockApplyFilter,
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

  it('FR-LOG-06 the branch MultiSelect can be searched, has no select-all, and clears itself', () => {
    const wrapper = mount(FilterBar)
    const props = wrapper.findComponent(MultiSelect).props()

    expect(props.filter).toBe(true)
    expect(props.showToggleAll).toBe(false)
    expect(props.showClear).toBe(true)
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

  it("FR-LOG-08 clicking the message field's clear icon empties it and applies at once", async () => {
    mockLogStore.filter = { query: 'fix' }
    const wrapper = mount(FilterBar)

    await wrapper.get('[data-test="clear-query"]').trigger('click')

    expect(
      (wrapper.get('input[placeholder="Search message"]').element as HTMLInputElement).value,
    ).toBe('')
    expect(mockApplyFilter).toHaveBeenCalledTimes(1)
    expect(mockApplyFilter).toHaveBeenCalledWith({})
  })

  it('FR-LOG-08 clearing the path keeps the other active filters', async () => {
    mockLogStore.filter = { path: 'src/', branches: ['main'] }
    const wrapper = mount(FilterBar)

    await wrapper.get('[data-test="clear-path"]').trigger('click')

    expect(mockApplyFilter).toHaveBeenCalledWith({ branches: ['main'] })
  })

  it('FR-LOG-08 an empty message field shows no clear icon', () => {
    const wrapper = mount(FilterBar)

    expect(wrapper.find('[data-test="clear-query"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="clear-path"]').exists()).toBe(false)
  })

  it('FR-LOG-08 shows no chip list and no clear-filters button', () => {
    mockLogStore.filter = { query: 'x', path: 'y' }
    mockLogStore.filterActive = true
    const wrapper = mount(FilterBar)

    expect(wrapper.find('.p-chip').exists()).toBe(false)
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(false)
  })

  it('FR-LOG-08 clearing the author AutoComplete applies without the author', async () => {
    mockLogStore.filter = { author: 'ann' }
    const wrapper = mount(FilterBar)

    wrapper.findComponent(AutoComplete).vm.$emit('clear')
    await wrapper.vm.$nextTick()

    expect(mockApplyFilter).toHaveBeenCalledWith({})
  })
})
