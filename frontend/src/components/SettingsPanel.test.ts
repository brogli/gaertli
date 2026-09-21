import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockSet } = vi.hoisted(() => ({ mockGet: vi.fn(), mockSet: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  SettingsService: { Get: mockGet, Set: mockSet },
}))

import SettingsPanel from './SettingsPanel.vue'

const filled = {
  recentRepos: [],
  openWorktrees: [],
  editor: 'zed',
  protectedBranches: ['main'],
  defaultPullMode: 'merge',
  diffMode: 'unified',
  autoFetchIntervalMinutes: 0,
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGet.mockReset()
    mockSet.mockReset()
    mockGet.mockResolvedValue(filled)
    mockSet.mockResolvedValue(undefined)
  })

  it('FR-UX-04 loads settings and submits the edited form', async () => {
    const wrapper = mount(SettingsPanel)
    await flushPromises()

    await wrapper.find('input[type=text]').setValue('subl')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    expect(mockSet.mock.calls[0]?.[0]).toMatchObject({ ...filled, editor: 'subl' })
  })
})
