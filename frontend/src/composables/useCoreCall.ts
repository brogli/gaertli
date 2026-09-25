import { ref } from 'vue'
import { Code, type Error as GoError } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CoreError is core.Error as a rejected bound-method call carries it in
// .cause, plus retry once run() attaches it. Only message is sure: a
// rejection that is no core error has nothing else.
export type CoreError = Partial<GoError> & Pick<GoError, 'message'> & { retry?: () => void }

// useCoreCall wraps a bound-method call: run(call) awaits it,
// catching a rejection and exposing it as a CoreError for ErrorBanner.
// onError, when given, sees the error first; returning true means it
// handled it (e.g. as a notice), so run leaves error unset.
export function useCoreCall() {
  const error = ref<CoreError | null>(null)

  async function run<T>(
    call: () => Promise<T>,
    onError?: (error: CoreError) => boolean,
  ): Promise<T | undefined> {
    error.value = null
    try {
      return await call()
    } catch (e) {
      const err = toCoreError(e)
      // A held lock is worth a retry: call is the exact call that failed.
      if (err.code === Code.Locked) err.retry = () => void run(call, onError)
      if (!onError?.(err)) error.value = err
      return undefined
    }
  }

  // runAndClose is run's "then close on success" shape for a dialog:
  // onSuccess runs only when call did not reject, so a failure leaves the
  // error set and the dialog open.
  async function runAndClose<T>(call: () => Promise<T>, onSuccess: () => void): Promise<void> {
    await run(call)
    if (!error.value) onSuccess()
  }

  return { error, run, runAndClose }
}

// toCoreError reads a rejected bound-method call's core.Error, or wraps any
// other rejection's text.
function toCoreError(e: unknown): CoreError {
  const cause = e && typeof e === 'object' ? (e as { cause?: unknown }).cause : undefined
  if (cause && typeof cause === 'object' && 'message' in cause) {
    return cause as CoreError
  }
  return { message: e instanceof Error ? e.message : String(e) }
}
