import {
  DOMWrapper,
  flushPromises,
  mount,
  type ComponentMountingOptions,
  type VueWrapper,
} from '@vue/test-utils'

// mountInBody mounts component attached to document.body, where OpenVue
// teleports dialogs and menus, and waits for it to settle. body wraps
// document.body for queries into the teleported content.
export async function mountInBody<T>(
  component: T,
  options: ComponentMountingOptions<T> = {},
): Promise<{ wrapper: VueWrapper; body: DOMWrapper<Element> }> {
  const wrapper = mount(component, { ...options, attachTo: document.body })
  await flushPromises()
  return { wrapper, body: new DOMWrapper(document.body) }
}
