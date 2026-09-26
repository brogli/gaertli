import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { Code } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../useCoreCall'

describe('useCoreCall', () => {
  it('FR-UX-16 a LOCKED rejection carries a retry that re-runs the same call', async () => {
    const call = vi
      .fn()
      .mockRejectedValueOnce({ cause: { code: Code.Locked, message: 'locked' } })
      .mockResolvedValueOnce('ok')
    const { error, run } = useCoreCall()

    await run(call)
    expect(call).toHaveBeenCalledTimes(1)
    expect(error.value?.retry).toBeTypeOf('function')

    error.value?.retry?.()
    await flushPromises()

    expect(call).toHaveBeenCalledTimes(2)
    expect(error.value).toBeNull()
  })

  it('FR-UX-16 a non-LOCKED rejection carries no retry', async () => {
    const call = vi.fn().mockRejectedValueOnce({ cause: { code: Code.GitFailed, message: 'x' } })
    const { error, run } = useCoreCall()

    await run(call)

    expect(error.value?.retry).toBeUndefined()
  })
})
