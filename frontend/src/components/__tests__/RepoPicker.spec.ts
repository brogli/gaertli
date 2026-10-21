// Requirements: FR-REPO-01, FR-REPO-07, FR-REPO-08, FR-UX-05, FR-UX-09, FR-WT-13, FR-WT-16
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { expectIconOnly } from '@/testing/expectIconOnly'
import { mountNotices } from '@/testing/mountNotices'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { popoverShows } from '@/testing/popover'
import { clickMenuItem, menuLabels } from '@/testing/contextMenu'

const { AppService, Facade } = vi.hoisted(() => ({
  AppService: {
    PickAndOpen: vi.fn(),
    NewWindow: vi.fn(),
    RecentRepos: vi.fn(),
    ListWSLDistros: vi.fn(),
    SelectRecentRepo: vi.fn(),
    OpenRecentRepo: vi.fn(),
    RemoveRecentRepo: vi.fn(),
  },
  Facade: { ListWorktrees: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ AppService, Facade }))

import RepoPicker from '../RepoPicker.vue'
import ErrorBanner from '../ErrorBanner.vue'
import { worktreeFixture } from '@/testing/fixtures/worktree'

const alpha = {
  path: '/repos/alpha',
  lastOpened: '2026-01-02T00:00:00Z',
  repoKey: '/repos/alpha/.git',
}
const beta = {
  path: '/home/u/beta',
  lastOpened: '2026-01-01T00:00:00Z',
  distro: 'Ubuntu',
  repoKey: '/home/u/beta/.git',
}
// openPicker mounts the picker for path, /repos/alpha unless props say
// otherwise, and opens its list, or renders it as a page; body holds the
// popover's teleported content.
async function openPicker(
  props: { path: string; distro?: string; asPage?: boolean } = { path: '/repos/alpha' },
) {
  const { wrapper, body } = await mountInBody(RepoPicker, { props })
  if (!props.asPage) {
    await wrapper.get('[data-test="repo-picker"]').trigger('click')
    await flushPromises()
  }
  return { wrapper, body }
}

function repoRow(body: DOMWrapper<Element>, path: string): DOMWrapper<Element> {
  return body.findAll('[data-test="repo-row"]').find((r) => r.attributes('title') === path)!
}

function actionRow(body: DOMWrapper<Element>, label: string): DOMWrapper<Element> {
  return body
    .findAll('[data-test="picker-action"]')
    .find((r) => r.get('[data-test="picker-action-label"]').text() === label)!
}

function actionLabels(body: DOMWrapper<Element>): string[] {
  return body.findAll('[data-test="picker-action-label"]').map((r) => r.text())
}

// rowLiFor is el's row, for checking the "…" button beside it.
function rowLiFor(el: DOMWrapper<Element>): DOMWrapper<Element> {
  return new DOMWrapper(el.element.closest('li'))
}

// repoMenu right-clicks path's row, opening its menu.
async function repoMenu(body: DOMWrapper<Element>, path: string): Promise<void> {
  await repoRow(body, path).trigger('contextmenu')
  await flushPromises()
}

// pickRepoMenuAction opens path's menu and clicks the item labelled action.
async function pickRepoMenuAction(
  body: DOMWrapper<Element>,
  path: string,
  action: string,
): Promise<void> {
  await repoMenu(body, path)
  await clickMenuItem(body, action)
  await flushPromises()
}

// listOpen tells whether the repo list is showing.
function listOpen(body: DOMWrapper<Element>): Promise<boolean> {
  return popoverShows(body, '.repo-list')
}

describe('RepoPicker', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.PickAndOpen.mockReset().mockResolvedValue(undefined)
    AppService.NewWindow.mockReset().mockResolvedValue(undefined)
    AppService.RecentRepos.mockReset().mockResolvedValue([alpha, beta])
    AppService.ListWSLDistros.mockReset().mockResolvedValue([])
    AppService.SelectRecentRepo.mockReset().mockResolvedValue(undefined)
    AppService.OpenRecentRepo.mockReset().mockResolvedValue(undefined)
    AppService.RemoveRecentRepo.mockReset().mockResolvedValue(undefined)
    Facade.ListWorktrees.mockReset().mockResolvedValue([
      worktreeFixture({ path: '/repos/alpha', branch: 'main', main: true }),
      worktreeFixture({ path: '/repos/alpha-feat', branch: 'feat' }),
    ])
  })

  afterEach(() => {
    window.innerWidth = 1024
  })

  it('the button names the repo by its main worktree; the list lists this repo first, checked, then the other recents', async () => {
    const { wrapper, body } = await openPicker({ path: '/repos/alpha-feat' })

    expect(wrapper.get('[data-test="repo-picker"]').text()).toContain('alpha')
    const rows = body.findAll('[data-test="repo-row"]')
    expect(rows.map((r) => r.attributes('title'))).toEqual(['/repos/alpha', beta.path])
    expect(rows[0]!.find('.current-marker i').exists()).toBe(true)
    expect(rows[1]!.find('.current-marker i').exists()).toBe(false)
    expect(rows[1]!.text()).toContain('WSL: Ubuntu')
  })

  it('choosing a recent repo hands the whole entry to the core, RepoKey included, for it to switch or fall back', async () => {
    const { body } = await openPicker()

    await repoRow(body, beta.path).trigger('click')

    expect(AppService.SelectRecentRepo).toHaveBeenCalledWith(beta)
  })

  it('a failure the core could not recover from shows in the list', async () => {
    AppService.SelectRecentRepo.mockRejectedValue(wailsError('still gone', 'NOT_A_REPO'))
    const { wrapper, body } = await openPicker()

    await repoRow(body, beta.path).trigger('click')
    await flushPromises()

    expect(wrapper.findComponent(ErrorBanner).text()).toContain('still gone')
  })

  it('window with no selection: asPage shows the list in place, with no button', async () => {
    const { wrapper, body } = await openPicker({ path: '', asPage: true })

    expect(wrapper.find('[data-test="repo-picker"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="repo-row"]')).toHaveLength(2)
    expect(actionLabels(body)).toEqual(['Open folder…', 'New window'])
  })

  it('"Open folder…" opens the folder picker', async () => {
    const { body } = await openPicker()

    await actionRow(body, 'Open folder…').trigger('click')

    expect(AppService.PickAndOpen).toHaveBeenCalledTimes(1)
  })

  it('"New window" follows "Open folder…", the only actions also where WSL has distros; choosing it opens a new window', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04'])
    const { body } = await openPicker()

    expect(actionLabels(body)).toEqual(['Open folder…', 'New window'])

    await actionRow(body, 'New window').trigger('click')

    expect(AppService.NewWindow).toHaveBeenCalledTimes(1)
  })

  it('"New window" shows shortcuts.newWindow through ShortcutHint; "Open folder…" shows none', async () => {
    const { body } = await openPicker()

    expect(actionRow(body, 'New window').find('[data-test="shortcut-hint"]').text()).toBe(
      'Ctrl+Shift+N',
    )
    expect(actionRow(body, 'Open folder…').find('[data-test="shortcut-hint"]').exists()).toBe(false)
  })

  it('a recent repo inside WSL names its distro, a local one none; the button of a WSL window marks it as WSL', async () => {
    const { body } = await openPicker({ path: '/repos/other' })
    const wsl = await mountInBody(RepoPicker, { props: { path: '/repos/other', distro: 'Ubuntu' } })

    expect(repoRow(body, beta.path).text()).toContain('WSL: Ubuntu')
    expect(repoRow(body, alpha.path).text()).not.toContain('WSL')
    expect(wsl.wrapper.get('[data-test="repo-picker"]').text()).toContain('WSL')
  })

  it('narrow (400px): the button shows its icon only, the path in title and aria-label', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountInBody(RepoPicker, { props: { path: '/repos/alpha' } })

    const button = wrapper.get('[data-test="repo-picker"]')
    expectIconOnly(button, '/repos/alpha')
  })

  it('a recent repo other than the window\'s own gets a "…" button and a right-click menu with "Open in new window" and "Remove from recents"; the current row and the actions get neither', async () => {
    const { body } = await openPicker()

    expect(rowLiFor(repoRow(body, alpha.path)).find('[data-test="row-actions"]').exists()).toBe(
      false,
    )
    expect(
      rowLiFor(actionRow(body, 'Open folder…')).find('[data-test="row-actions"]').exists(),
    ).toBe(false)
    expect(rowLiFor(repoRow(body, beta.path)).find('[data-test="row-actions"]').exists()).toBe(true)

    await repoMenu(body, alpha.path)
    expect(menuLabels(body)).toEqual([])

    await repoMenu(body, beta.path)
    expect(menuLabels(body)).toEqual(['Open in new window', 'Remove from recents'])
  })

  it('"…" then "Open in new window" closes the list and calls OpenRecentRepo with the whole entry', async () => {
    const { body } = await openPicker()

    await rowLiFor(repoRow(body, beta.path)).get('[data-test="row-actions"]').trigger('click')
    await clickMenuItem(body, 'Open in new window')

    expect(AppService.OpenRecentRepo).toHaveBeenCalledWith(beta)
    expect(await listOpen(body)).toBe(false)
  })

  it("Open in new window failing shows the refusal as a notice, not the list's banner", async () => {
    AppService.OpenRecentRepo.mockRejectedValueOnce(wailsError('boom'))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickRepoMenuAction(body, beta.path, 'Open in new window')

    expect(notice.get('.p-toast').text()).toContain('boom')
    expect(body.find('.error-banner').exists()).toBe(false)
  })

  it('page form: Remove from recents removes the row and reloads the recents, the list stays', async () => {
    AppService.RecentRepos.mockReset()
      .mockResolvedValueOnce([alpha, beta])
      .mockResolvedValueOnce([alpha])
    const { wrapper, body } = await openPicker({ path: '', asPage: true })

    await pickRepoMenuAction(body, beta.path, 'Remove from recents')

    expect(AppService.RemoveRecentRepo).toHaveBeenCalledWith(beta)
    expect(body.findAll('[data-test="repo-row"]').map((r) => r.attributes('title'))).toEqual([
      alpha.path,
    ])
    expect(wrapper.find('.repo-list').exists()).toBe(true)
  })

  it('Remove from recents failing shows the refusal as a notice', async () => {
    AppService.RemoveRecentRepo.mockRejectedValueOnce(wailsError('boom'))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickRepoMenuAction(body, beta.path, 'Remove from recents')

    expect(notice.get('.p-toast').text()).toContain('boom')
  })
})
