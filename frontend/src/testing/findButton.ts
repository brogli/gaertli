import type { BaseWrapper, DOMWrapper } from '@vue/test-utils'

// findButton is the first button in wrapper named text: its visible text,
// or its aria-label for a compact button that carries the name there
// instead.
export function findButton(
  wrapper: BaseWrapper<Node>,
  text: string,
): DOMWrapper<HTMLButtonElement> | undefined {
  return wrapper
    .findAll('button')
    .find((b) => b.text() === text || b.attributes('aria-label') === text)
}
