import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChangedFileTree from './ChangedFileTree.vue'

const a = { path: 'src/a.ts', status: 'M' }
const b = { path: 'src/b.ts', oldPath: 'old.ts', status: 'R' }
const files = [a, b]

// node finds the tree node labelled label; row is its clickable content.
const node = (wrapper: VueWrapper, label: string) =>
  wrapper.get(`[role="treeitem"][aria-label="${label}"]`)
const row = (wrapper: VueWrapper, label: string) =>
  node(wrapper, label).get('[data-pc-section="nodecontent"]')
const labels = (wrapper: VueWrapper) =>
  wrapper.findAll('[role="treeitem"]').map((n) => n.attributes('aria-label'))

describe('ChangedFileTree', () => {
  it('FR-LOG-16 starts with every folder unfolded; a click on a folder folds it, another unfolds it', async () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })
    expect(labels(wrapper)).toEqual(['src', 'a.ts', 'b.ts'])

    await row(wrapper, 'src').trigger('click')
    expect(labels(wrapper)).toEqual(['src'])

    await row(wrapper, 'src').trigger('click')
    expect(labels(wrapper)).toEqual(['src', 'a.ts', 'b.ts'])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('FR-LOG-16 Enter on a folder folds it', async () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })

    await node(wrapper, 'src').trigger('keydown', { key: 'Enter', code: 'Enter' })

    expect(labels(wrapper)).toEqual(['src'])
    expect(wrapper.emitted('open')).toBeUndefined()
  })

  it('FR-LOG-16 a click on a file selects it; the selected file shows selected', async () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })

    await row(wrapper, 'b.ts').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[b]])

    await wrapper.setProps({ selected: b })
    expect(row(wrapper, 'b.ts').attributes('data-p-selected')).toBe('true')
    expect(row(wrapper, 'a.ts').attributes('data-p-selected')).toBe('false')
  })

  it('FR-LOG-16 a double-click or Enter on a file opens it', async () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })

    await row(wrapper, 'b.ts').trigger('dblclick')
    await node(wrapper, 'b.ts').trigger('keydown', { key: 'Enter', code: 'Enter' })

    expect(wrapper.emitted('open')).toEqual([[b], [b]])
  })

  it('FR-ED-02 a file\'s "Open in editor", which a folder has not, emits open-editor with that file, not select', async () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })

    const button = node(wrapper, 'b.ts').get('[data-test="open-editor"]')
    expect(button.attributes('aria-label')).toBe('Open in editor')
    await button.trigger('click')

    expect(wrapper.emitted('open-editor')).toEqual([[b]])
    expect(wrapper.emitted('select')).toBeUndefined()
    expect(row(wrapper, 'src').find('[data-test="open-editor"]').exists()).toBe(false)
  })

  it("FR-UX-13 a file shows its status as an icon with the status as tooltip; a rename names its old path; the name's title is the full path", () => {
    const wrapper = mount(ChangedFileTree, { props: { files } })

    const statuses = wrapper.findAll('.status')
    expect(statuses.map((s) => s.attributes('title'))).toEqual(['Modified', 'Renamed'])
    expect(statuses[0]!.get('i').classes()).toContain('oi-file-edit')
    const name = row(wrapper, 'b.ts').get('.name')
    expect(name.text()).toBe('old.ts → b.ts')
    expect(name.attributes('title')).toBe('src/b.ts')
  })
})
