import { expect } from 'vitest'
import type { DOMWrapper } from '@vue/test-utils'

// expectIconOnly checks that button shows no text and names its action in
// title and aria-label. title defaults to name.
export function expectIconOnly(
  button: Pick<DOMWrapper<Element>, 'text' | 'attributes'>,
  name: string,
  title = name,
): void {
  expect(button.text()).toBe('')
  expect(button.attributes('title')).toBe(title)
  expect(button.attributes('aria-label')).toBe(name)
}
