import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

import ChannelsView from '../ChannelsView.vue'
import AdminPermissionGate from '@/components/admin/AdminPermissionGate.vue'

const authStore = vi.hoisted(() => ({
  canAdmin: vi.fn(),
}))

const adminAPI = vi.hoisted(() => ({
  channels: {
    list: vi.fn(),
  },
  settings: {
    getWebSearchEmulationConfig: vi.fn(),
  },
  groups: {
    getAll: vi.fn(),
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('@/api/admin', () => ({ adminAPI }))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' }
const DataTableStub = { template: '<div><slot name="empty" /></div>' }

function mountView() {
  return shallowMount(ChannelsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        AdminPermissionGate,
      },
    },
  })
}

describe('ChannelsView permissions', () => {
  beforeEach(() => {
    authStore.canAdmin.mockReset()
    authStore.canAdmin.mockReturnValue(false)
    adminAPI.channels.list.mockResolvedValue({ items: [], total: 0 })
    adminAPI.settings.getWebSearchEmulationConfig.mockResolvedValue({ enabled: false, providers: [] })
    adminAPI.groups.getAll.mockResolvedValue([])
  })

  it('does not expose channel creation commands without channels:create', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((button) => button.text().includes('admin.channels.createChannel'))).toBe(false)
  })
})
