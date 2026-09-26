import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { SettingsService, AppService, Facade } = vi.hoisted(() => ({
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
  AppService: { ListWSLDistros: vi.fn(), LogPath: vi.fn() },
  Facade: { Version: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  SettingsService,
  AppService,
  Facade,
}))

import SettingsPanel from '../SettingsPanel.vue'
import ErrorBanner from '../ErrorBanner.vue'
import AutoComplete from 'openvue/autocomplete'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { branchFixture } from '@/testing/fixtures/branch'
import { Theme } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
import { field } from '@/testing/dialog'
import { useRefsStore } from '@/stores/refs'
import { darkThemes } from '@/theme/themes'

const filled = defaultSettingsFixture({ protectedBranches: ['main'] })

async function mountPanel() {
  const wrapper = mount(SettingsPanel)
  await flushPromises()
  return wrapper
}

// withLocalBranches gives the refs store local branches of these names.
function withLocalBranches(...names: string[]): void {
  useRefsStore().refs = {
    branches: names.map((name) => branchFixture({ name })),
    remotes: [],
    tags: [],
  }
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    SettingsService.Get.mockReset().mockResolvedValue(filled)
    SettingsService.Set.mockReset().mockResolvedValue(undefined)
    AppService.ListWSLDistros.mockReset().mockResolvedValue([])
    AppService.LogPath.mockReset().mockResolvedValue({
      path: '/home/user/.local/state/nebegit/nebegit.log',
      error: '',
    })
    Facade.Version.mockReset().mockResolvedValue({ app: 'v0.1.0', protocol: 1 })
  })

  afterEach(() => {
    document.documentElement.classList.remove('nebegit-dark')
  })

  it('FR-UX-04 loads settings and submits the edited form', async () => {
    const wrapper = await mountPanel()

    const interval = wrapper.get('[role=spinbutton]')
    await interval.setValue(5)
    await interval.trigger('blur')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({
      ...filled,
      autoFetchIntervalMinutes: 5,
    })
  })

  it('FR-UX-04 the form appears only once it holds the stored settings, so no early choice is overwritten', async () => {
    let resolve!: (settings: typeof filled) => void
    SettingsService.Get.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const wrapper = mount(SettingsPanel)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)

    resolve(filled)
    await flushPromises()

    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('FR-UX-02 shows the app log path', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain('App log: /home/user/.local/state/nebegit/nebegit.log')
  })

  it('FR-UX-02 shows why the app log could not be opened', async () => {
    AppService.LogPath.mockResolvedValue({ path: '', error: 'permission denied' })
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain('App log could not be opened: permission denied')
  })

  it('FR-UX-04 saving successfully emits close', async () => {
    const wrapper = await mountPanel()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-ED-04, FR-ED-11 renders one independently editable form per environment, submitting the whole updated map', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({
        editorProfiles: {
          '': {
            name: 'Zed',
            folderCmd: 'zed {path}',
            fileCmd: 'zed {path}',
            fileAtLineCmd: 'zed {path}:{line}',
          },
          'Ubuntu-22.04': { name: 'VS Code', folderCmd: 'code {path}', fileCmd: 'code {path}' },
        },
      }),
    )

    const wrapper = await mountPanel()

    const fieldsets = wrapper.findAll('fieldset')
    expect(fieldsets).toHaveLength(2)

    await fieldsets[0]!.get('[data-test="folder-cmd"]').setValue('subl {path}')

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    const sent = SettingsService.Set.mock.calls[0]?.[0]
    expect(sent.editorProfiles['']).toMatchObject({ name: 'Zed', folderCmd: 'subl {path}' })
    expect(sent.editorProfiles['Ubuntu-22.04']).toMatchObject({
      name: 'VS Code',
      folderCmd: 'code {path}',
    })
  })

  it('FR-ED-03 a WSL distro always has its own editor form, defaulted to Zed, with no control to add one', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04'])

    const wrapper = await mountPanel()

    const fieldsets = wrapper.findAll('fieldset')
    expect(fieldsets).toHaveLength(2)
    expect(fieldsets[1]!.text()).toContain('Ubuntu-22.04')
    expect(fieldsets[1]!.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value).toBe(
      'zed {path}',
    )
    expect(wrapper.find('[data-test="add-distro-profile"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Add "Open in editor"')
  })

  it('FR-ED-03 picking a preset in the Ubuntu form and saving updates only that profile', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04'])

    const wrapper = await mountPanel()

    const fieldsets = wrapper.findAll('fieldset')
    await fieldsets[1]!
      .findAll('[data-test="editor-preset"]')
      .find((b) => b.text() === 'VS Code')!
      .trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const sent = SettingsService.Set.mock.calls[0]?.[0]
    expect(sent.editorProfiles['Ubuntu-22.04']).toMatchObject({
      name: 'VS Code',
      folderCmd: 'code {path}',
    })
    expect(sent.editorProfiles['']).toMatchObject({ name: 'Zed', folderCmd: 'zed {path}' })
  })

  it('FR-ED-03 a stored profile for a distro no longer installed still gets a form, kept on save', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({
        editorProfiles: {
          '': { name: 'Zed', folderCmd: 'zed {path}', fileCmd: 'zed {path}' },
          Ubuntu: { name: 'VS Code', folderCmd: 'code {path}', fileCmd: 'code {path}' },
        },
      }),
    )
    AppService.ListWSLDistros.mockResolvedValue(['Debian'])

    const wrapper = await mountPanel()

    expect(wrapper.findAll('legend').map((l) => l.text())).toEqual([
      'Open in editor: Linux',
      'Open in editor: WSL Debian',
      'Open in editor: WSL Ubuntu',
    ])
    expect(wrapper.text()).not.toContain('Add "Open in editor"')
    expect(wrapper.findAll('button').map((b) => b.text())).not.toContain('Test')

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const sent = SettingsService.Set.mock.calls[0]?.[0]
    expect(sent.editorProfiles.Ubuntu).toMatchObject({ name: 'VS Code' })
    expect(sent.editorProfiles.Debian).toMatchObject({ name: 'Zed' })
  })

  it('FR-ED-03 a Linux host with no WSL distros shows a single form naming the local system', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.findAll('fieldset')).toHaveLength(1)
    expect(wrapper.get('legend').text()).toBe('Open in editor: Linux')
  })

  it('FR-REPO-07 on a host without WSL, Settings shows no error and no distro controls', async () => {
    AppService.ListWSLDistros.mockResolvedValue(null)

    const wrapper = await mountPanel()

    expect(wrapper.findComponent(ErrorBanner).exists()).toBe(false)
    expect(wrapper.find('[data-test="add-distro-profile"]').exists()).toBe(false)
  })

  it('FR-UX-01 a rejected save renders ErrorBanner with the error', async () => {
    SettingsService.Set.mockRejectedValueOnce(
      new Error('boom', { cause: { message: 'settings write failed' } }),
    )
    const wrapper = await mountPanel()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'settings write failed' })
  })

  it('FR-UX-06 the stored theme "" (follow the system) shows System selected, and saving keeps ""', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.get('[data-test="theme-select"]').text()).toContain('System')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ theme: Theme.SystemTheme })
  })

  it('FR-UX-06 the theme select lists the seven entries in order, each dark theme with its own swatch colours', async () => {
    const wrapper = mount(SettingsPanel, { attachTo: document.body })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await wrapper.get('[data-test="theme-select"]').trigger('click')
    await flushPromises()

    const options = body.findAll('.p-select-overlay [role="option"]')
    expect(options.map((o) => o.text())).toEqual([
      'System',
      'Light',
      'Dark',
      ...darkThemes.map((t) => t.label),
    ])
    for (const darkTheme of darkThemes) {
      const swatch = body.get(
        `.p-select-overlay [role="option"][aria-label="${darkTheme.label}"] [data-test="theme-swatch"]`,
      )
      const style = swatch.attributes('style') ?? ''
      expect(style).toContain(darkTheme.surface[900])
      expect(style).toContain(darkTheme.accent)
    }
    wrapper.unmount()
  })

  it('FR-UX-06 the credits list the four dark themes by name, copyright and MIT licence, linking to their repo', async () => {
    const wrapper = await mountPanel()

    const credits = wrapper.findAll('[data-test="theme-credit"]')
    expect(credits).toHaveLength(darkThemes.length)
    darkThemes.forEach((darkTheme, i) => {
      expect(credits[i]!.text()).toContain(darkTheme.credit.name)
      expect(credits[i]!.text()).toContain(darkTheme.credit.copyright)
      expect(credits[i]!.text()).toContain('MIT')
      expect(credits[i]!.get('a').attributes('href')).toBe(darkTheme.credit.url)
    })
  })

  it('FR-UX-17 each editor environment is a fieldset of its own; every field has its own label on its own row', async () => {
    const wrapper = await mountPanel()

    const fieldset = wrapper.get('.p-fieldset')
    const labels = fieldset.findAll('label[for]')
    expect(labels.map((l) => l.text())).toEqual([
      'Name (shown as "Open in Zed")',
      'Folder command',
      'File command',
      'File-at-line command',
    ])
    for (const label of labels) {
      expect(fieldset.find(`input[id="${label.attributes('for')}"]`).exists()).toBe(true)
    }
    expect(wrapper.findAll('.settings-panel > .form-field label').map((l) => l.text())).toEqual([
      'Protected branches',
      'Default pull mode',
      'Diff mode',
      'Auto-fetch interval (minutes)',
      'Base folder for new worktrees',
      'Theme',
    ])
  })

  it('FR-UX-06 picking Dark and saving persists it and applies the theme immediately', async () => {
    const wrapper = mount(SettingsPanel, { attachTo: document.body })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await wrapper.get('[data-test="theme-select"]').trigger('click')
    await flushPromises()
    await body.get('.p-select-overlay [role="option"][aria-label="Dark"]').trigger('mousedown')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ theme: Theme.DarkTheme })
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)
    wrapper.unmount()
  })

  it('FR-ED-07 clicking the "VS Code" preset fills fileCmd and fileAtLineCmd', async () => {
    const wrapper = await mountPanel()

    const button = wrapper
      .findAll('[data-test="editor-preset"]')
      .find((b) => b.text() === 'VS Code')
    expect(button).toBeTruthy()
    await button!.trigger('click')

    const fieldset = wrapper.find('fieldset')
    expect(fieldset.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value).toBe(
      'code {path}',
    )
    const inputs = fieldset.findAll<HTMLInputElement>('input[type=text]')
    expect(inputs[2]!.element.value).toBe('code {path}') // File command
    expect(inputs[3]!.element.value).toBe('code -g {path}:{line}') // File-at-line command
  })

  it('FR-ED-07 clicking "Neovim (terminal)" after "VS Code" fully overwrites, not merges', async () => {
    const wrapper = await mountPanel()

    await wrapper
      .findAll('[data-test="editor-preset"]')
      .find((b) => b.text() === 'VS Code')!
      .trigger('click')
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

  it("FR-ED-03 each environment's form names its own profile", async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({
        editorProfiles: {
          '': { name: 'Zed', folderCmd: 'zed {path}', fileCmd: 'zed {path}' },
          Ubuntu: { name: 'VS Code', folderCmd: 'code {path}', fileCmd: 'code {path}' },
        },
      }),
    )

    const wrapper = await mountPanel()

    expect(wrapper.findAll('fieldset').map((f) => f.findAll('label')[0]!.text())).toEqual([
      'Name (shown as "Open in Zed")',
      'Name (shown as "Open in VS Code")',
    ])
  })

  it('FR-DIFF-06 checking "Ignore whitespace" and saving persists it', async () => {
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="ignore-whitespace"] input').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ ignoreWhitespace: true })
  })

  it('FR-UX-04 checking "Debug mode" and saving persists it', async () => {
    const wrapper = await mountPanel()

    await wrapper.get('[data-test="debug-mode"] input').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ debugMode: true })
  })

  it('FR-UX-04 a base folder for new worktrees per environment, local and each WSL distro; saving drops empty ones', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu', 'Debian'])
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ worktreeFolders: { Debian: '/srv/wt' } }),
    )
    const wrapper = await mountPanel()

    const local = field(wrapper, 'Base folder for new worktrees')
    expect(local.attributes('placeholder')).toBe("The repo's parent folder")
    expect(field(wrapper, 'Base folder for new worktrees in WSL: Debian').element.value).toBe(
      '/srv/wt',
    )
    await local.setValue(' /wt ')
    await field(wrapper, 'Base folder for new worktrees in WSL: Ubuntu').setValue('/home/u/wt')
    await field(wrapper, 'Base folder for new worktrees in WSL: Debian').setValue('')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set.mock.calls[0]?.[0].worktreeFolders).toEqual({
      '': '/wt',
      Ubuntu: '/home/u/wt',
    })
  })

  it('DIST-03 renders the app version fetched on mount', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain('v0.1.0')
  })

  it('FR-SYNC-06 the stored protected branches show as chips', async () => {
    const wrapper = await mountPanel()

    const chips = wrapper.findAll('.p-autocomplete-chip-item')
    expect(chips).toHaveLength(1)
    expect(chips[0]!.text()).toContain('main')
  })

  it('FR-SYNC-06 saving sends the chips as protectedBranches', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ protectedBranches: ['main', 'release'] }),
    )
    const wrapper = await mountPanel()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({
      protectedBranches: ['main', 'release'],
    })
  })

  it('FR-SYNC-06 removing a chip drops it from the saved list', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ protectedBranches: ['main', 'master'] }),
    )
    const wrapper = await mountPanel()

    const masterChip = wrapper
      .findAll('.p-autocomplete-chip-item')
      .find((chip) => chip.text().includes('master'))!
    await masterChip.get('.p-chip-remove-icon').trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ protectedBranches: ['main'] })
  })

  it('FR-SYNC-06 suggests branch names matching the query, dropping ones already chosen, then the typed text', async () => {
    const wrapper = await mountPanel()
    withLocalBranches('main', 'master', 'feature/x')

    wrapper.findComponent(AutoComplete).vm.$emit('complete', { query: 'ma' })
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(AutoComplete).props('suggestions')).toEqual(['master', 'ma'])
  })

  it('FR-SYNC-06 the typed text is the only suggestion when no branch matches', async () => {
    const wrapper = await mountPanel()
    withLocalBranches('main', 'master', 'feature/x')

    wrapper.findComponent(AutoComplete).vm.$emit('complete', { query: 'release' })
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(AutoComplete).props('suggestions')).toEqual(['release'])
  })

  it('FR-SYNC-06 a typed name that is already a chip is not suggested again', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ protectedBranches: ['release'] }),
    )
    const wrapper = await mountPanel()

    wrapper.findComponent(AutoComplete).vm.$emit('complete', { query: 'release' })
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(AutoComplete).props('suggestions')).toEqual([])
  })

  it('FR-SYNC-06 outside a repo, only the typed text is suggested', async () => {
    const wrapper = await mountPanel()

    wrapper.findComponent(AutoComplete).vm.$emit('complete', { query: 'dev' })
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(AutoComplete).props('suggestions')).toEqual(['dev'])
  })
})
