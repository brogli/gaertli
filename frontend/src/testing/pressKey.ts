// pressKey dispatches a window-level keydown, as a shortcut sees it.
export function pressKey(key: string, init: KeyboardEventInit = {}): void {
  window.dispatchEvent(new KeyboardEvent('keydown', { key, ...init }))
}
