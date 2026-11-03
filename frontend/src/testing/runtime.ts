import { vi, type Mock } from 'vitest'

// Stand-ins for @wailsio/runtime. A test file routes the module here:
//
//   vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
//
// Events.On records every listener and returns a no-op unlisten.
// Window.Name resolves to OWN_WINDOW, the window the test runs in.
const OWN_WINDOW = 'window-1'
export const Events: { On: Mock } = { On: vi.fn(() => () => {}) }
export const Window: { Close: Mock; Name: Mock } = {
  Close: vi.fn(),
  Name: vi.fn(() => Promise.resolve(OWN_WINDOW)),
}
export const System: { IsWindows: Mock } = { IsWindows: vi.fn(() => false) }

// emit fires every handler registered for name, as the runtime does. The
// event is for this window unless ev names another sender.
export function emit(name: string, ev: object = {}): void {
  for (const [registered, handler] of Events.On.mock.calls)
    if (registered === name) handler({ sender: OWN_WINDOW, ...ev })
}
