import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthlyLedgerEmailDialog from '../MonthlyLedgerEmailDialog.vue'

const { listEmailPreferences, setEmailPreference, canAdmin, showError, showSuccess } = vi.hoisted(() => ({
  listEmailPreferences: vi.fn(), setEmailPreference: vi.fn(), canAdmin: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { monthlyLedger: { listEmailPreferences, setEmailPreference } } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ canAdmin }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const user = () => ({ user_id: 7, email: 'user@example.test', username: 'User', enabled: false, effective_month: '' })
async function open() {
  const wrapper = mount(MonthlyLedgerEmailDialog, {
    props: { show: false },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('monthly ledger email preferences', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    canAdmin.mockReturnValue(true)
    listEmailPreferences.mockResolvedValue({ items: [user()], total: 1 })
    setEmailPreference.mockResolvedValue({ enabled: true })
  })

  it('loads users with default-off preferences and persists opt-in', async () => {
    const wrapper = await open()
    expect(listEmailPreferences).toHaveBeenCalledWith({ q: '', page: 1, page_size: 20 })
    const checkbox = wrapper.get<HTMLInputElement>('[data-test="email-user-7"]')
    expect(checkbox.element.checked).toBe(false)
    listEmailPreferences.mockResolvedValue({ items: [{ ...user(), enabled: true, effective_month: '2026-09' }], total: 1 })
    await checkbox.setValue(true)
    await flushPromises()
    expect(setEmailPreference).toHaveBeenCalledWith(7, true)
    expect(wrapper.get<HTMLInputElement>('[data-test="email-user-7"]').element.checked).toBe(true)
  })

  it('retains the saved preference after a failed update', async () => {
    setEmailPreference.mockRejectedValue(new Error('failed'))
    const wrapper = await open()
    await wrapper.get('[data-test="email-user-7"]').setValue(true)
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('[data-test="email-user-7"]').element.checked).toBe(false)
    expect(showError).toHaveBeenCalled()
  })

  it('prevents view-only administrators from updating preferences', async () => {
    canAdmin.mockReturnValue(false)
    const wrapper = await open()
    expect(wrapper.get('[data-test="email-user-7"]').attributes('disabled')).toBeDefined()
    expect(setEmailPreference).not.toHaveBeenCalled()
  })

  it('searches users without depending on the selected billing month', async () => {
    const wrapper = await open()
    await wrapper.get('input:not([type="checkbox"])').setValue('customer')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(listEmailPreferences).toHaveBeenLastCalledWith({ q: 'customer', page: 1, page_size: 20 })
  })
})
