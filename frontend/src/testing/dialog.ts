import { expect } from 'vitest'
import type { BaseWrapper, DOMWrapper, VueWrapper } from '@vue/test-utils'

// fieldLabels are the open dialog's field labels in order, each checked to
// name an element by its id.
export function fieldLabels(body: DOMWrapper<Element>): string[] {
  return body.findAll('.p-dialog label[for]').map((label) => {
    expect(body.find(`[id="${label.attributes('for')}"]`).exists()).toBe(true)
    return label.text()
  })
}

// field is the control in root that the label named label points at.
export function field(
  root: BaseWrapper<Node>,
  label: string,
): Omit<DOMWrapper<HTMLInputElement>, 'exists'> {
  const found = root.findAll('label[for]').find((l) => l.text() === label)
  expect(found, `a label "${label}"`).toBeTruthy()
  return root.get<HTMLInputElement>(`[id="${found!.attributes('for')}"]`)
}

// footerButtons are the names of the buttons in the open dialog's footer.
export function footerButtons(body: DOMWrapper<Element>): string[] {
  return body.findAll('.p-dialog-footer button').map((b) => b.text())
}

// expectDialogFooter checks the open dialog's footer holds exactly buttons,
// Cancel first, and that Cancel closes the dialog.
export async function expectDialogFooter(
  wrapper: VueWrapper,
  body: DOMWrapper<Element>,
  buttons: string[],
): Promise<void> {
  expect(footerButtons(body)).toEqual(buttons)
  await body.get('[data-test="dialog-cancel"]').trigger('click')
  expect(wrapper.emitted('close')).toHaveLength(1)
}
