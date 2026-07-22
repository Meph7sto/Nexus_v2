import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpenAIQuotaSummaryView from '../OpenAIQuotaSummaryView.vue'
import { accountsAPI, groupsAPI } from '@/api/admin'

const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

const authStore = vi.hoisted(() => ({
  canAdmin: vi.fn(() => true)
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn()
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${JSON.stringify(params)}` : key
  })
}))

vi.mock('@/api/admin', () => ({
  accountsAPI: {
    getOpenAIQuotaSummary: vi.fn()
  },
  groupsAPI: {
    getAllIncludingInactive: vi.fn()
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div><slot /></div>' }
}))

const response = {
  projection_at: '2026-07-20T15:00:00Z',
  generated_at: '2026-07-20T14:00:00Z',
  groups: [
    {
      group_id: 12,
      group_name: 'OpenAI Production Accounts With A Long Name',
      ungrouped: false,
      rows: [
        {
          account_type: 'plus',
          included_count: 10,
          error_count: 1,
          inactive_count: 2,
          other_excluded_count: 3,
          missing_5h_snapshot_count: 4,
          missing_7d_snapshot_count: 5,
          avg_5h_remaining_percent: 90,
          avg_7d_remaining_percent: 84.5,
          earliest_5h_recovery: {
            account_id: 42,
            account_name: 'openai-01',
            account_type: 'plus',
            reset_at: '2026-07-20T16:30:00Z',
            remaining_before_percent: 90,
            remaining_after_percent: 100
          },
          earliest_7d_recovery: null
        }
      ]
    }
  ]
}

describe('OpenAIQuotaSummaryView', () => {
  beforeEach(() => {
    authStore.canAdmin.mockReset()
    authStore.canAdmin.mockReturnValue(true)
    appStore.showError.mockReset()
    vi.mocked(accountsAPI.getOpenAIQuotaSummary).mockReset()
    vi.mocked(accountsAPI.getOpenAIQuotaSummary).mockResolvedValue(response)
    vi.mocked(groupsAPI.getAllIncludingInactive).mockReset()
    vi.mocked(groupsAPI.getAllIncludingInactive).mockResolvedValue([
      { id: 12, name: 'OpenAI Production Accounts With A Long Name', platform: 'openai' },
      { id: 99, name: 'Empty OpenAI Group', platform: 'openai' },
      { id: 100, name: 'Other Platform Group', platform: 'anthropic' },
    ] as never)
  })

  it('loads grouped rows with missing snapshots, exclusions, recovery, and long group names', async () => {
    const wrapper = mount(OpenAIQuotaSummaryView)
    await flushPromises()

    expect(authStore.canAdmin).toHaveBeenCalledWith('accounts', 'view')
    expect(accountsAPI.getOpenAIQuotaSummary).toHaveBeenCalledWith({})
    expect(groupsAPI.getAllIncludingInactive).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('OpenAI Production Accounts With A Long Name')
    expect(wrapper.text()).toContain('Empty OpenAI Group')
    expect(wrapper.text()).not.toContain('Other Platform Group')
    expect(wrapper.text()).toContain('90.0%')
    expect(wrapper.text()).toContain('84.5%')
    expect(wrapper.text()).toContain('90.0% -> 100.0%')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('admin.openAIQuotaSummary.partialSnapshot')
    expect(wrapper.text()).toContain('admin.openAIQuotaSummary.table.errors')
    expect(wrapper.text()).toContain('admin.openAIQuotaSummary.table.inactive')
    expect(wrapper.text()).toContain('admin.openAIQuotaSummary.table.other')
    expect(wrapper.text()).not.toContain('openai-01')
  })

  it('sends future projection, group, and plan type filters on refresh', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T14:00:00Z'))
    try {
      const wrapper = mount(OpenAIQuotaSummaryView)
      await flushPromises()

      await wrapper.get('[data-test="projection-mode-hours"]').trigger('click')
      await wrapper.get('[data-test="projection-amount"]').setValue('2')
      await wrapper.get('[data-test="group-filter"]').setValue('12')
      await wrapper.get('[data-test="type-filter"]').setValue('plus')
      await wrapper.get('[data-test="refresh"]').trigger('click')
      await flushPromises()

      expect(accountsAPI.getOpenAIQuotaSummary).toHaveBeenLastCalledWith({
        projection_at: '2026-07-20T16:00:00.000Z',
        group: '12',
        type: 'plus'
      })
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not request data without accounts:view permission', async () => {
    authStore.canAdmin.mockReturnValue(false)

    const wrapper = mount(OpenAIQuotaSummaryView)
    await flushPromises()

    expect(accountsAPI.getOpenAIQuotaSummary).not.toHaveBeenCalled()
    expect(groupsAPI.getAllIncludingInactive).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.openAIQuotaSummary.noPermission')
  })

  it('keeps loading distinct from an empty result and renders request errors', async () => {
    vi.mocked(accountsAPI.getOpenAIQuotaSummary).mockReturnValueOnce(new Promise(() => {}))
    const loadingWrapper = mount(OpenAIQuotaSummaryView)

    expect(loadingWrapper.text()).toContain('common.loading')
    expect(loadingWrapper.text()).not.toContain('common.noData')

    vi.mocked(accountsAPI.getOpenAIQuotaSummary).mockResolvedValueOnce({
      projection_at: '2026-07-20T15:00:00Z',
      generated_at: '2026-07-20T14:00:00Z',
      groups: []
    })
    const emptyWrapper = mount(OpenAIQuotaSummaryView)
    await flushPromises()

    expect(emptyWrapper.text()).toContain('common.noData')

    vi.mocked(accountsAPI.getOpenAIQuotaSummary).mockRejectedValueOnce(new Error('quota service unavailable'))
    const errorWrapper = mount(OpenAIQuotaSummaryView)
    await flushPromises()

    expect(appStore.showError).toHaveBeenCalledWith('quota service unavailable')
    expect(errorWrapper.get('[data-test="summary-error"]').text()).toContain('quota service unavailable')
  })

  it('keeps numeric column headers aligned with their values', () => {
    expect(styleSource).toMatch(/\.table th\.text-right\s*\{\s*text-align:\s*right;/)
  })
})
