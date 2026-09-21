import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import GraphCell from './GraphCell.vue'

describe('GraphCell', () => {
  it('FR-LOG-02 renders a dot at the lane, one line per edge, and an overflow marker', () => {
    const wrapper = mount(GraphCell, {
      props: {
        graph: {
          lane: 1,
          colour: 0,
          edges: [
            { lane: 0, kind: 'continue' },
            { lane: 1, kind: 'start' },
          ],
          overflow: 3,
        },
      },
    })

    const lines = wrapper.findAll('line')
    expect(lines).toHaveLength(2)

    const dots = wrapper.findAll('circle')
    expect(dots).toHaveLength(1)
    const dotX = dots[0]?.attributes('cx')
    expect(lines.some((line) => line.attributes('x1') === dotX)).toBe(true)

    expect(wrapper.text()).toContain('+3')
  })
})
