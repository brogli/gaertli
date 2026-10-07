import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { AppService } = vi.hoisted(() => ({ AppService: { SelectWorktree: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ AppService }))

import BranchInWorktreeDialog from '../BranchInWorktreeDialog.vue'
import { BranchInUse } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { footerButtons } from '@/testing/dialog'

describe('BranchInWorktreeDialog', () => {
  beforeEach(() => {
    AppService.SelectWorktree.mockReset().mockResolvedValue(undefined)
  })

  it('FR-BR-05 names the branch and the worktree, and shows the full path', async () => {
    const { body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side' },
    })

    const text = body.get('.p-dialog').text()
    expect(text).toContain('side')
    expect(text).toContain('r-side')
    expect(body.get('.p-dialog').text()).toContain('/home/u/r-side')
    expect(footerButtons(body)).toEqual(['Cancel', 'Switch to worktree'])
  })

  it('FR-BR-05 names the branch being rebased in the other worktree', async () => {
    const { body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side', inUse: BranchInUse.BranchRebasing },
    })

    const text = body.get('.p-dialog').text()
    expect(text).toContain('Branch in use elsewhere')
    expect(text).toContain('side is being rebased in the worktree r-side')
    expect(footerButtons(body)).toEqual(['Cancel', 'Switch to worktree'])
  })

  it('FR-BR-05 names the branch being bisected in the other worktree', async () => {
    const { body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side', inUse: BranchInUse.BranchBisecting },
    })

    const text = body.get('.p-dialog').text()
    expect(text).toContain('Branch in use elsewhere')
    expect(text).toContain('side is being bisected in the worktree r-side')
    expect(footerButtons(body)).toEqual(['Cancel', 'Switch to worktree'])
  })

  it('FR-BR-05 a plainly checked-out branch keeps the checked-out wording, no operation named', async () => {
    const { body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side' },
    })

    const text = body.get('.p-dialog').text()
    expect(text).toContain('Branch checked out elsewhere')
    expect(text).toContain('side is checked out in the worktree r-side')
    expect(footerButtons(body)).toEqual(['Cancel', 'Switch to worktree'])
  })

  it('FR-BR-05 switches to the worktree and closes', async () => {
    const { wrapper, body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side' },
    })

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(AppService.SelectWorktree).toHaveBeenCalledWith('/home/u/r-side')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it("FR-BR-05 a WSL window switches by the branch's worktree UNC path", async () => {
    const { body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side', distro: 'Ubuntu' },
    })

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(AppService.SelectWorktree).toHaveBeenCalledWith(
      '\\\\wsl.localhost\\Ubuntu\\home\\u\\r-side',
    )
  })

  it('FR-BR-05 a rejection shows in the banner and keeps the dialog open', async () => {
    AppService.SelectWorktree.mockRejectedValueOnce(wailsError('worktree is gone'))
    const { wrapper, body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side' },
    })

    await body.get('[data-test="branch-in-worktree-confirm"]').trigger('click')
    await flushPromises()

    expect(body.get('.error-banner').text()).toContain('worktree is gone')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-BR-05 Cancel closes without calling SelectWorktree', async () => {
    const { wrapper, body } = await mountInBody(BranchInWorktreeDialog, {
      props: { branch: 'side', path: '/home/u/r-side' },
    })

    await body.get('[data-test="branch-in-worktree-cancel"]').trigger('click')

    expect(AppService.SelectWorktree).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
