import { flushPromises, type DOMWrapper } from '@vue/test-utils'

// chooseOption picks label in the OpenVue Select marked data-test=testId:
// a click opens its overlay (teleported to body), a mousedown on the option
// chooses it (Select selects on mousedown, not click).
export async function chooseOption(
  body: DOMWrapper<Element>,
  testId: string,
  label: string,
): Promise<void> {
  await body.get(`[data-test="${testId}"]`).trigger('click')
  await flushPromises()
  await body.get(`[role="option"][aria-label="${label}"]`).trigger('mousedown')
  await flushPromises()
}
