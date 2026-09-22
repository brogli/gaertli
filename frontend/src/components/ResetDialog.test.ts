import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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

function mountDialog(branch = 'main') {
  return mount(ResetDialog, {
    props: { wt: '/repo', branch, sha: 'abc123', subject: 'Fix the bug', visible: true },
    attachTo: document.body,
  })
}

// capturedOptions reads the options passed to require()'s first call.
function capturedOptions(): { message: string; accept: () => void } {
  return mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
}

describe('ResetDialog', () => {
  let wrapper: VueWrapper | undefined

  beforeEach(() => {
    mockReset.mockReset()
    mockUncommittedCount.mockReset()
    mockUncommittedCount.mockResolvedValue(0)
    mockRequire.mockReset()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('FR-BR-02 renders the branch, the target subject, and all three mode explanations', async () => {
    wrapper = mountDialog()
    await flushPromises()
    const body = new DOMWrapper(document.body)

    expect(body.text()).toContain('main')
    expect(body.text()).toContain('Fix the bug')
    expect(body.text()).toContain('moves the branch pointer. Index and working files are unchanged.')
    expect(body.text()).toContain('moves the branch pointer and resets the index. Working files are unchanged.')
    expect(body.text()).toContain(
      'moves the branch pointer, resets the index, and overwrites working files. Uncommitted changes are lost.',
    )
  })

  it('FR-BR-02 renders "HEAD" instead of a blank branch name while detached', async () => {
    wrapper = mountDialog('')
    await flushPromises()
    const body = new DOMWrapper(document.body)

    expect(body.text()).toContain('HEAD')
  })

  it('FR-BR-02 clicking Reset at the default mixed mode calls Facade.Reset directly, no confirmation', async () => {
    mockReset.mockResolvedValue({ previousHead: 'def456' })
    wrapper = mountDialog()
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockReset).toHaveBeenCalledTimes(1)
    expect(mockReset).toHaveBeenCalledWith('/repo', 'abc123', 'mixed')
    expect(mockRequire).not.toHaveBeenCalled()
  })

  it('FR-BR-04 shows the previous HEAD after a successful reset, and Close emits update:visible false', async () => {
    mockReset.mockResolvedValue({ previousHead: 'abc123' })
    wrapper = mountDialog()
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('abc123')

    await body.get('[data-test="reset-close"]').trigger('click')

    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('FR-BR-03 hard mode confirms first, naming the uncommitted count, before calling Facade.Reset', async () => {
    mockUncommittedCount.mockResolvedValue(4)
    mockReset.mockResolvedValue({ previousHead: 'def456' })
    wrapper = mountDialog()
    await flushPromises()
    const body = new DOMWrapper(document.body)

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
    wrapper = mountDialog()
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('input#reset-mode-hard').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(mockRequire).toHaveBeenCalledTimes(1)
    expect(mockReset).not.toHaveBeenCalled()
  })
})
