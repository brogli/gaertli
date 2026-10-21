// Requirements: DIST-03, FR-DIFF-06, FR-ED-04, FR-ED-07, FR-ED-11, FR-ED-13, FR-REPO-07, FR-SYNC-06, FR-UX-01, FR-UX-02, FR-UX-04, FR-UX-06, FR-UX-17
import { flushPromises, mount, type BaseWrapper, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { wailsError } from '@/testing/errors'
import { AppService, Facade, SettingsService, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import SettingsPanel from '../SettingsPanel.vue'
import ErrorBanner from '../ErrorBanner.vue'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { Theme } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
import { field } from '@/testing/dialog'
import { mountInBody } from '@/testing/mountInBody'
import { chooseOption } from '@/testing/select'
import { System } from '@/testing/runtime'
import { darkThemes } from '@/theme/themes'

const filled = defaultSettingsFixture({ protectedBranches: ['main'] })

async function mountPanel() {
  const wrapper = mount(SettingsPanel)
  await flushPromises()
  return wrapper
}

async function submit(wrapper: VueWrapper): Promise<void> {
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

// pickPreset clicks the editor preset button named name inside root.
async function pickPreset(root: Pick<BaseWrapper<Node>, 'findAll'>, name: string): Promise<void> {
  await root
    .findAll('[data-test="editor-preset"]')
    .find((b) => b.text() === name)!
    .trigger('click')
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
    SettingsService.Get.mockResolvedValue(filled)
    System.IsWindows.mockReset().mockReturnValue(false)
    AppService.LogPath.mockResolvedValue({
      path: '/home/user/.local/state/nebegit/nebegit.log',
      error: '',
    })
    Facade.Version.mockResolvedValue({ app: 'v0.1.0', commit: '', protocol: 1 })
  })

  afterEach(() => {
    document.documentElement.classList.remove('nebegit-dark')
  })

  it('loads the stored settings and submits every edited field, keeping the system theme', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ protectedBranches: ['main', 'master'] }),
    )
    const wrapper = await mountPanel()
    expect(wrapper.get('[data-test="theme-select"]').text()).toContain('System')

    const interval = wrapper.get('[role=spinbutton]')
    await interval.setValue(5)
    await interval.trigger('blur')
    await wrapper.get('[data-test="ignore-whitespace"] input').setValue(true)
    await wrapper.get('[data-test="debug-mode"] input').setValue(true)
    const masterChip = wrapper
      .findAll('.p-autocomplete-chip-item')
      .find((chip) => chip.text().includes('master'))!
    await masterChip.get('.p-chip-remove-icon').trigger('click')
    await submit(wrapper)

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({
      ...filled,
      autoFetchIntervalMinutes: 5,
      ignoreWhitespace: true,
      debugMode: true,
      protectedBranches: ['main'],
      theme: Theme.SystemTheme,
    })
  })

  it('the form appears only once it holds the stored settings, so no early choice is overwritten', async () => {
    let resolve!: (settings: typeof filled) => void
    SettingsService.Get.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const wrapper = mount(SettingsPanel)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)

    resolve(filled)
    await flushPromises()

    expect(wrapper.find('form').exists()).toBe(true)
  })

  it.each([
    [
      'the app log path',
      { path: '/home/user/.local/state/nebegit/nebegit.log', error: '' },
      'App log: /home/user/.local/state/nebegit/nebegit.log',
    ],
    [
      'why the app log could not be opened',
      { path: '', error: 'permission denied' },
      'App log could not be opened: permission denied',
    ],
  ])('shows %s', async (_name, logPath, text) => {
    AppService.LogPath.mockResolvedValue(logPath)
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain(text)
  })

  it('saving successfully emits close', async () => {
    const wrapper = await mountPanel()

    await submit(wrapper)

    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('renders the stored editor profile in one form, submitting the edited profile', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({
        editorProfile: {
          name: 'Zed',
          folderCmd: 'zed {path}',
          fileCmd: 'zed {path}',
          fileAtLineCmd: 'zed {path}:{line}',
        },
      }),
    )

    const wrapper = await mountPanel()

    await wrapper.get('[data-test="folder-cmd"]').setValue('subl {path}')
    await submit(wrapper)

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0].editorProfile).toMatchObject({
      name: 'Zed',
      folderCmd: 'subl {path}',
    })
  })

  it('with WSL distros installed Settings still shows one editor form', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04', 'Debian'])

    const wrapper = await mountPanel()

    expect(wrapper.findAll('fieldset')).toHaveLength(1)
    expect(wrapper.findAll('[data-test="folder-cmd"]')).toHaveLength(1)
    expect(wrapper.get('legend').text()).toBe('Open in editor')
  })

  it('picking a preset with WSL distros installed saves it as the one profile', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04', 'Debian'])

    const wrapper = await mountPanel()

    await pickPreset(wrapper, 'VS Code')
    await submit(wrapper)

    const sent = SettingsService.Set.mock.calls[0]?.[0]
    expect(sent.editorProfile).toMatchObject({ name: 'VS Code', folderCmd: 'code {path}' })
    expect(sent.editorProfiles).toBeUndefined()
  })

  it('on a Windows host the Zed preset opens files in a window of their own', async () => {
    System.IsWindows.mockReturnValue(true)
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({
        editorProfile: { name: 'VS Code', folderCmd: 'code {path}', fileCmd: 'code {path}' },
      }),
    )

    const wrapper = await mountPanel()

    await pickPreset(wrapper, 'Zed')
    await submit(wrapper)

    expect(SettingsService.Set.mock.calls[0]?.[0].editorProfile).toMatchObject({
      folderCmd: 'zed {path}',
      fileCmd: 'zed -n {path}',
      fileAtLineCmd: 'zed -n {path}:{line}',
    })
  })

  it('on a Linux host the Zed preset opens files in the last used window', async () => {
    const wrapper = await mountPanel()

    await pickPreset(wrapper, 'Zed')
    await submit(wrapper)

    expect(SettingsService.Set.mock.calls[0]?.[0].editorProfile).toMatchObject({
      fileCmd: 'zed {path}',
      fileAtLineCmd: 'zed {path}:{line}',
    })
  })

  it('on a host without WSL, Settings shows no error and no distro controls', async () => {
    AppService.ListWSLDistros.mockResolvedValue(null)

    const wrapper = await mountPanel()

    expect(wrapper.findComponent(ErrorBanner).exists()).toBe(false)
    expect(wrapper.findAll('fieldset')).toHaveLength(1)
    expect(wrapper.get('legend').text()).toBe('Open in editor')
  })

  it('a rejected save renders ErrorBanner with the error', async () => {
    SettingsService.Set.mockRejectedValueOnce(wailsError('settings write failed'))
    const wrapper = await mountPanel()

    await submit(wrapper)

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'settings write failed' })
  })

  it('the theme select lists the seven entries in order', async () => {
    const { wrapper, body } = await mountInBody(SettingsPanel)

    await wrapper.get('[data-test="theme-select"]').trigger('click')
    await flushPromises()

    const options = body.findAll('.p-select-overlay [role="option"]')
    expect(options.map((o) => o.text())).toEqual([
      'System',
      'Light',
      'Dark',
      ...darkThemes.map((t) => t.label),
    ])
  })

  it('the credits list the four dark themes by name, copyright and MIT licence, linking to their repo', async () => {
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

  it('the editor is a fieldset of its own; every field has its own label on its own row', async () => {
    const wrapper = await mountPanel()

    const fieldset = wrapper.get('.p-fieldset')
    const labels = fieldset.findAll('label[for]')
    expect(labels.map((l) => l.text())).toEqual([
      'Name (shown as "Open in editor")',
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

  it('picking Dark and saving persists it and applies the theme immediately', async () => {
    const { wrapper, body } = await mountInBody(SettingsPanel)

    await chooseOption(body, 'theme-select', 'Dark')
    await submit(wrapper)

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({ theme: Theme.DarkTheme })
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)
  })

  it('a preset fills all three commands, and the next preset fully overwrites rather than merges', async () => {
    const wrapper = await mountPanel()
    const fieldset = wrapper.get('fieldset')
    const fields = () => ({
      folder: fieldset.get<HTMLInputElement>('[data-test="folder-cmd"]').element.value,
      file: fieldset.findAll<HTMLInputElement>('input[type=text]')[2]!.element.value,
      line: fieldset.findAll<HTMLInputElement>('input[type=text]')[3]!.element.value,
    })

    await pickPreset(fieldset, 'VS Code')
    expect(fields()).toEqual({
      folder: 'code {path}',
      file: 'code {path}',
      line: 'code -g {path}:{line}',
    })

    await pickPreset(fieldset, 'Neovim (terminal)')
    expect(fields()).toEqual({
      folder: 'x-terminal-emulator -e nvim {path}',
      file: 'x-terminal-emulator -e nvim {path}',
      line: 'x-terminal-emulator -e nvim +{line} {path}',
    })
  })

  it('a base folder for new worktrees per environment, local and each WSL distro; saving drops empty ones', async () => {
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
    await submit(wrapper)

    expect(SettingsService.Set.mock.calls[0]?.[0].worktreeFolders).toEqual({
      '': '/wt',
      Ubuntu: '/home/u/wt',
    })
  })

  it.each([
    [{ app: '0.1.57', commit: 'abc1234', protocol: 1 }, 'nebegit 0.1.57 (abc1234)'],
    [{ app: 'dev', commit: '', protocol: 1 }, 'nebegit dev'],
  ])(
    'shows the app version fetched on mount, with the commit when there is one',
    async (version, text) => {
      Facade.Version.mockResolvedValue(version)
      const wrapper = await mountPanel()

      expect(wrapper.find('[data-test="app-version"]').text()).toBe(text)
    },
  )

  it('the stored protected branches show as chips', async () => {
    const wrapper = await mountPanel()

    const chips = wrapper.findAll('.p-autocomplete-chip-item')
    expect(chips).toHaveLength(1)
    expect(chips[0]!.text()).toContain('main')
  })
})
