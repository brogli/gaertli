import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { RenameBranch: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import RenameBranchDialog from '../RenameBranchDialog.vue'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { fieldLabels, footerButtons } from '@/testing/dialog'

describe('RenameBranchDialog', () => {
  beforeEach(() => {
    Facade.RenameBranch.mockReset()
  })

  it('FR-BR-06 pre-fills the input with the current name and submits the edited name', async () => {
    Facade.RenameBranch.mockResolvedValue(undefined)
    const { body } = await mountInBody(RenameBranchDialog, {
      props: { wt: '/repo', name: 'feature' },
    })

    const input = body.get('input').element
    expect(input.value).toBe('feature')

    await body.get('input').setValue('renamed')
    await body.get('[data-test="rename-branch-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.RenameBranch).toHaveBeenCalledTimes(1)
    expect(Facade.RenameBranch).toHaveBeenCalledWith('/repo', 'feature', 'renamed')
  })

  it('FR-BR-06 shows the rejection message and keeps the dialog open on failure', async () => {
    Facade.RenameBranch.mockRejectedValueOnce(wailsError('"-bad" is not a valid branch name'))
    const { wrapper, body } = await mountInBody(RenameBranchDialog, {
      props: { wt: '/repo', name: 'feature' },
    })

    await body.get('input').setValue('-bad')
    await body.get('[data-test="rename-branch-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-UX-17 the name has its own label; Cancel and Rename sit together at the bottom', async () => {
    const { wrapper, body } = await mountInBody(RenameBranchDialog, {
      props: { wt: '/repo', name: 'feature' },
    })

    expect(fieldLabels(body)).toEqual(['Branch name'])
    expect(footerButtons(body)).toEqual(['Cancel', 'Rename'])
    await body.get('[data-test="dialog-cancel"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
