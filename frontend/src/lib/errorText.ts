import { i18n } from '../i18n'
import type { CoreError } from '../composables/useCoreCall'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'

// errorHeadline is the text an error shows first: the catalogue text for
// error.key once en.json has an entry for it, INTERNAL's fixed text for a
// bug or other non-git failure, or the English message as a fallback (an
// unrecognised key, or none at all).
export function errorHeadline(error: CoreError): string {
  const { global } = i18n
  if (error.code === Code.Internal) return global.t('errors.INTERNAL')
  const key = error.key ? `errors.${error.key}` : ''
  return key && global.te(key) ? global.t(key, error.args ?? {}) : error.message
}

// errorDetail is what "Show details" reveals below the headline: git's
// command and stderr. An internal error has neither, so its own message
// goes there instead, untranslated technical detail rather than something
// to show by default.
export function errorDetail(error: CoreError): string {
  return error.code === Code.Internal
    ? error.message
    : [error.command, error.gitStderr].filter(Boolean).join('\n')
}

// hasDetail tells whether an error has details worth a toggle: an editor
// start failure's message already names the command.
export function hasDetail(error: CoreError): boolean {
  return error.code !== Code.EditorNotFound
}
