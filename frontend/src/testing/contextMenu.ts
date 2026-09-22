import type { DOMWrapper } from '@vue/test-utils'

// clickMenuItem clicks the open context menu's item labelled label.
export async function clickMenuItem(body: DOMWrapper<Element>, label: string): Promise<void> {
  const labels = body.findAll('.p-contextmenu-item-label')
  const index = labels.findIndex((l) => l.text() === label)
  await body.findAll('.p-contextmenu-item-content')[index]!.trigger('click')
}
