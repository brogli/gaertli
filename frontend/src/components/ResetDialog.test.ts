import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockReset, mockUncommittedCount, mockRequire } = vi.hoisted(() => ({
  mockReset: vi.fn(),
  mockUncommittedCount: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Reset: mockReset, UncommittedCount: mockUncommittedCount },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import ResetDialog from './ResetDialog.vue'
import { mountInBody } from '../testing/mountInBody'

function mountDialog(branch = 'main') {
  return mountInBody(ResetDialog, {
    props: { wt: '/repo', branch, sha: 'abc123', subject: 'Fix the bug' },
  })
}

// capturedOptions reads the options passed to require()'s first call.
function capturedOptions(): { message: string; accept: () => void } {
  return mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
}

describe('ResetDialog', () => {
  beforeEach(() => {
    mockReset.mockReset()
    mockUncommittedCount.mockReset()
    mockUncommittedCount.mockResolvedValue(0)
    mockRequire.mockReset()
  })

  it('FR-BR-02 renders the branch, the target subject, and all three mode explanations', async () => {
    const { body } = await mountDialog()

    expect(body.text()).toContain('main')
    expect(body.text()).toContain('Fix the bug')
    expect(body.text()).toContain('moves the branch pointer. Index and working files are unchanged.')
    expect(body.text()).toContain('moves the branch pointer and resets the index. Working files are unchanged.')
    expect(body.text()).toContain(
      'moves the branch pointer, resets the index, and overwrites working files. Uncommitted changes are lost.',
    )
  })

  it('FR-BR-07 renders a fourth radio option, Keep, with its fixed explanation', async () => {
    const { body } = await mountDialog()

    expect(body.find('input#reset-mode-keep').exists()).toBe(true)
    expect(body.text()).toContain(
      'moves the branch pointer, resets the index and working files, but refuses if any local change would be lost.',
    )
  })

  it('FR-BR-07 clicking Reset at keep mode calls Facade.Reset directly, no confirmation', async () => {
    mockReset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('input#reset-mode-keep').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockReset).toHaveBeenCalledTimes(1)
    expect(mockReset).toHaveBeenCalledWith('/repo', 'abc123', 'keep')
    expect(mockRequire).not.toHaveBeenCalled()
  })

  it('FR-BR-02 renders "HEAD" instead of a blank branch name while detached', async () => {
    const { body } = await mountDialog('')

    expect(body.text()).toContain('HEAD')
  })

  it('FR-BR-02 clicking Reset at the default mixed mode calls Facade.Reset directly, no confirmation', async () => {
    mockReset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockReset).toHaveBeenCalledTimes(1)
    expect(mockReset).toHaveBeenCalledWith('/repo', 'abc123', 'mixed')
    expect(mockRequire).not.toHaveBeenCalled()
  })

  it('FR-BR-04 shows the previous HEAD after a successful reset, and Close closes the dialog', async () => {
    mockReset.mockResolvedValue({ previousHead: 'abc123' })
    const { wrapper, body } = await mountDialog()

    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('abc123')

    await body.get('[data-test="reset-close"]').trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-BR-03 hard mode confirms first, naming the uncommitted count, before calling Facade.Reset', async () => {
    mockUncommittedCount.mockResolvedValue(4)
    mockReset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockReset).not.toHaveBeenCalled()
    expect(mockRequire).toHaveBeenCalledTimes(1)
    expect(capturedOptions().message).toContain('4')

    capturedOptions().accept()
    await flushPromises()

    expect(mockReset).toHaveBeenCalledTimes(1)
    expect(mockReset).toHaveBeenCalledWith('/repo', 'abc123', 'hard')
  })

  it('FR-UX-03 never calls Facade.Reset if the confirmation is not accepted (QA-SAFE-01)', async () => {
    mockUncommittedCount.mockResolvedValue(4)
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockRequire).toHaveBeenCalledTimes(1)
    expect(mockReset).not.toHaveBeenCalled()
  })
})
