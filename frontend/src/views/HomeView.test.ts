import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockPickAndOpen, mockRecentRepos, mockOpenRecent } = vi.hoisted(() => ({
  mockPickAndOpen: vi.fn(),
  mockRecentRepos: vi.fn(),
  mockOpenRecent: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    PickAndOpen: mockPickAndOpen,
    RecentRepos: mockRecentRepos,
    OpenRecent: mockOpenRecent,
  },
}))

import HomeView from './HomeView.vue'

describe('HomeView', () => {
  beforeEach(() => {
    mockPickAndOpen.mockReset().mockResolvedValue(undefined)
    mockRecentRepos.mockReset().mockResolvedValue([])
    mockOpenRecent.mockReset().mockResolvedValue(undefined)
  })

  it('FR-REPO-01 opens the folder picker on click', async () => {
    const wrapper = mount(HomeView)

    await wrapper.get('button').trigger('click')

    expect(mockPickAndOpen).toHaveBeenCalledTimes(1)
  })

  it('FR-REPO-02 lists recent repos and opens the clicked one', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/a', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/b', lastOpened: '2026-01-02T00:00:00Z' },
    ])

    const wrapper = mount(HomeView)
    await flushPromises()

    expect(wrapper.text()).toContain('/a')
    expect(wrapper.text()).toContain('/b')

    await wrapper.get('li button').trigger('click')

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent).toHaveBeenCalledWith('/a')
  })
})
