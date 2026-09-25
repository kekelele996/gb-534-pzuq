import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AuditEvidenceDrawer from './AuditEvidenceDrawer.vue'

describe('AuditEvidenceDrawer', () => {
  it('renders metadata and readable before/after JSON for every entity type', () => {
    const wrapper = mount(AuditEvidenceDrawer, {
      props: {
        modelValue: true,
        event: {
          id: 9, request_id: 'req-534', actor_id: 4, actor_name: 'scientist', actor_role: 'process_scientist',
          entity_type: 'culture_recipe', entity_id: 5, action: 'transition',
          before_snapshot: '{"recipe_state":"draft"}', after_snapshot: '{"recipe_state":"validated"}',
          algorithm_version: '', duration_ms: 0, created_at: '2026-08-22T06:00:00Z',
        },
      },
      global: { stubs: { 'el-drawer': { template: '<div><slot /></div>' }, 'el-button': { template: '<button><slot /></button>' } } },
    })
    expect(wrapper.text()).toContain('culture_recipe #5')
    expect(wrapper.text()).toContain('req-534')
    expect(wrapper.text()).toContain('process_scientist')
    expect(wrapper.findAll('pre')[0].text()).toContain('"recipe_state": "draft"')
    expect(wrapper.findAll('pre')[1].text()).toContain('"recipe_state": "validated"')
  })
})
