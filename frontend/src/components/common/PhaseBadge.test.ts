import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PhaseBadge from './PhaseBadge.vue'

describe('PhaseBadge', () => {
  it('renders the shared fermentation phase label and class', () => {
    const wrapper = mount(PhaseBadge, { props: { phase: 'production' } })
    expect(wrapper.text()).toBe('产物期')
    expect(wrapper.classes()).toContain('phase-production')
  })

  it('keeps an unknown backend value visible for diagnosis', () => {
    expect(mount(PhaseBadge, { props: { phase: 'unexpected' } }).text()).toBe('unexpected')
  })
})
