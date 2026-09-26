import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { CherryPick: vi.fn(), Revert: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import PickDialog from '../PickDialog.vue'
import { mountInBody } from '@/testing/mountInBody'
import { footerButtons } from '@/testing/dialog'

function mountDialog(verb: 'cherry-pick' | 'revert', shas: string[]) {
  return mountInBody(PickDialog, { props: { wt: '/repo', verb, shas } })
}

describe('PickDialog', () => {
  beforeEach(() => {
    Facade.CherryPick.mockReset().mockResolvedValue(undefined)
    Facade.Revert.mockReset().mockResolvedValue(undefined)
  })

  it('FR-CP-06 does not render the "cherry picked from" checkbox for a revert', async () => {
    const { body } = await mountDialog('revert', ['a1'])

    expect(body.text()).not.toContain('cherry picked from')
  })

  it("FR-CP-06 Confirm calls CherryPick with the form's own options", async () => {
    const { body } = await mountDialog('cherry-pick', ['a1'])
    const checkboxes = body.findAll('input[type=checkbox]')
    await checkboxes[0]!.setValue(true) // Apply without committing
    await checkboxes[1]!.setValue(true) // Add "cherry picked from" line

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.CherryPick).toHaveBeenCalledTimes(1)
    expect(Facade.CherryPick).toHaveBeenCalledWith('/repo', ['a1'], {
      noCommit: true,
      recordOrigin: true,
    })
  })

  it('FR-RV-04 Confirm calls Revert, ignoring recordOrigin for a revert', async () => {
    const { body } = await mountDialog('revert', ['a1', 'a2'])
    await body.get('input[type=checkbox]').setValue(true) // Apply without committing

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Revert).toHaveBeenCalledTimes(1)
    expect(Facade.Revert).toHaveBeenCalledWith('/repo', ['a1', 'a2'], {
      noCommit: true,
      recordOrigin: false,
    })
  })

  it("FR-CP-06 a rejection renders in this dialog's own ErrorBanner and keeps it open", async () => {
    Facade.CherryPick.mockRejectedValueOnce(new Error('boom', { cause: { message: 'conflict' } }))
    const { wrapper, body } = await mountDialog('cherry-pick', ['a1'])

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('conflict')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-UX-17 Cancel and Confirm sit together at the bottom; Cancel closes', async () => {
    const { wrapper, body } = await mountDialog('cherry-pick', ['a1'])

    expect(footerButtons(body)).toEqual(['Cancel', 'Confirm'])
    await body.get('[data-test="dialog-cancel"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
