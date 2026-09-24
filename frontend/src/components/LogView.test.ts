import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    LogIndexOf: vi.fn().mockResolvedValue({ index: 0, found: false }),
  },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))
vi.mock('@wailsio/runtime', () => import('../testing/runtime'))

import LogView from './LogView.vue'
import FilterBar from './FilterBar.vue'
import LogList from './LogList.vue'
import { useLogStore } from '../stores/log'
import { commitRow } from '../testing/fixtures/log-rows'

describe('LogView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.LoadLog.mockReset().mockResolvedValue(undefined)
    Facade.LogRows.mockReset().mockResolvedValue({ rows: [], loaded: 0, done: true })
  })

  it('FR-UX-08 renders the filter bar above the commit list', async () => {
    const wrapper = mount(LogView, { props: { wt: '/repo' } })
    await flushPromises()

    expect(wrapper.findComponent(FilterBar).exists()).toBe(true)
    expect(wrapper.findComponent(LogList).props('wt')).toBe('/repo')
    const html = wrapper.html()
    expect(html.indexOf('filter-bar')).toBeLessThan(html.indexOf('log-list'))
  })

  it("FR-UX-08 forwards LogList's open event", async () => {
    const wrapper = mount(LogView, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, commitRow())
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('dblclick')

    expect(wrapper.emitted('open')).toHaveLength(1)
  })

  it("FR-WT-06 forwards LogList's create-worktree event with the row's sha", () => {
    const wrapper = mount(LogView, { props: { wt: '/repo' } })

    wrapper.findComponent(LogList).vm.$emit('create-worktree', 'abc123')

    expect(wrapper.emitted('create-worktree')).toEqual([['abc123']])
  })
})
