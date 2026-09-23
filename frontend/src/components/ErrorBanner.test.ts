import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ErrorBanner from './ErrorBanner.vue'
import { openSettingsKey } from '../lib/openSettings'

const editorNotFound = { message: 'no editor configured', code: 'EDITOR_NOT_FOUND' }

// withSettings provides MainView's open-settings action, as every banner
// in a main window has it.
function withSettings(openSettings = vi.fn()) {
  return { provide: { [openSettingsKey as symbol]: openSettings } }
}

describe('ErrorBanner', () => {
  it("FR-UX-01 details show the command and git's full output", async () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: 'GIT_FAILED', message: "a branch named 'main' already exists", command: 'git branch main', gitStderr: "fatal: a branch named 'main' already exists\n" } },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.get('p').text()).toBe("a branch named 'main' already exists")
    expect(wrapper.get('pre').text()).toContain('git branch main')
    expect(wrapper.get('pre').text()).toContain("fatal: a branch named 'main' already exists")
  })

  it('FR-ED-06 shows an "Open editor setting" button for EDITOR_NOT_FOUND that opens Settings', async () => {
    const openSettings = vi.fn()
    const wrapper = mount(ErrorBanner, { props: { error: editorNotFound }, global: withSettings(openSettings) })

    const button = wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(openSettings).toHaveBeenCalledTimes(1)
  })

  it('FR-ED-06 shows no "Open editor setting" button where there is no Settings dialog to open', () => {
    const wrapper = mount(ErrorBanner, { props: { error: editorNotFound } })

    expect(wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')).toBeUndefined()
  })

  it('FR-ED-06 shows no "Open editor setting" button for a different error code', () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { message: 'not a git repository', code: 'NOT_A_REPO' } },
    })

    expect(wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')).toBeUndefined()
  })

  it('FR-ED-10 shows both fallback buttons alongside "Open editor setting" for EDITOR_NOT_FOUND', () => {
    const wrapper = mount(ErrorBanner, { props: { error: editorNotFound }, global: withSettings() })

    const labels = wrapper.findAll('button').map((b) => b.text())
    expect(labels).toContain('Open editor setting')
    expect(labels).toContain("Open with git's editor")
    expect(labels).toContain('Open with system default')
  })

  it('FR-ED-10 clicking each fallback button emits fallback with its own kind', async () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { message: 'no editor configured', code: 'EDITOR_NOT_FOUND' } },
    })

    await wrapper.findAll('button').find((b) => b.text() === "Open with git's editor")!.trigger('click')
    await wrapper.findAll('button').find((b) => b.text() === 'Open with system default')!.trigger('click')

    expect(wrapper.emitted('fallback')).toEqual([['git'], ['system']])
  })

  it('FR-ED-08 editorActions false hides all three editor buttons even for EDITOR_NOT_FOUND', () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: editorNotFound, editorActions: false },
      global: withSettings(),
    })

    const labels = wrapper.findAll('button').map((b) => b.text())
    expect(labels).not.toContain('Open editor setting')
    expect(labels).not.toContain("Open with git's editor")
    expect(labels).not.toContain('Open with system default')
  })
})
