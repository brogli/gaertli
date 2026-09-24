import { vi, type Mock } from 'vitest'

// ConfirmOptions is what a component passes to useConfirm().require.
type ConfirmOptions = { header: string; message: string; accept: () => void }

// confirmRequire records every confirmation a component asks for. A test
// file routes openvue/useconfirm here:
//
//   vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
export const confirmRequire: Mock = vi.fn()

export function useConfirm(): { require: Mock; close: Mock } {
  return { require: confirmRequire, close: vi.fn() }
}

// confirmOptions is the options of the n-th confirmation asked for.
export function confirmOptions(n = 0): ConfirmOptions {
  return confirmRequire.mock.calls[n]![0] as ConfirmOptions
}
