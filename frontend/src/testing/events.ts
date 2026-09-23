import type { Mock } from 'vitest'

// registeredHandler returns the first handler registered for name through
// a mocked Events.On, so a test can fire the event itself.
export function registeredHandler(on: Mock, name: string): ((ev: object) => void) | undefined {
  return on.mock.calls.find((call) => call[0] === name)?.[1]
}
