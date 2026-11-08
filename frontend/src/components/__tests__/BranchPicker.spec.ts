// Requirements: FR-BR-01, FR-BR-05, FR-BR-06, FR-BR-08, FR-BR-09, FR-BR-11, FR-BR-13, FR-CONF-09, FR-LOG-01, FR-LOG-06, FR-REPO-03, FR-UPD-01, FR-UX-01, FR-UX-03, FR-UX-09, FR-UX-14, FR-WT-02, FR-WT-15
import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { expectIconOnly } from '@/testing/expectIconOnly'
import { mountNotices } from '@/testing/mountNotices'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { popoverShows } from '@/testing/popover'

const { AppService, Facade } = vi.hoisted(() => ({
  AppService: { SelectWorktree: vi.fn(), OpenDiff: vi.fn() },
  Facade: {
    Checkout: vi.fn(),
    UpdateBranch: vi.fn(),
    RenameBranch: vi.fn(),
    DeleteBranch: vi.fn(),
    Merge: vi.fn(),
    Rebase: vi.fn(),
    Status: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ AppService, Facade }))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import BranchPicker from '../BranchPicker.vue'
import RenameBranchDialog from '../RenameBranchDialog.vue'
import { useBranchesStore } from '@/stores/branches'
import { useWorktreesStore } from '@/stores/worktrees'
import { useLogStore } from '@/stores/log'
import {
  BranchInUse,
  Code,
  OperationKind,
  type Refs,
  type Summary,
} from '../../../bindings/github.com/brogli/gaertli/internal/core'
import { DiffKind } from '@/lib/diffTarget'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { branchFixture } from '@/testing/fixtures/branch'
import { refsOfEveryKind } from '@/testing/fixtures/refs'
import { summaryFixture } from '@/testing/fixtures/summary'
import { clickMenuItem, menuLabels } from '@/testing/contextMenu'
import { findButton } from '@/testing/findButton'

const main = branchFixture()
const feat = branchFixture({
  name: 'feat',
  sha: 'sha-feat',
  current: false,
  upstream: 'origin/feat',
  ahead: 2,
})

// mountPicker mounts the picker on refs and summary, its list closed.
function mountPicker(refs: Refs, summary: Summary = summaryFixture({ sha: 'sha-main' })) {
  useBranchesStore().refs = refs
  useWorktreesStore().summary = summary
  return mountInBody(BranchPicker, {
    props: { wt: '/repo' },
    global: { stubs: { transition: false } },
  })
}

// openPicker mounts the picker on refs and summary and opens its list.
async function openPicker(refs: Refs = refsOfEveryKind([main, feat]), summary?: Summary) {
  const { wrapper, body } = await mountPicker(refs, summary)
  await wrapper.get('[data-test="branch-picker"]').trigger('click')
  await flushPromises()
  return { wrapper, body }
}

// refRow is the <li> whose ref-name is exactly name; the row and its
// sibling checkbox both live inside it.
function refRow(body: DOMWrapper<Element>, name: string): DOMWrapper<Element> {
  return body.findAll('.picker-rows li').find((li) => {
    const ref = li.find('.ref-name')
    return ref.exists() && ref.text() === name
  })!
}

// checkoutRow clicks a ref row's own clickable part, checking it out.
async function checkoutRow(body: DOMWrapper<Element>, name: string): Promise<void> {
  await refRow(body, name).get('[data-test="ref-row"]').trigger('click')
}

// openActions right-clicks name's row, opening its context menu.
async function openActions(body: DOMWrapper<Element>, name: string): Promise<void> {
  await refRow(body, name).get('[data-test="ref-row"]').trigger('contextmenu')
  await flushPromises()
}

async function pickAction(body: DOMWrapper<Element>, name: string, action: string): Promise<void> {
  await openActions(body, name)
  await clickMenuItem(body, action)
  await flushPromises()
}

// listOpen tells whether the branch list is showing.
function listOpen(body: DOMWrapper<Element>): Promise<boolean> {
  return popoverShows(body, '.branch-list')
}

describe('BranchPicker', () => {
  beforeEach(() => {
    window.innerWidth = 1024 // medium: the default every test not itself about layout width expects
    setActivePinia(createPinia())
    AppService.SelectWorktree.mockReset().mockResolvedValue(undefined)
    Facade.Checkout.mockReset().mockResolvedValue(undefined)
    Facade.UpdateBranch.mockReset().mockResolvedValue('job-1')
    Facade.RenameBranch.mockReset().mockResolvedValue(undefined)
    Facade.DeleteBranch.mockReset().mockResolvedValue(undefined)
    Facade.Merge.mockReset().mockResolvedValue({ outcome: 'merged' })
    Facade.Rebase.mockReset().mockResolvedValue({ outcome: 'rebased' })
    Facade.Status.mockReset().mockResolvedValue(
      changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] }),
    )
    AppService.OpenDiff.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
  })

  it('the button names the current branch; the list starts with "New branch…", then Local, Remote and Tags', async () => {
    const { wrapper, body } = await openPicker()

    expect(wrapper.get('[data-test="branch-picker"]').text()).toContain('main')
    const rows = body.findAll('.picker-rows li').map((li) => li.text())
    expect(rows[0]).toBe('New branch…')
    expect(rows.slice(1).map((t) => t.replace(/\s+/g, ' ').trim())).toEqual([
      'Local',
      'main',
      expect.stringMatching(/^feat\s*2/),
      'Remote',
      expect.stringMatching(/^origin\/x/),
      'Tags',
      expect.stringMatching(/^v1/),
    ])
    expect(refRow(body, 'main').find('[data-test="current-marker"]').exists()).toBe(true)
    expect(refRow(body, 'feat').find('[data-test="current-marker"]').exists()).toBe(false)
  })

  it('the button shows ahead/behind while the branch tracks an upstream, and the operation in progress', async () => {
    const { wrapper } = await mountPicker(
      refsOfEveryKind([branchFixture({ upstream: 'origin/main', ahead: 2, behind: 1 })]),
      summaryFixture({ operation: OperationKind.MergeOperation }),
    )

    const button = wrapper.get('[data-test="branch-picker"]')
    expect(button.get('[data-test="ahead-behind"]').text()).toBe('2 1')
    expect(button.text()).toContain('merge')
  })

  it('the button shows no ahead/behind without an upstream', async () => {
    const { wrapper } = await mountPicker(refsOfEveryKind([main]), summaryFixture())

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('narrow (400px): the button shows its icon only, the branch name in title and aria-label, and no ahead/behind even with an upstream', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountPicker(
      refsOfEveryKind([branchFixture({ upstream: 'origin/main', ahead: 2, behind: 1 })]),
      summaryFixture(),
    )

    const button = wrapper.get('[data-test="branch-picker"]')
    expectIconOnly(button, 'main')
    expect(button.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('while detached the button shows the short hash and says so', async () => {
    const { wrapper } = await mountPicker(
      refsOfEveryKind([]),
      summaryFixture({ branch: '', detached: true, sha: 'abc1234def' }),
    )

    const text = wrapper.get('[data-test="branch-picker"]').text()
    expect(text).toContain('abc1234')
    expect(text).toContain('detached')
  })

  it('a click checks out a local branch, a remote one by its full ref name, and asks before detaching at a tag; the current one does nothing', async () => {
    const { wrapper, body } = await openPicker()

    await checkoutRow(body, 'main')
    expect(Facade.Checkout).not.toHaveBeenCalled()

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    await checkoutRow(body, 'feat')
    await flushPromises()
    expect(Facade.Checkout).toHaveBeenLastCalledWith('/repo', 'feat')

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    await checkoutRow(body, 'origin/x')
    await flushPromises()
    expect(Facade.Checkout).toHaveBeenLastCalledWith('/repo', 'refs/remotes/origin/x')

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    await checkoutRow(body, 'v1')
    expect(wrapper.emitted('checkout-commit')).toEqual([['sha-v1']])
    expect(Facade.Checkout).toHaveBeenCalledTimes(2)
  })

  it('a remote branch whose short name a local branch has checks out that local branch', async () => {
    const x = branchFixture({ name: 'x', sha: 'sha-local-x', current: false })
    const { body } = await openPicker(refsOfEveryKind([main, x]))

    await checkoutRow(body, 'origin/x')
    await flushPromises()

    expect(Facade.Checkout).toHaveBeenCalledWith('/repo', 'x')
  })

  it.each([
    [
      'a local branch checked out in another worktree',
      branchFixture({ name: 'side', current: false, checkedOutIn: '/repo-side' }),
      [],
      'side',
      '',
    ],
    [
      'a local branch another worktree is rebasing or bisecting, which the event includes',
      branchFixture({
        name: 'side',
        current: false,
        checkedOutIn: '/repo-side',
        inUse: BranchInUse.BranchRebasing,
      }),
      [],
      'side',
      BranchInUse.BranchRebasing,
    ],
    [
      'a remote branch whose local branch of the same name is checked out elsewhere',
      branchFixture({ name: 'side', current: false, checkedOutIn: '/repo-side' }),
      [{ name: 'origin', branches: [{ name: 'side', sha: 'sha-side' }] }],
      'origin/side',
      '',
    ],
  ])(
    '%s asks instead of checking out, by click or "Check out"',
    async (_name, side, remotes, row, inUse) => {
      const { wrapper, body } = await openPicker({
        branches: [main, side],
        remotes,
        tags: [],
      })

      await checkoutRow(body, row)
      expect(await listOpen(body)).toBe(false)

      await wrapper.get('[data-test="branch-picker"]').trigger('click')
      await flushPromises()
      await pickAction(body, row, 'Check out')

      const event = ['side', '/repo-side', inUse]
      expect(wrapper.emitted('checkout-elsewhere')).toEqual([event, event])
      expect(AppService.SelectWorktree).not.toHaveBeenCalled()
      expect(Facade.Checkout).not.toHaveBeenCalled()
    },
  )

  it("a checkout git refuses shows git's reason in the list, gone when the list opens again", async () => {
    Facade.Checkout.mockRejectedValueOnce(wailsError('would be overwritten', 'DIRTY_WORKTREE'))
    const { wrapper, body } = await openPicker()
    await checkoutRow(body, 'feat')
    await flushPromises()
    expect(body.get('.branch-list .error-banner').text()).toContain('would be overwritten')

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    expect(await listOpen(body)).toBe(false)
    await wrapper.get('[data-test="branch-picker"]').trigger('click')

    expect(await listOpen(body)).toBe(true)
    expect(body.find('.branch-list .error-banner').exists()).toBe(false)
  })

  it('typing keeps only the matching refs, under their section; Enter checks out the first', async () => {
    const { body } = await openPicker()
    const filter = body.get('.picker-list input')

    await filter.setValue('fe')

    const rows = body.findAll('.picker-rows li').map((li) => li.text().replace(/\s+/g, ' ').trim())
    expect(rows).toEqual(['Local', expect.stringMatching(/^feat/)])
    await filter.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(Facade.Checkout).toHaveBeenCalledWith('/repo', 'feat')
  })

  it('"New branch…" starts the branch at the current branch, picked in a list by its full ref name', async () => {
    const { wrapper, body } = await openPicker()

    await body.findAll('.picker-rows li')[0]!.get('[data-test="picker-action"]').trigger('click')

    expect(wrapper.emitted('create-branch')).toEqual([[{ ref: 'refs/heads/main' }]])
  })

  it('"New branch…" while detached starts the branch at HEAD, fixed', async () => {
    const detached = summaryFixture({ branch: '', detached: true, sha: 'abc1234567' })
    const { wrapper, body } = await openPicker(refsOfEveryKind([feat]), detached)

    await body.findAll('.picker-rows li')[0]!.get('[data-test="picker-action"]').trigger('click')

    expect(wrapper.emitted('create-branch')).toEqual([[{ rev: 'abc1234567', label: 'abc1234' }]])
  })

  it("a right-click on a local branch opens its actions: check out, new branch, new worktree, update, rename, delete and show in log; the current branch's lack check out, update, rename and delete", async () => {
    const { body } = await openPicker()
    expect(body.find('[data-test="ref-actions"]').exists()).toBe(false)

    await openActions(body, 'feat')
    expect(menuLabels(body)).toEqual([
      'Check out',
      'New branch from here…',
      'New worktree from here…',
      'Update from upstream',
      'Rename…',
      'Delete',
      'Merge into main',
      'Rebase main onto feat',
      'Show in log',
    ])

    await openActions(body, 'main')
    expect(menuLabels(body)).toEqual([
      'New branch from here…',
      'New worktree from here…',
      'Show in log',
    ])
  })

  it("a remote branch's actions offer check out, new branch, new worktree and show in log; a tag's also show history", async () => {
    const { body } = await openPicker()
    const expected = (name: string) => [
      'Check out',
      'New branch from here…',
      'New worktree from here…',
      'Merge into main',
      `Rebase main onto ${name}`,
      'Show in log',
    ]

    await openActions(body, 'origin/x')
    expect(menuLabels(body)).toEqual(expected('origin/x'))
    await openActions(body, 'v1')
    expect(menuLabels(body)).toEqual([...expected('v1'), 'Show history'])
  })

  it('a detached HEAD names itself in the merge and rebase actions', async () => {
    const { body } = await openPicker(
      refsOfEveryKind([branchFixture({ current: false }), feat]),
      summaryFixture({ detached: true, sha: 'abcdef1234567' }),
    )

    await openActions(body, 'feat')

    expect(menuLabels(body)).toContain('Merge into abcdef1 (detached)')
    expect(menuLabels(body)).toContain('Rebase abcdef1 (detached) onto feat')
  })

  it.each([
    ['FR-BR-11', 'Merge', 'Merge into main', 'fastForward', 'Fast-forwarded main to feat'],
    ['FR-BR-11', 'Merge', 'Merge into main', 'merged', 'Merged feat into main'],
    ['FR-BR-11', 'Merge', 'Merge into main', 'upToDate', 'main is already up to date with feat'],
    ['FR-BR-12', 'Rebase', 'Rebase main onto feat', 'rebased', 'Rebased main onto feat'],
    [
      'FR-BR-12',
      'Rebase',
      'Rebase main onto feat',
      'upToDate',
      'main is already up to date with feat',
    ],
  ] as const)(
    '%s "%s" action calls the full ref; outcome %s shows a success notice (%#)',
    async (_id, kind, action, outcome, text) => {
      Facade[kind].mockResolvedValueOnce({ outcome })
      const notice = await mountNotices()
      const { body } = await openPicker()

      await pickAction(body, 'feat', action)

      expect(Facade[kind]).toHaveBeenCalledWith('/repo', 'refs/heads/feat', false)
      expect(notice.get('.p-toast').text()).toContain(text)
    },
  )

  it.each([
    ['FR-BR-11', 'Merge', 'Merge into main', 'Stash and merge'],
    ['FR-BR-12', 'Rebase', 'Rebase main onto feat', 'Stash and rebase'],
  ] as const)(
    '%s a %s refused for local changes offers "%s", which retries with stash',
    async (_id, kind, action, stashAction) => {
      Facade[kind].mockRejectedValueOnce(wailsError('local changes in the way', 'DIRTY_WORKTREE'))
      const notice = await mountNotices()
      const { body } = await openPicker()

      await pickAction(body, 'feat', action)
      await findButton(notice, stashAction)!.trigger('click')

      expect(Facade[kind]).toHaveBeenLastCalledWith('/repo', 'refs/heads/feat', true)
    },
  )

  it('a merge that stops on conflicts opens the diff window on the first conflicted file, with no notice', async () => {
    Facade.Merge.mockRejectedValueOnce(wailsError('conflicts', Code.Conflicted))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Merge into main')
    await flushPromises()

    expect(notice.find('.p-toast-message').exists()).toBe(false)
    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: DiffKind.WorkingDiff,
      conflicted: true,
      path: 'a.txt',
      oldPath: '',
    })
  })

  it('a merge that stops for another reason shows the reason as a notice', async () => {
    Facade.Merge.mockRejectedValueOnce(wailsError('hook failed', 'STOPPED'))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Merge into main')

    expect(notice.get('.p-toast').text()).toContain('hook failed')
  })

  it('clicking a row\'s "…" opens the same menu as its right-click', async () => {
    const { body } = await openPicker()

    await openActions(body, 'feat')
    const fromRightClick = menuLabels(body)

    await refRow(body, 'feat').get('[data-test="row-actions"]').trigger('click')
    await flushPromises()

    expect(menuLabels(body)).toEqual(fromRightClick)
  })

  it('the right-clicked entry becomes the active one', async () => {
    const { body } = await openPicker()

    await openActions(body, 'v1')

    expect(body.get('.picker-rows li.active').text()).toContain('v1')
  })

  it("Shift+F10 or the context-menu key in the filter opens the active entry's actions", async () => {
    const { body } = await openPicker()
    const filter = body.get('.picker-list input')

    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'ContextMenu' })
    await flushPromises()
    expect(menuLabels(body)).not.toContain('Check out') // main, the current branch

    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'F10', shiftKey: true })
    await flushPromises()
    expect(menuLabels(body)).toContain('Check out') // feat
  })

  it('choosing an action closes the list', async () => {
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Rename…')

    expect(await listOpen(body)).toBe(false)
  })

  it('"New branch from here…" starts the branch at that branch or tag, picked in a list by its full ref name', async () => {
    const { wrapper, body } = await openPicker()

    for (const name of ['feat', 'origin/x', 'v1']) {
      await pickAction(body, name, 'New branch from here…')
      await wrapper.get('[data-test="branch-picker"]').trigger('click')
    }

    expect(wrapper.emitted('create-branch')).toEqual([
      [{ ref: 'refs/heads/feat' }],
      [{ ref: 'refs/remotes/origin/x' }],
      [{ ref: 'refs/tags/v1' }],
    ])
  })

  it('"New worktree from here…" uses a free local branch itself, else starts a new branch at the ref', async () => {
    const { wrapper, body } = await openPicker()

    for (const name of ['feat', 'main', 'origin/x', 'v1']) {
      await pickAction(body, name, 'New worktree from here…')
      await wrapper.get('[data-test="branch-picker"]').trigger('click')
    }

    expect(wrapper.emitted('create-worktree')).toEqual([
      [{ newBranch: false, branch: 'feat' }],
      [{ newBranch: true, start: { ref: 'refs/heads/main' } }],
      [{ newBranch: true, start: { ref: 'refs/remotes/origin/x' } }],
      [{ newBranch: true, start: { ref: 'refs/tags/v1' } }],
    ])
  })

  it('Escape closes the actions menu and the list', async () => {
    const { body } = await openPicker()
    await openActions(body, 'feat')
    expect(menuLabels(body)).not.toEqual([])

    document.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape' }))

    expect(await listOpen(body)).toBe(false)
    expect(menuLabels(body)).toEqual([])
  })

  it('"Update from upstream" starts the update of that branch', async () => {
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Update from upstream')

    expect(Facade.UpdateBranch).toHaveBeenCalledWith('/repo', 'feat')
  })

  it('a local branch without an upstream offers no "Update from upstream"', async () => {
    const local = branchFixture({ name: 'local', current: false })
    const { body } = await openPicker(refsOfEveryKind([main, feat, local]))

    await openActions(body, 'feat')
    expect(menuLabels(body)).toContain('Update from upstream')

    await openActions(body, 'local')
    expect(menuLabels(body)).not.toContain('Update from upstream')
  })

  it("an action's refusal shows git's reason as a notice, the list being closed", async () => {
    Facade.UpdateBranch.mockRejectedValueOnce(wailsError('no upstream configured', 'NO_UPSTREAM'))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Update from upstream')

    expect(await listOpen(body)).toBe(false)
    expect(notice.get('.p-toast').text()).toContain('no upstream configured')
  })

  it('"Rename…" opens the rename dialog on the branch', async () => {
    const { wrapper, body } = await openPicker()

    await pickAction(body, 'feat', 'Rename…')

    expect(wrapper.findComponent(RenameBranchDialog).props()).toMatchObject({
      wt: '/repo',
      name: 'feat',
    })
  })

  it('"Delete" deletes a merged branch without asking', async () => {
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(Facade.DeleteBranch).toHaveBeenCalledWith('/repo', 'feat', false)
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it("deleting an unmerged branch asks first, naming git's reason, then deletes forced", async () => {
    Facade.DeleteBranch.mockRejectedValueOnce(wailsError('not fully merged', 'UNMERGED_BRANCH'))
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(confirmOptions().message).toContain('not fully merged')
    expect(body.find('.p-toast-message').exists()).toBe(false)
    confirmOptions().accept()
    await flushPromises()
    expect(Facade.DeleteBranch).toHaveBeenLastCalledWith('/repo', 'feat', true)
  })

  it('a delete refused for another reason shows the error as a notice, no confirmation', async () => {
    Facade.DeleteBranch.mockRejectedValueOnce(wailsError('used by worktree', 'GIT_FAILED'))
    const notice = await mountNotices()
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(notice.get('.p-toast').text()).toContain('used by worktree')
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it('"Show in log" jumps the log to the ref\'s commit and asks for the log view', async () => {
    const { wrapper, body } = await openPicker()
    const jumpTo = vi.spyOn(useLogStore(), 'jumpTo').mockResolvedValue(undefined)

    await pickAction(body, 'v1', 'Show in log')

    expect(jumpTo).toHaveBeenCalledWith('sha-v1')
    expect(wrapper.emitted('show-log')).toHaveLength(1)
  })

  it('a tag\'s "Show history" closes the list, filters the log to that tag by its full ref name, keeping the other fields, and asks for the log view', async () => {
    const { wrapper, body } = await openPicker()
    const log = useLogStore()
    log.filter = { path: 'src/', branches: ['refs/heads/feat'] }
    const applyFilter = vi.spyOn(log, 'applyFilter').mockResolvedValue(undefined)

    await pickAction(body, 'v1', 'Show history')

    expect(await listOpen(body)).toBe(false)
    expect(applyFilter).toHaveBeenCalledWith({ path: 'src/', branches: ['refs/tags/v1'] })
    expect(wrapper.emitted('show-log')).toHaveLength(1)
  })
})
