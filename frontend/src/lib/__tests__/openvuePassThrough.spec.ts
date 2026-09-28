import { h } from 'vue'
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'

describe('openvuePassThrough', () => {
  it('FR-UX-20 a cancelable mousedown on a splitter gutter is default-prevented and resizing still works', () => {
    const wrapper = mount(Splitter, {
      slots: {
        default: () => [
          h(SplitterPanel, null, { default: () => 'left' }),
          h(SplitterPanel, null, { default: () => 'right' }),
        ],
      },
    })
    const root = wrapper.element
    const gutter = wrapper.get('[data-p-gutter-resizing]').element

    // happy-dom's MouseEvent always reports pageX 0; onResizeStart falls
    // back to a touch event's pageX when it is falsy, so a real value is
    // needed for the mouse-down to run instead of throwing.
    const mousedown = new MouseEvent('mousedown', { bubbles: true, cancelable: true })
    Object.defineProperty(mousedown, 'pageX', { value: 10 })
    gutter.dispatchEvent(mousedown)

    expect(mousedown.defaultPrevented).toBe(true)
    expect(root.getAttribute('data-p-resizing')).toBe('true')

    document.dispatchEvent(new MouseEvent('mousemove', { bubbles: true }))
    document.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }))

    expect(root.getAttribute('data-p-resizing')).toBe('false')
  })
})
