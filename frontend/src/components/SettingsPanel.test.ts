import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockSet, mockListWSLDistros, mockInstallWSLLauncher, mockTestEditorProfile, mockVersion } =
  vi.hoisted(() => ({
    mockGet: vi.fn(),
    mockSet: vi.fn(),
    mockListWSLDistros: vi.fn(),
    mockInstallWSLLauncher: vi.fn(),
    mockTestEditorProfile: vi.fn(),
    mockVersion: vi.fn(),
  }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  SettingsService: { Get: mockGet, Set: mockSet },
  AppService: {
    ListWSLDistros: mockListWSLDistros,
    InstallWSLLauncher: mockInstallWSLLauncher,
    TestEditorProfile: mockTestEditorProfile,
  },
  Facade: { Version: mockVersion },
}))

import SettingsPanel from './SettingsPanel.vue'
import ErrorBanner from './ErrorBanner.vue'
import { defaultSettingsFixture } from '../testing/fixtures/settings'

const filled = defaultSettingsFixture({ protectedBranches: ['main'] })

async function mountPanel() {
  const wrapper = mount(SettingsPanel)
  await flushPromises()
  return wrapper
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGet.mockReset()
    mockSet.mockReset()
    mockGet.mockResolvedValue(filled)
    mockSet.mockResolvedValue(undefined)
    mockListWSLDistros.mockReset()
    mockListWSLDistros.mockResolvedValue([])
    mockInstallWSLLauncher.mockReset()
    mockInstallWSLLauncher.mockResolvedValue(undefined)
    mockTestEditorProfile.mockReset()
    mockTestEditorProfile.mockResolvedValue(undefined)
    mockVersion.mockReset()
    mockVersion.mockResolvedValue({ app: 'v0.1.0', protocol: 1 })
  })

  afterEach(() => {
    document.documentElement.classList.remove('nebegit-dark')
  })

  it('FR-UX-04 loads settings and submits the edited form', async () => {
    const wrapper = await mountPanel()

    await wrapper.find('input[type=number]').setValue(5)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    expect(mockSet.mock.calls[0]?.[0]).toMatchObject({ ...filled, autoFetchIntervalMinutes: 5 })
  })

  it('FR-UX-04 the form appears only once it holds the stored settings, so no early choice is overwritten', async () => {
    let resolve!: (settings: typeof filled) => void
    mockGet.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const wrapper = mount(SettingsPanel)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)

    resolve(filled)
    await flushPromises()

    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('FR-UX-04 saving successfully emits close', async () => {
    const wrapper = await mountPanel()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-REPO-06 clicking "Install nebegit command" calls InstallWSLLauncher with the distro name', async () => {
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, default: true }])

    const wrapper = await mountPanel()

    const button = wrapper.findAll('button').find((b) => b.text() === 'Install nebegit command')
    expect(button).toBeTruthy()
    await button!.trigger('click')

    expect(mockInstallWSLLauncher).toHaveBeenCalledTimes(1)
    expect(mockInstallWSLLauncher).toHaveBeenCalledWith('Ubuntu-22.04')
  })

  it('FR-ED-04, FR-ED-11 renders one independently editable form per environment, submitting the whole updated map', async () => {
    mockGet.mockResolvedValue(
      defaultSettingsFixture({
        editorProfiles: {
          '': { name: 'Zed', folderCmd: 'zed {path}', fileCmd: 'zed {path}', fileAtLineCmd: 'zed {path}:{line}' },
          'Ubuntu-22.04': { name: 'VS Code', folderCmd: 'code {path}', fileCmd: 'code {path}' },
        },
      }),
    )

    const wrapper = await mountPanel()

    const fieldsets = wrapper.findAll('fieldset')
    expect(fieldsets).toHaveLength(2)
    expect(wrapper.text()).toContain('Local')
    expect(wrapper.text()).toContain('Ubuntu-22.04')

    const local = fieldsets.find((f) => f.text().includes('Local'))!
    await local.get('[data-test="folder-cmd"]').setValue('subl {path}')

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    const sent = mockSet.mock.calls[0]?.[0]
    expect(sent.editorProfiles['']).toMatchObject({ name: 'Zed', folderCmd: 'subl {path}' })
    expect(sent.editorProfiles['Ubuntu-22.04']).toMatchObject({ name: 'VS Code', folderCmd: 'code {path}' })
  })

  it('FR-ED-11 adding a distro profile seeds it with the built-in default, unsaved until submit', async () => {
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, default: true }])

    const wrapper = await mountPanel()

    expect(wrapper.findAll('fieldset')).toHaveLength(1) // Local only, so far

    const addButton = wrapper
      .findAll('[data-test="add-distro-profile"]')
      .find((b) => b.text() === 'Ubuntu-22.04')
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')

    const fieldsets = wrapper.findAll('fieldset')
    expect(fieldsets).toHaveLength(2)
    const added = fieldsets.find((f) => f.text().includes('Ubuntu-22.04'))!
    expect(added.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value).toBe('zed {path}')

    expect(mockSet).not.toHaveBeenCalled()
  })

  it('FR-UX-01 a rejected save renders ErrorBanner with the error', async () => {
    mockSet.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'settings write failed' } }),
    )
    const wrapper = await mountPanel()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'settings write failed' })
  })

  it('FR-UX-06 picking Dark and saving persists it and applies the theme immediately', async () => {
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="theme-select"]').setValue('dark')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    expect(mockSet.mock.calls[0]?.[0]).toMatchObject({ theme: 'dark' })
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)
  })

  it('FR-ED-07 clicking the "VS Code" preset fills fileCmd and fileAtLineCmd', async () => {
    const wrapper = await mountPanel()

    const button = wrapper.findAll('[data-test="editor-preset"]').find((b) => b.text() === 'VS Code')
    expect(button).toBeTruthy()
    await button!.trigger('click')

    const fieldset = wrapper.find('fieldset')
    expect(fieldset.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value).toBe('code {path}')
    const inputs = fieldset.findAll<HTMLInputElement>('input[type=text]')
    expect(inputs[2]!.element.value).toBe('code {path}') // File command
    expect(inputs[3]!.element.value).toBe('code -g {path}:{line}') // File-at-line command
  })

  it('FR-ED-07 clicking "Neovim (terminal)" after "VS Code" fully overwrites, not merges', async () => {
    const wrapper = await mountPanel()

    await wrapper.findAll('[data-test="editor-preset"]').find((b) => b.text() === 'VS Code')!.trigger('click')
    await wrapper
      .findAll('[data-test="editor-preset"]')
      .find((b) => b.text() === 'Neovim (terminal)')!
      .trigger('click')

    const fieldset = wrapper.find('fieldset')
    expect(fieldset.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value).toBe(
      'x-terminal-emulator -e nvim {path}',
    )
    const inputs = fieldset.findAll<HTMLInputElement>('input[type=text]')
    expect(inputs[2]!.element.value).toBe('x-terminal-emulator -e nvim {path}')
    expect(inputs[3]!.element.value).toBe('x-terminal-emulator -e nvim +{line} {path}')
  })

  it('FR-ED-08 clicking "Test" on the Local fieldset calls AppService.TestEditorProfile with "" and the current profile', async () => {
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="test-editor-profile"]').trigger('click')

    expect(mockTestEditorProfile).toHaveBeenCalledTimes(1)
    expect(mockTestEditorProfile).toHaveBeenCalledWith(
      '',
      expect.objectContaining({ name: 'Zed', folderCmd: 'zed {path}' }),
    )
  })

  it('FR-ED-08 a failed "Test" shows the error with no worktree-only fallback buttons', async () => {
    mockTestEditorProfile.mockRejectedValue(
      Object.assign(new Error('editor not found'), { cause: { message: 'no editor', code: 'EDITOR_NOT_FOUND' } }),
    )
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="test-editor-profile"]').trigger('click')
    await flushPromises()

    expect(wrapper.findComponent(ErrorBanner).props('editorActions')).toBe(false)
    const labels = wrapper.findAll('button').map((b) => b.text())
    expect(labels).not.toContain("Open with git's editor")
    expect(labels).not.toContain('Open with system default')
  })

  it('FR-DIFF-06 checking "Ignore whitespace" and saving persists it', async () => {
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="ignore-whitespace"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    expect(mockSet.mock.calls[0]?.[0]).toMatchObject({ ignoreWhitespace: true })
  })

  it('QA-INST-03 renders the app version fetched on mount', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain('v0.1.0')
  })
})
