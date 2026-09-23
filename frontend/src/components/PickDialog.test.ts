import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCherryPickWith, mockRevertWith } = vi.hoisted(() => ({
  mockCherryPickWith: vi.fn(),
  mockRevertWith: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CherryPickWith: mockCherryPickWith, RevertWith: mockRevertWith },
}))

import PickDialog from './PickDialog.vue'
import { mountInBody } from '../testing/mountInBody'

function mountDialog(verb: 'cherry-pick' | 'revert', shas: string[], singleMerge: boolean) {
  return mountInBody(PickDialog, { props: { wt: '/repo', verb, shas, singleMerge } })
}

describe('PickDialog', () => {
  beforeEach(() => {
    mockCherryPickWith.mockReset().mockResolvedValue(undefined)
    mockRevertWith.mockReset().mockResolvedValue(undefined)
  })

  it('FR-CP-08 renders "Pick against parent" only for a single selected merge commit', async () => {
    const { body } = await mountDialog('cherry-pick', ['m1'], true)

    expect(body.find('[data-test="pick-mainline"]').exists()).toBe(true)
  })

  it('FR-CP-08 does not render "Pick against parent" for a non-merge single selection', async () => {
    const { body } = await mountDialog('cherry-pick', ['a1'], false)

    expect(body.find('[data-test="pick-mainline"]').exists()).toBe(false)
  })

  it('FR-CP-08 does not render "Pick against parent" for a revert, even of a single merge commit', async () => {
    const { body } = await mountDialog('revert', ['m1'], true)

    expect(body.find('[data-test="pick-mainline"]').exists()).toBe(false)
  })

  it('FR-CP-06 does not render the "cherry picked from" checkbox for a revert', async () => {
    const { body } = await mountDialog('revert', ['a1'], false)

    expect(body.text()).not.toContain('cherry picked from')
  })

  it('FR-CP-06, FR-CP-08 Confirm calls CherryPickWith with the form\'s own options', async () => {
    const { body } = await mountDialog('cherry-pick', ['m1'], true)
    const checkboxes = body.findAll('input[type=checkbox]')
    await checkboxes[0]!.setValue(true) // Apply without committing
    await checkboxes[1]!.setValue(true) // Add "cherry picked from" line
    await body.get('[data-test="pick-mainline"]').setValue(2)

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(mockCherryPickWith).toHaveBeenCalledTimes(1)
    expect(mockCherryPickWith).toHaveBeenCalledWith('/repo', ['m1'], {
      noCommit: true,
      recordOrigin: true,
      mainline: 2,
    })
  })

  it('FR-RV-04 Confirm calls RevertWith, ignoring recordOrigin/mainline for a revert', async () => {
    const { body } = await mountDialog('revert', ['a1', 'a2'], false)
    await body.get('input[type=checkbox]').setValue(true) // Apply without committing

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(mockRevertWith).toHaveBeenCalledTimes(1)
    expect(mockRevertWith).toHaveBeenCalledWith('/repo', ['a1', 'a2'], {
      noCommit: true,
      recordOrigin: false,
      mainline: 0,
    })
  })

  it('FR-CP-06 a rejection renders in this dialog\'s own ErrorBanner and keeps it open', async () => {
    mockCherryPickWith.mockRejectedValueOnce(Object.assign(new Error('boom'), { cause: { message: 'conflict' } }))
    const { wrapper, body } = await mountDialog('cherry-pick', ['a1'], false)

    await body.get('[data-test="pick-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('conflict')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
