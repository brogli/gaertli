import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { App } from 'vue'

const { LogService } = vi.hoisted(() => ({
  LogService: { Error: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('../../../bindings/github.com/wailsapp/wails/v3/pkg/services/log', () => ({
  LogService,
}))

import { installErrorLogging } from '../errorLogging'

function fakeApp(): App {
  // eslint-disable-next-line @typescript-eslint/consistent-type-assertions -- test double stands in for vue's App, which it does not fully implement; installErrorLogging only touches app.config.errorHandler
  return { config: {} } as App
}

describe('installErrorLogging', () => {
  beforeEach(() => {
    LogService.Error.mockReset().mockResolvedValue(undefined)
  })

  it('FR-UX-02 a component error reaches the app log', () => {
    const app = fakeApp()
    installErrorLogging(app)

    app.config.errorHandler?.(new Error('boom'), null, 'render')

    expect(LogService.Error).toHaveBeenCalledWith(expect.stringContaining('boom'))
  })

  it('FR-UX-02 an uncaught window error reaches the app log', () => {
    installErrorLogging(fakeApp())

    window.dispatchEvent(new ErrorEvent('error', { message: 'window boom' }))

    expect(LogService.Error).toHaveBeenCalledWith(expect.stringContaining('window boom'))
  })

  it('FR-UX-02 an unhandled promise rejection reaches the app log', () => {
    installErrorLogging(fakeApp())
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions -- test double stands in for PromiseRejectionEvent, which happy-dom does not implement
    const event = new Event('unhandledrejection') as PromiseRejectionEvent
    Object.defineProperty(event, 'reason', { value: 'rejected boom' })

    window.dispatchEvent(event)

    expect(LogService.Error).toHaveBeenCalledWith(expect.stringContaining('rejected boom'))
  })
})
