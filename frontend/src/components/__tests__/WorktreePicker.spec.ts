import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '@/testing/mountInBody'
import NoticeArea from '../NoticeArea.vue'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const { AppService, Facade } = vi.hoisted(() => ({
  AppService: { OpenPath: vi.fn(), SelectWorktree: vi.fn() },
  Facade: {
    ListWorktrees: vi.fn(),
    OpenFolderAt: vi.fn(),
    RemoveWorktree: vi.fn(),
    PruneWorktrees: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ AppService, Facade }))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import WorktreePicker from '../WorktreePicker.vue'
import CreateWorktreeDialog from '../CreateWorktreeDialog.vue'
import { worktreeFixture } from '@/testing/fixtures/worktree'
import { clickMenuItem, menuLabels } from '@/testing/contextMenu'

const main = worktreeFixture({ path: '/repos/alpha', branch: 'main', main: true })
const feat = worktreeFixture({ path: '/repos/alpha-feat', branch: 'feat' })

// openPicker mounts the picker on wt/path and opens its list; body holds
// the popover's teleported content.
async function openPicker(props: { wt: string; path: string; distro?: string }) {
  const { wrapper, body } = await mountInBody(WorktreePicker, { props })
  await wrapper.get('[data-test="worktree-picker"]').trigger('click')
  await flushPromises()
  return { wrapper, body }
}

// worktreeRow is the <li> whose row names path; a button lives beside the
// row, both inside this <li>.
function worktreeRow(body: DOMWrapper<Element>, path: string): DOMWrapper<Element> {
  return body.findAll('.picker-rows li').find((li) => {
    const row = li.find('[data-test="worktree-row"]')
    return row.exists() && row.attributes('title') === path
  })!
}

function actionRow(body: DOMWrapper<Element>, label: string): DOMWrapper<Element> {
  return body.findAll('[data-test="picker-action"]').find((r) => r.text() === label)!
}

// worktreeMenu right-clicks path's row, opening its menu.
async function worktreeMenu(body: DOMWrapper<Element>, path: string): Promise<void> {
  await worktreeRow(body, path).get('[data-test="worktree-row"]').trigger('contextmenu')
  await flushPromises()
}

// pickWorktreeMenuAction opens path's menu and clicks the item labelled action.
async function pickWorktreeMenuAction(
  body: DOMWrapper<Element>,
  path: string,
  action: string,
): Promise<void> {
  await worktreeMenu(body, path)
  await clickMenuItem(body, action)
  await flushPromises()
}

// listOpen tells whether the worktree list is showing.
async function listOpen(body: DOMWrapper<Element>): Promise<boolean> {
  await flushPromises()
  await new Promise((resolve) => requestAnimationFrame(resolve))
  await flushPromises()
  return body.find('.worktree-list').exists()
}

describe('WorktreePicker', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.OpenPath.mockReset().mockResolvedValue(undefined)
    AppService.SelectWorktree.mockReset().mockResolvedValue(undefined)
    Facade.ListWorktrees.mockReset().mockResolvedValue([main, feat])
    Facade.OpenFolderAt.mockReset().mockResolvedValue(undefined)
    Facade.RemoveWorktree.mockReset().mockResolvedValue(undefined)
    Facade.PruneWorktrees.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
    Events.On.mockReset()
  })

  afterEach(() => {
    window.innerWidth = 1024
  })

  it("FR-WT-13 the button names the worktree by its folder; the list starts with New worktree…, then this repo's worktrees, the current one checked", async () => {
    const { wrapper, body } = await openPicker({
      wt: '/repos/alpha-feat',
      path: '/repos/alpha-feat',
    })

    expect(wrapper.get('[data-test="worktree-picker"]').text()).toContain('alpha-feat')
    expect(actionRow(body, 'New worktree…')).toBeTruthy()
    expect(worktreeRow(body, '/repos/alpha').find('.current-marker i').exists()).toBe(false)
    expect(worktreeRow(body, '/repos/alpha-feat').find('.current-marker i').exists()).toBe(true)
  })

  it('FR-WT-13 a row shows its folder name first and its branch after it, smaller', async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    const parts = worktreeRow(body, '/repos/alpha-feat').findAll(
      '[data-test="worktree-folder"], [data-test="worktree-branch"]',
    )
    expect(parts.map((p) => p.text())).toEqual(['alpha-feat', 'feat'])
    expect(parts[0]!.classes()).not.toContain('muted')
    expect(parts[1]!.classes()).toContain('muted')
  })

  it("FR-WT-13 typing part of a worktree's folder name leaves only its row", async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await body.get('.picker-list input').setValue('feat')
    await flushPromises()

    expect(body.findAll('.picker-rows li')).toHaveLength(1)
    expect(worktreeRow(body, '/repos/alpha-feat')).toBeTruthy()
  })

  it('FR-WT-08 choosing a worktree switches this window; its "open in new window" button calls OpenPath', async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await worktreeRow(body, '/repos/alpha-feat').get('[data-test="worktree-row"]').trigger('click')
    expect(AppService.SelectWorktree).toHaveBeenCalledWith('/repos/alpha-feat')

    await worktreeRow(body, '/repos/alpha-feat')
      .get('[data-test="open-in-new-window"]')
      .trigger('click')
    expect(AppService.OpenPath).toHaveBeenCalledWith('/repos/alpha-feat')
  })

  it('FR-WT-13 a right-click opens a worktree\'s menu: the current one, another, one missing and a bare one differ; no "…" buttons show', async () => {
    const other = worktreeFixture({ path: '/repos/alpha-other', branch: 'other' })
    const gone = worktreeFixture({ path: '/repos/alpha-gone', branch: 'gone', missing: true })
    const bare = worktreeFixture({ path: '/repos/alpha.git', branch: '', sha: '', bare: true })
    Facade.ListWorktrees.mockResolvedValue([main, other, gone, bare])
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    expect(body.find('[data-test="row-actions"]').exists()).toBe(false)

    // A bare row is a header: checked first, with no earlier menu open to
    // linger, since PickerList leaves an open menu as is for an empty one.
    await worktreeMenu(body, '/repos/alpha.git')
    expect(menuLabels(body)).toEqual([])

    await worktreeMenu(body, '/repos/alpha')
    expect(menuLabels(body)).toEqual(['Open in new window', 'Open in editor'])

    await worktreeMenu(body, '/repos/alpha-other')
    expect(menuLabels(body)).toEqual([
      'Switch to worktree',
      'Open in new window',
      'Open in editor',
      'Remove worktree',
    ])

    await worktreeMenu(body, '/repos/alpha-gone')
    expect(menuLabels(body)).toEqual([
      'Switch to worktree',
      'Open in new window',
      'Remove worktree',
    ])
  })

  it("FR-WT-13 Shift+F10 in the filter opens the active worktree's menu", async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })
    const filter = body.get('.picker-list input')

    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'F10', shiftKey: true })

    expect(menuLabels(body)).toEqual([
      'Switch to worktree',
      'Open in new window',
      'Open in editor',
      'Remove worktree',
    ])
  })

  it('FR-WT-08 the menu\'s "Switch to worktree", "Open in new window" and "Open in editor" call SelectWorktree, OpenPath and OpenFolderAt with the worktree\'s id', async () => {
    const { wrapper, body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await pickWorktreeMenuAction(body, '/repos/alpha-feat', 'Switch to worktree')
    expect(AppService.SelectWorktree).toHaveBeenCalledWith('/repos/alpha-feat')

    await wrapper.get('[data-test="worktree-picker"]').trigger('click')
    await pickWorktreeMenuAction(body, '/repos/alpha-feat', 'Open in new window')
    expect(AppService.OpenPath).toHaveBeenCalledWith('/repos/alpha-feat')

    await wrapper.get('[data-test="worktree-picker"]').trigger('click')
    await pickWorktreeMenuAction(body, '/repos/alpha-feat', 'Open in editor')
    expect(Facade.OpenFolderAt).toHaveBeenCalledWith('/repos/alpha-feat')
  })

  it("FR-WT-07 the menu's Remove worktree confirms, naming the loss, before removing a clean worktree", async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await pickWorktreeMenuAction(body, '/repos/alpha-feat', 'Remove worktree')
    expect(confirmOptions().message).not.toContain('uncommitted changes')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.RemoveWorktree).toHaveBeenCalledWith('/repos/alpha', '/repos/alpha-feat', false)
  })

  it("FR-WT-13 the menu's Open in new window failing shows the reason as a notice, not the list's banner", async () => {
    AppService.OpenPath.mockRejectedValueOnce(new Error('boom', { cause: { message: 'boom' } }))
    const { body: notice } = await mountInBody(NoticeArea)
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await pickWorktreeMenuAction(body, '/repos/alpha-feat', 'Open in new window')

    expect(notice.get('.p-toast').text()).toContain('boom')
    expect(body.find('.error-banner').exists()).toBe(false)
  })

  it('FR-WT-13 the Remove button closes the list before confirming; a refusal other than DIRTY_WORKTREE shows as a notice, no second confirmation', async () => {
    Facade.RemoveWorktree.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'GIT_FAILED', message: 'is locked' } }),
    )
    const { body: notice } = await mountInBody(NoticeArea)
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await worktreeRow(body, '/repos/alpha-feat')
      .get('[data-test="remove-worktree"]')
      .trigger('click')
    expect(await listOpen(body)).toBe(false)

    confirmOptions().accept()
    await flushPromises()

    expect(notice.get('.p-toast').text()).toContain('is locked')
    expect(confirmRequire).toHaveBeenCalledTimes(1)
  })

  it("FR-WT-08 a WSL repo's worktree is switched to and opened by its UNC path", async () => {
    const wip = worktreeFixture({ path: '/home/u/beta-wip', branch: 'wip' })
    Facade.ListWorktrees.mockResolvedValue([main, wip])
    const { body } = await openPicker({
      wt: '\\\\wsl.localhost\\Ubuntu\\home\\u\\beta',
      path: '/home/u/beta',
      distro: 'Ubuntu',
    })

    await worktreeRow(body, '/home/u/beta-wip').get('[data-test="worktree-row"]').trigger('click')
    await worktreeRow(body, '/home/u/beta-wip').get('[data-test="open-in-editor"]').trigger('click')

    const wipUNC = '\\\\wsl.localhost\\Ubuntu\\home\\u\\beta-wip'
    expect(AppService.SelectWorktree).toHaveBeenCalledWith(wipUNC)
    expect(Facade.OpenFolderAt).toHaveBeenCalledWith(wipUNC)
  })

  it('FR-ED-06 open in editor failing EDITOR_NOT_FOUND shows an error notice naming the command', async () => {
    const wip = worktreeFixture({ path: '/repos/alpha-wip', branch: 'wip' })
    Facade.ListWorktrees.mockResolvedValue([main, wip])
    Facade.OpenFolderAt.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'EDITOR_NOT_FOUND', message: '"myed" is not on PATH' } }),
    )
    const { body: notice } = await mountInBody(NoticeArea)
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await worktreeRow(body, '/repos/alpha-wip').get('[data-test="open-in-editor"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
  })

  it('FR-WT-01 a worktree row shows its branch, folder, and flags main, dirty, locked and missing', async () => {
    Facade.ListWorktrees.mockResolvedValue([
      main,
      worktreeFixture({ path: '/repos/wt-dirty', branch: 'd', dirty: true }),
      worktreeFixture({ path: '/repos/wt-locked', branch: 'l', locked: true }),
      worktreeFixture({ path: '/repos/wt-gone', branch: 'g', missing: true }),
    ])
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    expect(worktreeRow(body, '/repos/alpha').text()).toContain('main')
    expect(worktreeRow(body, '/repos/wt-dirty').find('[data-test="dirty-marker"]').exists()).toBe(
      true,
    )
    expect(worktreeRow(body, '/repos/wt-locked').find('[data-test="locked-marker"]').exists()).toBe(
      true,
    )
    expect(worktreeRow(body, '/repos/wt-gone').text()).toContain('missing')
    expect(worktreeRow(body, '/repos/wt-gone').find('[data-test="open-in-editor"]').exists()).toBe(
      false,
    )
  })

  it('FR-WT-01 a bare repo is listed by its folder and marked bare, with no actions, and cannot be chosen', async () => {
    Facade.ListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/repos/alpha.git', branch: '', sha: '', main: true, bare: true }),
      feat,
    ])
    const { body } = await openPicker({ wt: '/repos/alpha-feat', path: '/repos/alpha-feat' })
    const bare = worktreeRow(body, '/repos/alpha.git')

    expect(bare.text()).toContain('alpha.git')
    expect(bare.text()).toContain('bare')
    expect(bare.text()).not.toContain('main')
    expect(bare.text()).not.toContain('detached')
    expect(bare.findAll('button')).toHaveLength(0)
    await bare.get('[data-test="worktree-row"]').trigger('click')
    expect(AppService.SelectWorktree).not.toHaveBeenCalled()
  })

  it('FR-WT-07 the main and the current worktree have no remove button; a linked one confirms, naming the loss, before removing', async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    expect(worktreeRow(body, '/repos/alpha').find('[data-test="remove-worktree"]').exists()).toBe(
      false,
    )
    await worktreeRow(body, '/repos/alpha-feat')
      .get('[data-test="remove-worktree"]')
      .trigger('click')
    expect(confirmOptions().message).not.toContain('uncommitted changes')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.RemoveWorktree).toHaveBeenCalledWith('/repos/alpha', '/repos/alpha-feat', false)
  })

  it('FR-WT-07 with a linked worktree selected, neither it nor the main worktree offers Remove; another linked one does', async () => {
    const third = worktreeFixture({ path: '/repos/alpha-third', branch: 'third' })
    Facade.ListWorktrees.mockResolvedValue([main, feat, third])
    const { body } = await openPicker({ wt: '/repos/alpha-feat', path: '/repos/alpha-feat' })

    expect(worktreeRow(body, '/repos/alpha').find('[data-test="remove-worktree"]').exists()).toBe(
      false,
    )
    expect(
      worktreeRow(body, '/repos/alpha-feat').find('[data-test="remove-worktree"]').exists(),
    ).toBe(false)
    expect(
      worktreeRow(body, '/repos/alpha-third').find('[data-test="remove-worktree"]').exists(),
    ).toBe(true)
  })

  it('FR-WT-07 a dirty worktree names the loss and removes forced', async () => {
    const dirty = worktreeFixture({ path: '/repos/wt-dirty', dirty: true })
    Facade.ListWorktrees.mockResolvedValue([main, dirty])
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await worktreeRow(body, '/repos/wt-dirty').get('[data-test="remove-worktree"]').trigger('click')
    expect(confirmOptions().message).toContain('uncommitted changes that will be lost')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.RemoveWorktree).toHaveBeenCalledWith('/repos/alpha', '/repos/wt-dirty', true)
  })

  it('FR-WT-07 "Clean up stale entries" shows only when a worktree is missing, prunes without asking', async () => {
    Facade.ListWorktrees.mockResolvedValue([
      main,
      worktreeFixture({ path: '/repos/gone', missing: true }),
    ])
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await actionRow(body, 'Clean up stale entries').trigger('click')
    await flushPromises()

    expect(Facade.PruneWorktrees).toHaveBeenCalledWith('/repos/alpha')
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it('FR-WT-02 "New worktree…" opens the dialog on this repo', async () => {
    const { wrapper, body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })

    await actionRow(body, 'New worktree…').trigger('click')
    await flushPromises()

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.props('repoPath')).toBe('/repos/alpha')
  })

  it('FR-WT-10 a repo:changed{scope:"worktrees"} event re-lists the worktrees', async () => {
    const { body } = await openPicker({ wt: '/repos/alpha', path: '/repos/alpha' })
    Facade.ListWorktrees.mockClear()
    const added = worktreeFixture({ path: '/repos/alpha-added', branch: 'added' })
    Facade.ListWorktrees.mockResolvedValueOnce([main, feat, added])

    emit('repo:changed', { data: { scope: ChangeScope.WorktreesChanged } })
    await flushPromises()

    expect(Facade.ListWorktrees).toHaveBeenCalledWith('/repos/alpha')
    expect(worktreeRow(body, '/repos/alpha-added')).toBeTruthy()
  })

  it("FR-WT-10 opening the picker re-lists this repo's worktrees and clears the filter", async () => {
    const { wrapper, body } = await mountInBody(WorktreePicker, {
      props: { wt: '/repos/alpha', path: '/repos/alpha' },
      global: { stubs: { transition: false } },
    })
    const button = wrapper.get('[data-test="worktree-picker"]')
    await button.trigger('click')
    await flushPromises()
    await body.get('.picker-list input').setValue('feat')
    Facade.ListWorktrees.mockClear()

    await button.trigger('click')
    await new Promise((resolve) => requestAnimationFrame(resolve))
    await button.trigger('click')
    await flushPromises()

    expect(Facade.ListWorktrees).toHaveBeenCalledWith('/repos/alpha')
    expect(body.get<HTMLInputElement>('.picker-list input').element.value).toBe('')
  })

  it('FR-UX-09 narrow (400px): the button shows its icon only, the path in title and aria-label', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountInBody(WorktreePicker, {
      props: { wt: '/repos/alpha', path: '/repos/alpha' },
    })

    const button = wrapper.get('[data-test="worktree-picker"]')
    expect(button.text()).toBe('')
    expect(button.attributes('title')).toBe('/repos/alpha')
    expect(button.attributes('aria-label')).toBe('/repos/alpha')
  })
})
