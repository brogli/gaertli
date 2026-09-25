import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import AutoComplete from 'openvue/autocomplete'
import MultiSelect from 'openvue/multiselect'

const mockApplyFilter = vi.fn()
const mockClearQuery = vi.fn(() => {
  mockLogStore.query = ''
  mockLogStore.matches = []
  mockLogStore.current = 0
})
// nextMatch/prevMatch step with the same wrap the real store uses, so the
// component's "n of m" label can be checked after a keypress.
const mockNextMatch = vi.fn(() => {
  mockLogStore.current = (mockLogStore.current + 1) % mockLogStore.matches.length
})
const mockPrevMatch = vi.fn(() => {
  mockLogStore.current =
    (mockLogStore.current - 1 + mockLogStore.matches.length) % mockLogStore.matches.length
})

const mockLogStore: {
  filter: { author?: string; branches?: string[]; path?: string }
  filterActive: boolean
  knownAuthors: string[]
  query: string
  matches: number[]
  current: number
  applyFilter: typeof mockApplyFilter
  clearQuery: typeof mockClearQuery
  nextMatch: typeof mockNextMatch
  prevMatch: typeof mockPrevMatch
} = reactive({
  filter: {},
  filterActive: false,
  knownAuthors: [],
  query: '',
  matches: [],
  current: 0,
  applyFilter: mockApplyFilter,
  clearQuery: mockClearQuery,
  nextMatch: mockNextMatch,
  prevMatch: mockPrevMatch,
})

const mockRefsStore: { refs: { branches: unknown[]; remotes: unknown[]; tags: unknown[] } } = {
  refs: { branches: [], remotes: [], tags: [] },
}

vi.mock('../stores/log', () => ({ useLogStore: () => mockLogStore }))
vi.mock('../stores/refs', () => ({ useRefsStore: () => mockRefsStore }))

import FilterBar from './FilterBar.vue'

describe('FilterBar', () => {
  beforeEach(() => {
    mockApplyFilter.mockReset()
    mockClearQuery.mockClear()
    mockNextMatch.mockClear()
    mockPrevMatch.mockClear()
    mockLogStore.filter = {}
    mockLogStore.filterActive = false
    mockLogStore.knownAuthors = []
    mockLogStore.query = ''
    mockLogStore.matches = []
    mockLogStore.current = 0
    mockRefsStore.refs = { branches: [], remotes: [], tags: [] }
  })

  it('FR-LOG-05 shows "n of m", steps with Enter/Shift+Enter, and Escape clears the query', async () => {
    mockLogStore.matches = [3, 9, 20]
    const wrapper = mount(FilterBar)
    const input = wrapper.find('input[placeholder="Search message"]')

    await input.setValue('fix')
    expect(wrapper.get('[data-test="match-label"]').text()).toBe('1 of 3')

    await input.trigger('keydown', { key: 'Enter' })
    expect(mockNextMatch).toHaveBeenCalledTimes(1)
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-test="match-label"]').text()).toBe('2 of 3')

    mockLogStore.current = 0 // fresh "1 of 3" again
    await input.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(mockPrevMatch).toHaveBeenCalledTimes(1)
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-test="match-label"]').text()).toBe('3 of 3') // wraps back from the first

    await input.trigger('keydown', { key: 'Escape' })
    expect(mockClearQuery).toHaveBeenCalledTimes(1)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-test="match-label"]').exists()).toBe(false)
  })

  it('FR-LOG-06 the branch MultiSelect can be searched, has no select-all, and clears itself', () => {
    const wrapper = mount(FilterBar)
    const props = wrapper.findComponent(MultiSelect).props()

    expect(props.filter).toBe(true)
    expect(props.showToggleAll).toBe(false)
    expect(props.showClear).toBe(true)
  })

  it('FR-LOG-06 the options list local branches, remote branches and tags in groups, by short name, standing for full ref names', () => {
    mockRefsStore.refs = {
      branches: [{ name: 'main' }, { name: 'v1' }],
      remotes: [{ name: 'origin', branches: [{ name: 'main' }] }],
      tags: [{ name: 'v1' }],
    }
    const wrapper = mount(FilterBar)
    const props = wrapper.findComponent(MultiSelect).props()

    expect(props.optionGroupLabel).toBe('label')
    expect(props.optionGroupChildren).toBe('items')
    expect(props.options).toEqual([
      {
        label: 'Local',
        items: [
          { label: 'main', value: 'refs/heads/main' },
          { label: 'v1', value: 'refs/heads/v1' },
        ],
      },
      { label: 'Remote', items: [{ label: 'origin/main', value: 'refs/remotes/origin/main' }] },
      { label: 'Tags', items: [{ label: 'v1', value: 'refs/tags/v1' }] },
    ])
  })

  it('FR-LOG-06 picking a tag applies it by its full ref name, keeping the other fields', async () => {
    mockLogStore.filter = { path: 'src/' }
    const wrapper = mount(FilterBar)

    wrapper.findComponent(MultiSelect).vm.$emit('update:modelValue', ['refs/tags/v1'])
    await wrapper.vm.$nextTick()

    expect(mockApplyFilter).toHaveBeenCalledWith({ path: 'src/', branches: ['refs/tags/v1'] })
  })

  it('FR-LOG-06 a branch or tag filter set elsewhere shows in the field', async () => {
    const wrapper = mount(FilterBar)

    mockLogStore.filter = { branches: ['refs/tags/v1'] }
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(MultiSelect).props('modelValue')).toEqual(['refs/tags/v1'])
  })

  it('FR-LOG-06 selecting branches in the MultiSelect applies immediately', async () => {
    const wrapper = mount(FilterBar)

    wrapper
      .findComponent(MultiSelect)
      .vm.$emit('update:modelValue', ['refs/heads/main', 'refs/remotes/origin/main'])
    await wrapper.vm.$nextTick()

    expect(mockApplyFilter).toHaveBeenCalledTimes(1)
    expect(mockApplyFilter).toHaveBeenCalledWith(
      expect.objectContaining({ branches: ['refs/heads/main', 'refs/remotes/origin/main'] }),
    )
  })

  it("FR-LOG-05 clicking the message field's clear icon clears the query, not the branches/author/path filter", async () => {
    mockLogStore.query = 'fix'
    const wrapper = mount(FilterBar)

    await wrapper.get('[data-test="clear-query"]').trigger('click')

    expect(mockClearQuery).toHaveBeenCalledTimes(1)
    expect(mockApplyFilter).not.toHaveBeenCalled()
  })

  it('FR-LOG-08 clearing the path keeps the other active filters', async () => {
    mockLogStore.filter = { path: 'src/', branches: ['refs/heads/main'] }
    const wrapper = mount(FilterBar)

    await wrapper.get('[data-test="clear-path"]').trigger('click')

    expect(mockApplyFilter).toHaveBeenCalledWith({ branches: ['refs/heads/main'] })
  })

  it('FR-LOG-08 an empty message field shows no clear icon', () => {
    const wrapper = mount(FilterBar)

    expect(wrapper.find('[data-test="clear-query"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="clear-path"]').exists()).toBe(false)
  })

  it('FR-LOG-08 shows no chip list and no clear-filters button', () => {
    mockLogStore.filter = { path: 'y' }
    mockLogStore.query = 'x'
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
