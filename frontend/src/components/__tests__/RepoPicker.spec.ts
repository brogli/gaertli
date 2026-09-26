import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '@/testing/mountInBody'

const { AppService, Facade } = vi.hoisted(() => ({
  AppService: {
    PickAndOpen: vi.fn(),
    NewWindow: vi.fn(),
    RecentRepos: vi.fn(),
    ListWSLDistros: vi.fn(),
    SelectRecentRepo: vi.fn(),
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
// openPicker mounts the picker for path and opens its list, or renders it
// as a page; body holds the popover's teleported content.
async function openPicker(props: { path: string; distro?: string; asPage?: boolean }) {
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

describe('RepoPicker', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.PickAndOpen.mockReset().mockResolvedValue(undefined)
    AppService.NewWindow.mockReset().mockResolvedValue(undefined)
    AppService.RecentRepos.mockReset().mockResolvedValue([alpha, beta])
    AppService.ListWSLDistros.mockReset().mockResolvedValue([])
    AppService.SelectRecentRepo.mockReset().mockResolvedValue(undefined)
    Facade.ListWorktrees.mockReset().mockResolvedValue([
      worktreeFixture({ path: '/repos/alpha', branch: 'main', main: true }),
      worktreeFixture({ path: '/repos/alpha-feat', branch: 'feat' }),
    ])
  })

  afterEach(() => {
    window.innerWidth = 1024
  })

  it('FR-WT-13 the button names the repo by its main worktree; the list lists this repo first, checked, then the other recents', async () => {
    const { wrapper, body } = await openPicker({ path: '/repos/alpha-feat' })

    expect(wrapper.get('[data-test="repo-picker"]').text()).toContain('alpha')
    const rows = body.findAll('[data-test="repo-row"]')
    expect(rows.map((r) => r.attributes('title'))).toEqual(['/repos/alpha', beta.path])
    expect(rows[0]!.find('.current-marker i').exists()).toBe(true)
    expect(rows[1]!.find('.current-marker i').exists()).toBe(false)
    expect(rows[1]!.text()).toContain('WSL: Ubuntu')
  })

  it('FR-WT-13 choosing a recent repo hands the whole entry to the core, RepoKey included, for it to switch or fall back', async () => {
    const { body } = await openPicker({ path: '/repos/alpha' })

    await repoRow(body, beta.path).trigger('click')

    expect(AppService.SelectRecentRepo).toHaveBeenCalledWith(beta)
  })

  it('FR-WT-13 a failure the core could not recover from shows in the list', async () => {
    AppService.SelectRecentRepo.mockRejectedValue(
      new Error('boom', { cause: { code: 'NOT_A_REPO', message: 'still gone' } }),
    )
    const { wrapper, body } = await openPicker({ path: '/repos/alpha' })

    await repoRow(body, beta.path).trigger('click')
    await flushPromises()

    expect(wrapper.findComponent(ErrorBanner).text()).toContain('still gone')
  })

  it('FR-WT-13 window with no selection: asPage shows the list in place, with no button', async () => {
    const { wrapper, body } = await openPicker({ path: '', asPage: true })

    expect(wrapper.find('[data-test="repo-picker"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="repo-row"]')).toHaveLength(2)
    expect(actionLabels(body)).toEqual(['Open folder…', 'New window'])
  })

  it('FR-REPO-01 "Open folder…" opens the folder picker', async () => {
    const { body } = await openPicker({ path: '/repos/alpha' })

    await actionRow(body, 'Open folder…').trigger('click')

    expect(AppService.PickAndOpen).toHaveBeenCalledTimes(1)
  })

  it('FR-WT-13 "New window" follows "Open folder…"; choosing it opens a new window', async () => {
    const { body } = await openPicker({ path: '/repos/alpha' })

    expect(actionLabels(body)).toEqual(['Open folder…', 'New window'])

    await actionRow(body, 'New window').trigger('click')

    expect(AppService.NewWindow).toHaveBeenCalledTimes(1)
  })

  it('FR-WT-13 "Open folder…" and "New window" are the only actions, also where WSL has distros', async () => {
    AppService.ListWSLDistros.mockResolvedValue(['Ubuntu-22.04'])
    const { body } = await openPicker({ path: '/repos/alpha' })

    expect(actionLabels(body)).toEqual(['Open folder…', 'New window'])
  })

  it('FR-UX-05 "New window" shows its shortcut after the label; "Open folder…" shows none', async () => {
    const { body } = await openPicker({ path: '/repos/alpha' })

    const newWindow = actionRow(body, 'New window')
    expect(newWindow.find('[data-test="picker-action-label"] + *').text()).toBe('Ctrl+Shift+N')
    expect(
      actionRow(body, 'Open folder…').find('[data-test="picker-action-label"] + *').exists(),
    ).toBe(false)
  })

  it('FR-REPO-07 a recent repo inside WSL names its distro; a local one names none', async () => {
    const { body } = await openPicker({ path: '/repos/other' })

    expect(repoRow(body, beta.path).text()).toContain('WSL: Ubuntu')
    expect(repoRow(body, alpha.path).text()).not.toContain('WSL')
  })

  it('FR-REPO-07 the button of a WSL window marks it as WSL', async () => {
    const { wrapper } = await openPicker({ path: '/home/u/beta', distro: 'Ubuntu' })

    expect(wrapper.get('[data-test="repo-picker"]').text()).toContain('WSL')
  })

  it('FR-UX-09 narrow (400px): the button shows its icon only, the path in title and aria-label', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountInBody(RepoPicker, { props: { path: '/repos/alpha' } })

    const button = wrapper.get('[data-test="repo-picker"]')
    expect(button.text()).toBe('')
    expect(button.attributes('title')).toBe('/repos/alpha')
    expect(button.attributes('aria-label')).toBe('/repos/alpha')
  })
})
