import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCommandLog } = vi.hoisted(() => ({ mockCommandLog: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CommandLog: mockCommandLog },
}))

import CommandLogPanel from './CommandLogPanel.vue'

describe('CommandLogPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCommandLog.mockReset()
    mockCommandLog.mockResolvedValue([
      { at: '', command: 'git status', dir: '', background: false, durationMs: 0, exitCode: 0 },
      { at: '', command: 'git fetch', dir: '', background: true, durationMs: 0, exitCode: 0 },
    ])
  })

  it('FR-UX-02 hides background commands by default, shows them once toggled', async () => {
    const wrapper = mount(CommandLogPanel, { props: { wt: '/repo' } })
    await flushPromises()

    expect(wrapper.text()).toContain('git status')
    expect(wrapper.text()).not.toContain('git fetch')

    await wrapper.find('input[type=checkbox]').setValue(true)

    expect(wrapper.text()).toContain('git status')
    expect(wrapper.text()).toContain('git fetch')
  })
})
