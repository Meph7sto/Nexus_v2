import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AdminPermissionMatrix from '../AdminPermissionMatrix.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const definitions = [
  {
    resource: 'users' as const,
    label: 'Users',
    actions: ['view', 'update'] as const,
    super_admin_only: false,
  },
  {
    resource: 'settings' as const,
    label: 'Settings',
    actions: ['view', 'update'] as const,
    super_admin_only: true,
  },
]

describe('AdminPermissionMatrix', () => {
  it('adds view when a non-view action is enabled and clears a row when view is removed', async () => {
    const wrapper = mount(AdminPermissionMatrix, {
      props: {
        modelValue: [],
        definitions,
      },
    })

    expect(wrapper.text()).toContain('Users')
    expect(wrapper.text()).not.toContain('Settings')

    const actions = wrapper.findAll('input[type="checkbox"]')
    await actions[1].setValue(true)

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([
      { resource: 'users', actions: ['view', 'update'] },
    ])

    await wrapper.setProps({ modelValue: [{ resource: 'users', actions: ['view', 'update'] }] })
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(false)

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([])
  })
})
