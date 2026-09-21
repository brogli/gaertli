import { ref } from 'vue'

// CoreError is the shape core.Error serialises to, read off a rejected
// bound-method call's .cause (research/wails/service-pattern-bindings-errors.md).
export interface CoreError {
  message: string
  gitStderr?: string
  command?: string
  code?: string
}

// useCoreCall wraps a bound-method call (FR-UX-01): run(call) awaits it,
// catching a rejection and exposing it as a CoreError for ErrorBanner.
export function useCoreCall() {
  const error = ref<CoreError | null>(null)

  async function run<T>(call: () => Promise<T>): Promise<T | undefined> {
    error.value = null
    try {
      return await call()
    } catch (e) {
      error.value = toCoreError(e)
      return undefined
    }
  }

  return { error, run }
}

function toCoreError(e: unknown): CoreError {
  const cause = e && typeof e === 'object' ? (e as { cause?: unknown }).cause : undefined
  if (cause && typeof cause === 'object' && 'message' in cause) {
    return cause as CoreError
  }
  return { message: e instanceof Error ? e.message : String(e) }
}
