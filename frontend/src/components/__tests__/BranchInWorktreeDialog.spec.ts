// Requirements: FR-BR-05
import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { AppService } = vi.hoisted(() => ({ AppService: { SelectWorktree: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ AppService }))

import BranchInWorktreeDialog from '../BranchInWorktreeDialog.vue'
import { BranchInUse } from '../../../bindings/github.com/brogli/gaertli/internal/core'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { footerButtons } from '@/testing/dialog'

// mountDialog mounts the dialog for the branch side, checked out in
// /home/u/r-side.
function mountDialog(props: { inUse?: BranchInUse | undefined; distro?: string } = {}) {
  return mountInBody(BranchInWorktreeDialog, {
    props: { branch: 'side', path: '/home/u/r-side', ...props },
  })
}

describe('BranchInWorktreeDialog', () => {
  beforeEach(() => {
    AppService.SelectWorktree.mockReset().mockResolvedValue(undefined)
  })

  it('names the branch and the worktree, and shows the full path', async () => {
    const { body } = await mountDialog()

    expect(body.get('.p-dialog').text()).toContain('/home/u/r-side')
    expect(footerButtons(body)).toEqual(['Cancel', 'Switch to worktree'])
  })

  it.each([
    {
      state: 'checked out',
      inUse: undefined,
      title: 'Branch checked out elsewhere',
      sentence: 'side is checked out in the worktree r-side',
    },
    {
      state: 'rebased',
      inUse: BranchInUse.BranchRebasing,
      title: 'Branch in use elsewhere',
      sentence: 'side is being rebased in the worktree r-side',
    },
    {
      state: 'bisected',
      inUse: BranchInUse.BranchBisecting,
      title: 'Branch in use elsewhere',
      sentence: 'side is being bisected in the worktree r-side',
    },
  ])('names the branch $state in the other worktree', async ({ inUse, title, sentence }) => {
    const { body } = await mountDialog({ inUse })

    const text = body.get('.p-dialog').text()
    expect(text).toContain(title)
    expect(text).toContain(sentence)
  })

  it('switches to the worktree and closes', async () => {
    const { wrapper, body } = await mountDialog()

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(AppService.SelectWorktree).toHaveBeenCalledWith('/home/u/r-side')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it("a WSL window switches by the branch's worktree UNC path", async () => {
    const { body } = await mountDialog({ distro: 'Ubuntu' })

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(AppService.SelectWorktree).toHaveBeenCalledWith(
      '\\\\wsl.localhost\\Ubuntu\\home\\u\\r-side',
    )
  })

  it('a rejection shows in the banner and keeps the dialog open', async () => {
    AppService.SelectWorktree.mockRejectedValueOnce(wailsError('worktree is gone'))
    const { wrapper, body } = await mountDialog()

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(body.get('.error-banner').text()).toContain('worktree is gone')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('Cancel closes without calling SelectWorktree', async () => {
    const { wrapper, body } = await mountDialog()

    await body.get('[data-test="branch-in-worktree-cancel"]').trigger('click')

    expect(AppService.SelectWorktree).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
