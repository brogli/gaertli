import { ref, type Ref } from 'vue'
import { Code, type Error as GoError } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CoreError is core.Error as a rejected bound-method call carries it in
// .cause, plus retry once run() attaches it. Only message is sure: a
// rejection that is no core error has nothing else.
export type CoreError = Partial<GoError> & Pick<GoError, 'message'> & { retry?: () => void }

export interface UseCoreCall {
  error: Ref<CoreError | null>
  run: <T>(
    call: () => Promise<T>,
    onError?: (error: CoreError) => boolean,
  ) => Promise<T | undefined>
  runAndClose: <T>(call: () => Promise<T>, onSuccess: () => void) => Promise<void>
}

// useCoreCall wraps a bound-method call: run(call) awaits it,
// catching a rejection and exposing it as a CoreError for ErrorBanner.
// onError, when given, sees the error first; returning true means it
// handled it (e.g. as a notice), so run leaves error unset.
export function useCoreCall(): UseCoreCall {
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
  const cause = hasCause(e) ? e.cause : undefined
  if (isCoreError(cause)) return cause
  return { message: e instanceof Error ? e.message : String(e) }
}

function hasCause(e: unknown): e is { cause: unknown } {
  return typeof e === 'object' && e !== null && 'cause' in e
}

function isCoreError(cause: unknown): cause is CoreError {
  return typeof cause === 'object' && cause !== null && 'message' in cause
}
