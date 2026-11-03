import { flushPromises, type DOMWrapper } from '@vue/test-utils'

// popoverShows tells whether selector matches in body once a popover's
// opening or closing has settled, an animation frame after it was asked for.
export async function popoverShows(body: DOMWrapper<Element>, selector: string): Promise<boolean> {
  await flushPromises()
  await new Promise((resolve) => requestAnimationFrame(resolve))
  await flushPromises()
  return body.find(selector).exists()
}
