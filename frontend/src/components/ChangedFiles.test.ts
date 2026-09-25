import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import ChangedFiles from './ChangedFiles.vue'

const files = [
  { path: 'src/a.ts', status: 'M' },
  { path: 'src/b.ts', status: 'A' },
]

describe('ChangedFiles', () => {
  beforeEach(() => localStorage.clear())

  it('FR-LOG-16 counts the files above them and starts as a flat list', () => {
    const wrapper = mount(ChangedFiles, { props: { files, selected: null } })

    expect(wrapper.get('h3').text()).toBe('2 changed files')
    expect(wrapper.findAll('.files li')).toHaveLength(2)
    expect(wrapper.find('[role="tree"]').exists()).toBe(false)
  })

  it('FR-LOG-16 the toggle switches to the folder tree and back, pressed while the tree shows', async () => {
    const wrapper = mount(ChangedFiles, { props: { files, selected: null } })
    const toggle = () => wrapper.get('[data-test="file-tree-toggle"]')
    expect(toggle().attributes('aria-label')).toBe('Show as folder tree')
    expect(toggle().attributes('aria-pressed')).toBe('false')

    await toggle().trigger('click')
    expect(wrapper.find('[role="tree"]').exists()).toBe(true)
    expect(wrapper.find('.files').exists()).toBe(false)
    expect(toggle().attributes('aria-label')).toBe('Show as flat list')
    expect(toggle().attributes('aria-pressed')).toBe('true')

    await toggle().trigger('click')
    expect(wrapper.findAll('.files li')).toHaveLength(2)
  })

  it('FR-LOG-16 the choice is remembered: the next mount starts as a tree', async () => {
    await mount(ChangedFiles, { props: { files, selected: null } })
      .get('[data-test="file-tree-toggle"]')
      .trigger('click')

    const next = mount(ChangedFiles, { props: { files, selected: null } })

    expect(next.find('[role="tree"]').exists()).toBe(true)
  })

  it('FR-LOG-16 both modes pass on select, open and open-editor', async () => {
    const wrapper = mount(ChangedFiles, { props: { files, selected: null } })
    await wrapper.findAll('.files li')[1]!.trigger('dblclick')
    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')
    const tree = wrapper.get('[role="treeitem"][aria-label="a.ts"]')
    await tree.get('[data-pc-section="nodecontent"]').trigger('click')
    await tree.get('[data-test="open-editor"]').trigger('click')

    expect(wrapper.emitted('open')).toEqual([[files[1]]])
    expect(wrapper.emitted('select')).toEqual([[files[0]]])
    expect(wrapper.emitted('open-editor')).toEqual([[files[0]]])
  })
})
