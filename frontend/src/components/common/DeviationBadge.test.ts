import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DeviationBadge from './DeviationBadge.vue'

describe('DeviationBadge', () => {
  it.each([
    ['normal', '正常'],
    ['watch', '关注'],
    ['major', '重大'],
    ['critical', '严重'],
  ])('maps %s to %s', (level, label) => {
    const wrapper = mount(DeviationBadge, { props: { level } })
    expect(wrapper.text()).toBe(label)
    expect(wrapper.classes()).toContain(level)
  })
})
