import type { App } from 'vue'
import { LogService } from '../../bindings/github.com/wailsapp/wails/v3/pkg/services/log'

// logSafely never rejects: a failure sending the log call itself must not
// become another unhandledrejection.
function logSafely(message: string): void {
  LogService.Error(message).catch(() => {})
}

// installErrorLogging sends every uncaught error to the app log: a
// component error, a plain window error, and a promise nothing awaited.
export function installErrorLogging(app: App): void {
  app.config.errorHandler = (err) => logSafely(String(err))
  window.addEventListener('error', (ev) => logSafely(ev.message))
  window.addEventListener('unhandledrejection', (ev) => logSafely(String(ev.reason)))
}
