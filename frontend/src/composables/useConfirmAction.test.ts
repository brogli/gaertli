import { describe, expect, it, vi } from 'vitest'

const { mockRequire } = vi.hoisted(() => ({ mockRequire: vi.fn() }))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import { useConfirmAction } from './useConfirmAction'

describe('useConfirmAction', () => {
  it('QA-SAFE-01 requires header and message, calling onAccept only once accept fires', () => {
    mockRequire.mockReset()
    const { confirmAction } = useConfirmAction()
    const onAccept = vi.fn()

    confirmAction('H', 'M', onAccept)

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { header: string; message: string; accept: () => void }
    expect(options.header).toBe('H')
    expect(options.message).toBe('M')
    expect(onAccept).not.toHaveBeenCalled()

    options.accept()

    expect(onAccept).toHaveBeenCalledTimes(1)
  })
})
