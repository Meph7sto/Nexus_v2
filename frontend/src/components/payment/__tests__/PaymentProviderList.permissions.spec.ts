import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PaymentProviderList from '../PaymentProviderList.vue'
import AdminPermissionGate from '@/components/admin/AdminPermissionGate.vue'

const authStore = vi.hoisted(() => ({
  canAdmin: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('PaymentProviderList permissions', () => {
  beforeEach(() => {
    authStore.canAdmin.mockReset()
    authStore.canAdmin.mockReturnValue(false)
  })

  it('does not expose provider creation without payment_providers:create', () => {
    const wrapper = mount(PaymentProviderList, {
      props: {
        providers: [],
        loading: false,
        canCreate: true,
        enabledPaymentTypes: ['alipay'],
        allPaymentTypes: [{ value: 'alipay', label: 'Alipay' }],
        redirectLabel: 'Redirect',
      },
      global: {
        stubs: {
          AdminPermissionGate,
        },
      },
    })

    expect(wrapper.findAll('button').some((button) => button.text().includes('admin.settings.payment.createProvider'))).toBe(false)
  })
})
