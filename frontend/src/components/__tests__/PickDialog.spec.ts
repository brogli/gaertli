// Requirements: FR-CONF-09, FR-CP-06, FR-RV-04
import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { CherryPick: vi.fn(), Revert: vi.fn(), Status: vi.fn() },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ Facade, AppService }))

import PickDialog from '../PickDialog.vue'
import { DiffKind } from '@/lib/diffTarget'
import { wailsError } from '@/testing/errors'
import { Code } from '../../../bindings/github.com/brogli/gaertli/internal/core'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { mountInBody } from '@/testing/mountInBody'

function mountDialog(verb: 'cherry-pick' | 'revert', shas: string[]) {
  return mountInBody(PickDialog, { props: { wt: '/repo', verb, shas } })
}

describe('PickDialog', () => {
  beforeEach(() => {
    Facade.CherryPick.mockReset().mockResolvedValue(undefined)
    Facade.Revert.mockReset().mockResolvedValue(undefined)
    Facade.Status.mockReset().mockResolvedValue(
      changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] }),
    )
    AppService.OpenDiff.mockReset().mockResolvedValue(undefined)
  })

  it('does not render the "cherry picked from" checkbox for a revert', async () => {
    const { body } = await mountDialog('revert', ['a1'])

    expect(body.text()).not.toContain('cherry picked from')
  })

  it("Confirm calls CherryPick with the form's own options", async () => {
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

  it('Confirm calls Revert, ignoring recordOrigin for a revert', async () => {
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

  it("a rejection renders in this dialog's own ErrorBanner and keeps it open", async () => {
    Facade.CherryPick.mockRejectedValueOnce(wailsError('conflict'))
    const { wrapper, body } = await mountDialog('cherry-pick', ['a1'])

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('conflict')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('a stop on conflicts opens the diff window and closes the dialog', async () => {
    Facade.CherryPick.mockRejectedValueOnce(wailsError('conflicts', Code.Conflicted))
    const { wrapper, body } = await mountDialog('cherry-pick', ['a1'])

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: DiffKind.WorkingDiff,
      conflicted: true,
      path: 'a.txt',
      oldPath: '',
    })
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
