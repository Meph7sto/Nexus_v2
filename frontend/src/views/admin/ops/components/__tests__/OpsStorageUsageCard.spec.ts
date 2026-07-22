import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import OpsStorageUsageCard from '../OpsStorageUsageCard.vue'

const { mockGetStorageUsage, permissionState } = vi.hoisted(() => ({
  mockGetStorageUsage: vi.fn(),
  permissionState: { canView: true },
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getStorageUsage: (...args: unknown[]) => mockGetStorageUsage(...args),
  },
}))

vi.mock('@/composables/useAdminPermissionGate', () => ({
  useAdminPermissionGate: () => ({
    can: () => ({
      __v_isRef: true,
      get value() {
        return permissionState.canView
      },
    }),
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const storageResponse = {
  generated_at: '2026-07-20T12:00:00Z',
  total_used_bytes: 1102195982336,
  items: [
    {
      key: 'postgres_db',
      label: 'PostgreSQL database',
      kind: 'database',
      source: 'postgres',
      used_bytes: 1073741824,
      status: 'ok' as const,
    },
    {
      key: 'app_data',
      label: 'Application data directory with a deliberately long label',
      kind: 'directory',
      source: 'default',
      used_bytes: 1610612736,
      status: 'ok' as const,
    },
    {
      key: 'backup',
      label: 'Backup snapshot',
      kind: 'directory',
      source: 'config',
      used_bytes: 1099511627776,
      status: 'ok' as const,
    },
    {
      key: 'archive',
      label: 'Archive volume',
      kind: 'directory',
      source: 'config',
      status: 'unavailable' as const,
      error: 'Configured archive volume is unavailable and this intentionally long diagnostic must wrap safely.',
    },
    {
      key: 'optional',
      label: 'Optional volume',
      kind: 'directory',
      source: 'config',
      status: 'unconfigured' as const,
    },
  ],
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function mountCard(refreshKey = 0) {
  return mount(OpsStorageUsageCard, {
    props: { refreshKey },
    global: {
      stubs: {
        HelpTooltip: true,
      },
    },
  })
}

describe('OpsStorageUsageCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    permissionState.canView = true
  })

  it('shows an initial loading state, cancels a stale refresh, and renders the newest response', async () => {
    const first = createDeferred<typeof storageResponse>()
    const second = createDeferred<typeof storageResponse>()
    mockGetStorageUsage.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)

    const wrapper = mountCard()
    await nextTick()

    expect(wrapper.get('[data-testid="storage-loading"]').attributes('aria-live')).toBe('polite')
    const firstSignal = mockGetStorageUsage.mock.calls[0][0].signal as AbortSignal

    await wrapper.setProps({ refreshKey: 1 })
    await nextTick()

    expect(firstSignal.aborted).toBe(true)
    first.reject({ code: 'ERR_CANCELED' })
    second.resolve(storageResponse)
    await flushPromises()

    expect(wrapper.text()).toContain('PostgreSQL database')
    expect(wrapper.text()).toContain('1 TB')
    expect(wrapper.find('[data-testid="storage-loading"]').exists()).toBe(false)
  })

  it('keeps the previous value visible while a refresh is in flight', async () => {
    const refresh = createDeferred<typeof storageResponse>()
    mockGetStorageUsage.mockResolvedValueOnce(storageResponse).mockReturnValueOnce(refresh.promise)

    const wrapper = mountCard()
    await flushPromises()
    await wrapper.setProps({ refreshKey: 1 })
    await nextTick()

    expect(wrapper.text()).toContain('1 TB')
    expect(wrapper.find('[data-testid="storage-refreshing"]').exists()).toBe(true)

    refresh.resolve({ ...storageResponse, total_used_bytes: 1073741824 })
    await flushPromises()
    expect(wrapper.text()).toContain('1 GB')
  })

  it('renders successful, partial, unconfigured, long-text, and large-byte states without exposing a raw path', async () => {
    mockGetStorageUsage.mockResolvedValue(storageResponse)

    const wrapper = mountCard()
    await flushPromises()

    expect(wrapper.text()).toContain('1.5 GB')
    expect(wrapper.text()).toContain('1 TB')
    expect(wrapper.text()).toContain('admin.ops.storage.status.unavailable')
    expect(wrapper.text()).toContain('admin.ops.storage.status.unconfigured')
    expect(wrapper.text()).toContain('Configured archive volume is unavailable')
    expect(wrapper.get('[data-testid="storage-item-app_data"]').attributes('title')).toContain(
      'Application data directory with a deliberately long label'
    )
  })

  it('renders empty and overall-error states', async () => {
    mockGetStorageUsage.mockResolvedValueOnce({
      generated_at: '2026-07-20T12:00:00Z',
      total_used_bytes: 0,
      items: [],
    })

    const emptyWrapper = mountCard()
    await flushPromises()
    expect(emptyWrapper.text()).toContain('admin.ops.storage.empty')

    mockGetStorageUsage.mockRejectedValueOnce(new Error('Storage collection failed'))
    const errorWrapper = mountCard(1)
    await flushPromises()
    expect(errorWrapper.get('[role="alert"]').text()).toContain('Storage collection failed')
  })

  it('does not render or issue a request without ops:view', async () => {
    permissionState.canView = false

    const wrapper = mountCard()
    await flushPromises()

    expect(mockGetStorageUsage).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="storage-card"]').exists()).toBe(false)
  })
})
