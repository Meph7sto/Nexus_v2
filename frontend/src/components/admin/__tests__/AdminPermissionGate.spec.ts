import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AdminPermissionGate from '../AdminPermissionGate.vue'

const authStore = vi.hoisted(() => ({
  canAdmin: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

describe('AdminPermissionGate', () => {
  beforeEach(() => {
    authStore.canAdmin.mockReset()
  })

  it('fails closed and does not render a command without the required capability', () => {
    authStore.canAdmin.mockReturnValue(false)

    const wrapper = mount(AdminPermissionGate, {
      props: { resource: 'users', action: 'create' },
      slots: { default: '<button data-test="create-user">Create user</button>' },
    })

    expect(authStore.canAdmin).toHaveBeenCalledWith('users', 'create')
    expect(wrapper.find('[data-test="create-user"]').exists()).toBe(false)
  })

  it('renders a command when the required capability is granted', () => {
    authStore.canAdmin.mockReturnValue(true)

    const wrapper = mount(AdminPermissionGate, {
      props: { resource: 'groups', action: 'delete' },
      slots: { default: '<button data-test="delete-group">Delete group</button>' },
    })

    expect(authStore.canAdmin).toHaveBeenCalledWith('groups', 'delete')
    expect(wrapper.get('[data-test="delete-group"]').text()).toBe('Delete group')
  })
})
