import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChangedFileList from './ChangedFileList.vue'

const files = [
  { path: 'a.txt', status: 'M' },
  { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
]

describe('ChangedFileList', () => {
  it('FR-CMP-02 clicking a file row emits select with that file', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('li')[1]!.trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([files[1]])
  })

  it('FR-CMP-02 marks the selected file with a selected class', () => {
    const wrapper = mount(ChangedFileList, { props: { files, selected: files[1]! } })

    const rows = wrapper.findAll('li')
    expect(rows[0]!.classes()).not.toContain('selected')
    expect(rows[1]!.classes()).toContain('selected')
  })

  it('FR-CMP-02 double-clicking a file row emits open with that file', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('li')[1]!.trigger('dblclick')

    expect(wrapper.emitted('open')?.[0]).toEqual([files[1]])
  })

  it('FR-ED-02 clicking a file row\'s "Open in editor" control emits open-editor with that file, not select', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(wrapper.emitted('open-editor')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })
})
