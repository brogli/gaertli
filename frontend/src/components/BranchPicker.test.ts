import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

const { AppService, Facade } = vi.hoisted(() => ({
  AppService: { SelectWorktree: vi.fn() },
  Facade: {
    Checkout: vi.fn(),
    UpdateBranch: vi.fn(),
    RenameBranch: vi.fn(),
    DeleteBranch: vi.fn(),
  },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ AppService, Facade }))

vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
import { confirmOptions, confirmRequire } from '../testing/confirm'

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))

import BranchPicker from './BranchPicker.vue'
import RenameBranchDialog from './RenameBranchDialog.vue'
import { useRefsStore } from '../stores/refs'
import { useRepoStore } from '../stores/repo'
import { useLogStore } from '../stores/log'
import type { Branch, Refs, Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '../testing/fixtures/branch'
import { summaryFixture } from '../testing/fixtures/summary'
import { clickMenuItem, menuLabels } from '../testing/contextMenu'

const main = branchFixture()
const feat = branchFixture({
  name: 'feat',
  sha: 'sha-feat',
  current: false,
  upstream: 'origin/feat',
  ahead: 2,
})

function refsWith(branches: Branch[] = [main, feat]): Refs {
  return {
    branches,
    remotes: [{ name: 'origin', branches: [{ name: 'x', sha: 'sha-x' }] }],
    tags: [{ name: 'v1', sha: 'sha-v1' }],
  }
}

// mountPicker mounts the picker on refs and summary, its list closed.
function mountPicker(
  refs: Refs,
  summary: Summary = summaryFixture({ sha: 'sha-main' }),
  props: { wt: string; distro?: string } = { wt: '/repo' },
) {
  useRefsStore().refs = refs
  useRepoStore().summary = summary
  return mountInBody(BranchPicker, { props, global: { stubs: { transition: false } } })
}

// openPicker mounts the picker on refs and opens its list.
async function openPicker(
  refs: Refs = refsWith(),
  props: { wt: string; distro?: string } = { wt: '/repo' },
) {
  const { wrapper, body } = await mountPicker(refs, undefined, props)
  await wrapper.get('[data-test="branch-picker"]').trigger('click')
  await flushPromises()
  return { wrapper, body }
}

// refRow is the <li> whose ref-name is exactly name; the row and its
// sibling checkbox/actions button both live inside it.
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

async function openActions(body: DOMWrapper<Element>, name: string): Promise<void> {
  await refRow(body, name).get('[data-test="ref-actions"]').trigger('click')
  await flushPromises()
}

async function pickAction(body: DOMWrapper<Element>, name: string, action: string): Promise<void> {
  await openActions(body, name)
  await clickMenuItem(body, action, 'menu')
  await flushPromises()
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
    confirmRequire.mockReset()
  })

  it('FR-BR-09 the button names the current branch; the list starts with "New branch…", then Local, Remote and Tags', async () => {
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

  it('FR-REPO-03 the button shows ahead/behind while the branch tracks an upstream, and the operation in progress', async () => {
    const { wrapper } = await mountPicker(
      refsWith([branchFixture({ upstream: 'origin/main', ahead: 2, behind: 1 })]),
      summaryFixture({ operation: 'merge' }),
    )

    const button = wrapper.get('[data-test="branch-picker"]')
    expect(button.get('[data-test="ahead-behind"]').text()).toBe('2 1')
    expect(button.text()).toContain('merge')
  })

  it('FR-REPO-03 the button shows no ahead/behind without an upstream', async () => {
    const { wrapper } = await mountPicker(refsWith([main]), summaryFixture())

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('FR-UX-09 narrow (400px): the button shows no ahead/behind, even with an upstream', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountPicker(
      refsWith([branchFixture({ upstream: 'origin/main', ahead: 2, behind: 1 })]),
      summaryFixture(),
    )

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('FR-UX-09 narrow (400px): the button shows its icon only, the branch name in title and aria-label', async () => {
    window.innerWidth = 400
    const { wrapper } = await mountPicker(refsWith([main]), summaryFixture())

    const button = wrapper.get('[data-test="branch-picker"]')
    expect(button.text()).toBe('')
    expect(button.attributes('title')).toBe('main')
    expect(button.attributes('aria-label')).toBe('main')
  })

  it('FR-BR-08 while detached the button shows the short hash and says so', async () => {
    const { wrapper } = await mountPicker(
      refsWith([]),
      summaryFixture({ branch: '', detached: true, sha: 'abc1234def' }),
    )

    const text = wrapper.get('[data-test="branch-picker"]').text()
    expect(text).toContain('abc1234')
    expect(text).toContain('detached')
  })

  it('FR-BR-09 a click checks out a local branch, a remote one by its short name, and asks before detaching at a tag; the current one does nothing', async () => {
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
    expect(Facade.Checkout).toHaveBeenLastCalledWith('/repo', 'x')

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    await checkoutRow(body, 'v1')
    expect(wrapper.emitted('checkout-commit')).toEqual([['sha-v1']])
    expect(Facade.Checkout).toHaveBeenCalledTimes(2)
  })

  it('FR-BR-09 a local branch checked out in another worktree switches this window there, by click or "Check out"', async () => {
    const side = branchFixture({
      name: 'side',
      sha: 'sha-side',
      current: false,
      checkedOutIn: '/repo-side',
    })
    const { wrapper, body } = await openPicker(refsWith([main, feat, side]))

    await checkoutRow(body, 'side')
    await flushPromises()
    expect(AppService.SelectWorktree).toHaveBeenLastCalledWith('/repo-side')

    await wrapper.get('[data-test="branch-picker"]').trigger('click')
    await flushPromises()
    await pickAction(body, 'side', 'Check out')

    expect(AppService.SelectWorktree).toHaveBeenCalledTimes(2)
    expect(AppService.SelectWorktree).toHaveBeenLastCalledWith('/repo-side')
    expect(Facade.Checkout).not.toHaveBeenCalled()
  })

  it("FR-BR-09 a WSL window switches to the branch's worktree by its UNC path", async () => {
    const side = branchFixture({ name: 'side', current: false, checkedOutIn: '/home/u/r-side' })
    const { body } = await openPicker(refsWith([main, side]), {
      wt: '\\\\wsl.localhost\\Ubuntu\\home\\u\\r',
      distro: 'Ubuntu',
    })

    await checkoutRow(body, 'side')
    await flushPromises()

    expect(AppService.SelectWorktree).toHaveBeenCalledWith(
      '\\\\wsl.localhost\\Ubuntu\\home\\u\\r-side',
    )
  })

  it("FR-BR-05 a checkout git refuses shows git's reason in the list", async () => {
    Facade.Checkout.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { code: 'DIRTY_WORKTREE', message: 'would be overwritten' },
      }),
    )
    const { body } = await openPicker()

    await checkoutRow(body, 'feat')
    await flushPromises()

    expect(body.get('.branch-list .error-banner').text()).toContain('would be overwritten')
  })

  it('FR-BR-09 typing keeps only the matching refs, under their section; Enter checks out the first', async () => {
    const { body } = await openPicker()
    const filter = body.get('.picker-list input')

    await filter.setValue('fe')

    const rows = body.findAll('.picker-rows li').map((li) => li.text().replace(/\s+/g, ' ').trim())
    expect(rows).toEqual(['Local', expect.stringMatching(/^feat/)])
    await filter.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(Facade.Checkout).toHaveBeenCalledWith('/repo', 'feat')
  })

  it('FR-BR-01 "New branch…" asks for a branch at HEAD', async () => {
    const { wrapper, body } = await openPicker()

    await body.findAll('.picker-rows li')[0]!.get('[data-test="picker-action"]').trigger('click')

    expect(wrapper.emitted('create-branch')).toEqual([['sha-main']])
  })

  it("FR-BR-09 a local branch's actions offer check out, new branch, new worktree, update, rename, delete and show in log; the current branch's lack check out, update, rename and delete", async () => {
    const { body } = await openPicker()

    await openActions(body, 'feat')
    expect(menuLabels(body)).toEqual([
      'Check out',
      'New branch from here…',
      'New worktree from here…',
      'Update from upstream',
      'Rename…',
      'Delete',
      'Show in log',
    ])

    await openActions(body, 'main')
    expect(menuLabels(body)).toEqual([
      'New branch from here…',
      'New worktree from here…',
      'Show in log',
    ])
  })

  it("FR-BR-09 a remote branch's and a tag's actions offer check out, new branch, new worktree and show in log", async () => {
    const { body } = await openPicker()
    const expected = [
      'Check out',
      'New branch from here…',
      'New worktree from here…',
      'Show in log',
    ]

    await openActions(body, 'origin/x')
    expect(menuLabels(body)).toEqual(expected)
    await openActions(body, 'v1')
    expect(menuLabels(body)).toEqual(expected)
  })

  it('FR-BR-01 "New branch from here…" asks for a branch starting at the ref', async () => {
    const { wrapper, body } = await openPicker()

    for (const name of ['feat', 'origin/x', 'v1']) {
      await pickAction(body, name, 'New branch from here…')
      await wrapper.get('[data-test="branch-picker"]').trigger('click')
    }

    expect(wrapper.emitted('create-branch')).toEqual([['feat'], ['origin/x'], ['v1']])
  })

  it('FR-WT-06 "New worktree from here…" uses a free local branch itself, else starts a new branch at the ref', async () => {
    const { wrapper, body } = await openPicker()

    for (const name of ['feat', 'main', 'origin/x', 'v1']) {
      await pickAction(body, name, 'New worktree from here…')
      await wrapper.get('[data-test="branch-picker"]').trigger('click')
    }

    expect(wrapper.emitted('create-worktree')).toEqual([
      [{ newBranch: false, branch: 'feat' }],
      [{ newBranch: true, start: 'main' }],
      [{ newBranch: true, start: 'origin/x' }],
      [{ newBranch: true, start: 'v1' }],
    ])
  })

  it('FR-BR-09 closing the list closes its open actions menu', async () => {
    const { body } = await openPicker()
    await openActions(body, 'feat')
    expect(menuLabels(body)).not.toEqual([])

    document.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape' }))
    await flushPromises()
    await new Promise((resolve) => requestAnimationFrame(resolve))
    await flushPromises()

    expect(body.find('.branch-list').exists()).toBe(false)
    expect(menuLabels(body)).toEqual([])
  })

  it('FR-UPD-01 "Update from upstream" starts the update of that branch', async () => {
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Update from upstream')

    expect(Facade.UpdateBranch).toHaveBeenCalledWith('/repo', 'feat')
  })

  it('FR-UX-01 a refused update shows its error in the list', async () => {
    Facade.UpdateBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { message: 'no upstream configured', code: 'NO_UPSTREAM' },
      }),
    )
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Update from upstream')

    expect(body.get('.branch-list .error-banner').text()).toContain('no upstream configured')
  })

  it('FR-BR-06 "Rename…" opens the rename dialog on the branch', async () => {
    const { wrapper, body } = await openPicker()

    await pickAction(body, 'feat', 'Rename…')

    expect(wrapper.findComponent(RenameBranchDialog).props()).toMatchObject({
      wt: '/repo',
      name: 'feat',
    })
  })

  it('FR-BR-06 "Delete" deletes a merged branch without asking', async () => {
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(Facade.DeleteBranch).toHaveBeenCalledWith('/repo', 'feat', false)
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it("FR-UX-03 deleting an unmerged branch asks first, naming git's reason, then deletes forced", async () => {
    Facade.DeleteBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { code: 'UNMERGED_BRANCH', message: 'not fully merged' },
      }),
    )
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(confirmOptions().message).toContain('not fully merged')
    expect(body.find('.branch-list .error-banner').exists()).toBe(false)
    confirmOptions().accept()
    await flushPromises()
    expect(Facade.DeleteBranch).toHaveBeenLastCalledWith('/repo', 'feat', true)
  })

  it('FR-BR-06 a delete refused for another reason shows the error, no confirmation', async () => {
    Facade.DeleteBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { code: 'GIT_FAILED', message: 'used by worktree' },
      }),
    )
    const { body } = await openPicker()

    await pickAction(body, 'feat', 'Delete')

    expect(body.get('.branch-list .error-banner').text()).toContain('used by worktree')
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it('FR-LOG-01 "Show in log" jumps the log to the ref\'s commit', async () => {
    const { body } = await openPicker()
    const jumpTo = vi.spyOn(useLogStore(), 'jumpTo').mockResolvedValue(undefined)

    await pickAction(body, 'v1', 'Show in log')

    expect(jumpTo).toHaveBeenCalledWith('sha-v1')
  })

  it('FR-UPD-06 checking two eligible branches offers "Update 2 branches" at the top, which updates both', async () => {
    const other = branchFixture({
      name: 'other',
      sha: 'sha-other',
      current: false,
      upstream: 'origin/other',
    })
    const { body } = await openPicker(refsWith([main, feat, other]))

    const boxes = body.findAll('[data-test="update-select"]')
    expect(boxes).toHaveLength(2)
    await boxes[0]!.setValue(true)
    await boxes[1]!.setValue(true)
    const update = body.findAll('.picker-rows li')[1]!
    expect(update.text()).toBe('Update 2 branches')
    await update.get('[data-test="picker-action"]').trigger('click')
    await flushPromises()

    expect(Facade.UpdateBranch).toHaveBeenCalledWith('/repo', 'feat')
    expect(Facade.UpdateBranch).toHaveBeenCalledWith('/repo', 'other')
    expect(Facade.Checkout).not.toHaveBeenCalled()
  })

  it('FR-UPD-06 the current branch and one without an upstream have no checkbox; a checkbox names its branch', async () => {
    const noUpstream = branchFixture({ name: 'local', current: false })
    const { body } = await openPicker(refsWith([main, feat, noUpstream]))

    const boxes = body.findAll('[data-test="update-select"]')
    expect(boxes).toHaveLength(1)
    expect(boxes[0]!.attributes('title')).toBe('Select feat to update')
    expect(boxes[0]!.attributes('aria-label')).toBe('Select feat to update')
  })
})
