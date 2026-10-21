// Requirements: FR-LOG-05, FR-UX-05
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref, type VNodeChild } from 'vue'
import type { KeyFilter } from '@vueuse/core'
import Dialog from 'openvue/dialog'
import Popover from 'openvue/popover'
import ContextMenu from 'openvue/contextmenu'
import { useShortcut, type UseShortcutOptions } from '../useShortcut'

// mountShortcut registers a shortcut via useShortcut, rendering whatever
// overlay markup content() returns alongside it, attached to document.body,
// where OpenVue teleports dialogs, popovers and menus.
function mountShortcut(
  match: KeyFilter,
  options: UseShortcutOptions = {},
  content: () => VNodeChild = () => null,
) {
  const handler = vi.fn()
  const Host = defineComponent({
    setup() {
      useShortcut(match, handler, options)
      return () => h('div', [content()])
    },
  })
  return { wrapper: mount(Host, { attachTo: document.body }), handler }
}

// pressKey dispatches a window-level keydown and reports whether the guard
// prevented its default.
function pressKey(key = 'k'): boolean {
  const event = new KeyboardEvent('keydown', { key, cancelable: true })
  window.dispatchEvent(event)
  return event.defaultPrevented
}

describe('useShortcut', () => {
  it('skips the handler while an OpenVue Dialog is open, runs it once closed', async () => {
    const dialogVisible = ref(false)
    const { handler } = mountShortcut('k', {}, () =>
      h(Dialog, {
        visible: dialogVisible.value,
        'onUpdate:visible': (v: boolean) => (dialogVisible.value = v),
      }),
    )
    dialogVisible.value = true
    await flushPromises()

    pressKey()
    expect(handler).not.toHaveBeenCalled()

    dialogVisible.value = false
    await flushPromises()
    pressKey()

    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('skips the handler for an open Popover, and for a shown ContextMenu, always preventing default', async () => {
    const popoverRef = ref<InstanceType<typeof Popover>>()
    const menuRef = ref<InstanceType<typeof ContextMenu>>()
    const { wrapper, handler } = mountShortcut('k', {}, () =>
      h('div', [
        h('button', {
          'data-test': 'popover-trigger',
          onClick: (e: MouseEvent) => popoverRef.value?.toggle(e),
        }),
        h('div', {
          'data-test': 'menu-trigger',
          onContextmenu: (e: MouseEvent) => menuRef.value?.show(e),
        }),
        h(Popover, { ref: popoverRef }, { default: () => h('span', 'popover content') }),
        h(ContextMenu, { ref: menuRef, model: [{ label: 'Item' }] }),
      ]),
    )

    await wrapper.get('[data-test="popover-trigger"]').trigger('click')
    await flushPromises()
    expect(pressKey()).toBe(true)
    expect(handler).not.toHaveBeenCalled()

    await wrapper.get('[data-test="menu-trigger"]').trigger('contextmenu')
    await flushPromises()
    expect(pressKey()).toBe(true)
    expect(handler).not.toHaveBeenCalled()
  })

  it('inTextField:false skips a key targeting an input, runs when it does not', () => {
    const { handler } = mountShortcut('Escape', { inTextField: false })
    const input = document.createElement('input')
    document.body.appendChild(input)

    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(handler).not.toHaveBeenCalled()

    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(handler).toHaveBeenCalledTimes(1)

    document.body.removeChild(input)
  })

  it('a listbox in the page (the repo picker as a window) does not block the handler', () => {
    const { handler } = mountShortcut('k', {}, () =>
      h('ul', { role: 'listbox' }, [h('li', { role: 'option' }, 'a')]),
    )

    pressKey()

    expect(handler).toHaveBeenCalledTimes(1)
  })
})
