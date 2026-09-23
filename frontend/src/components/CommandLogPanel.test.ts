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

  it('FR-UX-10 renders one entry per line, monospace, full command in title, no ul/li', async () => {
    const longCommand =
      'git commit -m "a very long commit message that would wrap onto several lines without truncation"'
    mockCommandLog.mockResolvedValue([
      { at: '', command: 'git status', dir: '', background: false, durationMs: 0, exitCode: 0 },
      { at: '', command: 'git add -A', dir: '', background: false, durationMs: 0, exitCode: 0 },
      { at: '', command: longCommand, dir: '', background: false, durationMs: 0, exitCode: 0 },
    ])

    const wrapper = mount(CommandLogPanel, { props: { wt: '/repo' } })
    await flushPromises()

    expect(wrapper.find('ul').exists()).toBe(false)
    expect(wrapper.find('li').exists()).toBe(false)
    const entries = wrapper.findAll('[data-test="command-log-entry"]')
    expect(entries).toHaveLength(3)
    expect(entries[2]!.attributes('title')).toBe(longCommand)
  })
})
