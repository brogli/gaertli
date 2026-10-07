// wailsError is a rejection as the generated bindings raise it: the core's
// error rides in cause.
export function wailsError(message: string, code?: string): Error {
  return new Error('boom', { cause: code === undefined ? { message } : { code, message } })
}

// editorNotFound is the rejection of an editor command missing from PATH.
export function editorNotFound(editor: string): Error {
  return wailsError(`"${editor}" is not on PATH`, 'EDITOR_NOT_FOUND')
}
