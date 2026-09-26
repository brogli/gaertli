import { describe, expect, it, vi } from 'vitest'

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

import { useConfirmAction } from '../useConfirmAction'

describe('useConfirmAction', () => {
  it('QA-SAFE-01 requires header and message, calling onAccept only once accept fires', () => {
    confirmRequire.mockReset()
    const { confirmAction } = useConfirmAction()
    const onAccept = vi.fn()

    confirmAction('H', 'M', onAccept)

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(options.header).toBe('H')
    expect(options.message).toBe('M')
    expect(onAccept).not.toHaveBeenCalled()

    options.accept()

    expect(onAccept).toHaveBeenCalledTimes(1)
  })
})
