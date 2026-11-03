import { onKeyStroke, type KeyFilter } from '@vueuse/core'

// ContextMenu's own root list is role menubar; a submenu (nested items) is
// role menu.
const OVERLAY_SELECTOR = '[role="dialog"], [role="alertdialog"], [role="menu"], [role="menubar"]'
const TEXT_FIELD_SELECTOR = 'input, textarea, [role="combobox"]'

export interface UseShortcutOptions {
  // Also skips a key whose target is an input, textarea or [role=combobox],
  // so the field's own handling (e.g. the log search's Escape) keeps it.
  inTextField?: boolean
}

// useShortcut is a window-level shortcut that skips its handler while a
// dialog, popup or menu is open (OpenVue's Dialog, Popover and ContextMenu
// close on their own document listener without stopping the event, so a
// plain onKeyStroke would otherwise fire behind them too; a closing overlay
// is still in the DOM while the key event runs). A listbox does not count:
// the repo picker's list is one even as a window's whole content.
// preventDefault always runs for a matched key, also when the handler is
// skipped, so e.g. F5 never reloads the WebView with a dialog open.
export function useShortcut(
  key: KeyFilter,
  handler: (event: KeyboardEvent) => void,
  options: UseShortcutOptions = {},
): void {
  onKeyStroke(key, (event) => {
    event.preventDefault()
    if (document.querySelector(OVERLAY_SELECTOR)) return
    if (
      options.inTextField === false &&
      event.target instanceof Element &&
      event.target.closest(TEXT_FIELD_SELECTOR)
    )
      return
    handler(event)
  })
}
