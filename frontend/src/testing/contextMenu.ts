import type { DOMWrapper } from '@vue/test-utils'

type Menu = 'contextmenu' | 'menu' | 'tieredmenu'

// clickMenuItem clicks the open menu's item labelled label: a ContextMenu's
// by default, a popup Menu's with menu 'menu', a SplitButton's TieredMenu
// with menu 'tieredmenu'.
export async function clickMenuItem(
  body: DOMWrapper<Element>,
  label: string,
  menu: Menu = 'contextmenu',
): Promise<void> {
  const index = menuLabels(body, menu).indexOf(label)
  await body.findAll(`.p-${menu}-item-content`)[index]!.trigger('click')
}

// menuLabels lists the open menu's item labels: a popup Menu's by default,
// any other with menu as for clickMenuItem.
export function menuLabels(body: DOMWrapper<Element>, menu: Menu = 'menu'): string[] {
  return body.findAll(`.p-${menu}-item-label`).map((l) => l.text())
}
