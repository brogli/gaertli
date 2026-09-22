import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockSet, mockListWSLDistros, mockInstallWSLLauncher } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockSet: vi.fn(),
  mockListWSLDistros: vi.fn(),
  mockInstallWSLLauncher: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  SettingsService: { Get: mockGet, Set: mockSet },
  AppService: { ListWSLDistros: mockListWSLDistros, InstallWSLLauncher: mockInstallWSLLauncher },
}))

import SettingsPanel from './SettingsPanel.vue'
import { defaultSettingsFixture } from '../testing/fixtures/settings'

const filled = defaultSettingsFixture({ protectedBranches: ['main'] })

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
  })

  it('FR-UX-04 loads settings and submits the edited form', async () => {
    const wrapper = mount(SettingsPanel)
    await flushPromises()

    await wrapper.find('input[type=number]').setValue(5)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockSet).toHaveBeenCalledTimes(1)
    expect(mockSet.mock.calls[0]?.[0]).toMatchObject({ ...filled, autoFetchIntervalMinutes: 5 })
  })

  it('FR-REPO-06 clicking "Install nebegit command" calls InstallWSLLauncher with the distro name', async () => {
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, default: true }])

    const wrapper = mount(SettingsPanel)
    await flushPromises()

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

    const wrapper = mount(SettingsPanel)
    await flushPromises()

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

    const wrapper = mount(SettingsPanel)
    await flushPromises()

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
})
