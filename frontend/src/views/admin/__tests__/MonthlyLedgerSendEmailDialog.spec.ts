import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthlyLedgerSendEmailDialog from '../MonthlyLedgerSendEmailDialog.vue'
import type { MonthlyLedgerRow } from '@/api/admin/monthlyLedger'

const { listEmailPreferences, getEmailQuota, setEmailDailyLimit, sendManualEmail, canAdmin, showSuccess } = vi.hoisted(() => ({
  listEmailPreferences: vi.fn(), getEmailQuota: vi.fn(), setEmailDailyLimit: vi.fn(), sendManualEmail: vi.fn(), canAdmin: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { monthlyLedger: { listEmailPreferences, getEmailQuota, setEmailDailyLimit, sendManualEmail } } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ canAdmin }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

async function open() {
  const wrapper = mount(MonthlyLedgerSendEmailDialog, {
    props: { show: true, initialMonth: '2026-09', recipient: { user_id: 7, email: 'user@example.test', username: 'User', outstanding_amount: 343.68 } as MonthlyLedgerRow },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }, Icon: true } },
  })
  await flushPromises()
  return wrapper
}

describe('manual monthly ledger email', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    canAdmin.mockReturnValue(true)
    listEmailPreferences.mockResolvedValue({ items: [{ user_id: 7, email: 'user@example.test', username: 'User', enabled: false }], total: 1 })
    getEmailQuota.mockResolvedValue({ date: '2026-09-28', daily_limit: 5, used: 0 })
    sendManualEmail.mockResolvedValue({ sent: true })
    setEmailDailyLimit.mockResolvedValue({ daily_limit: 8 })
  })

  it('prefills the row amount and sends to the row user for the page month', async () => {
    const wrapper = await open()
    expect(wrapper.get<HTMLInputElement>('#email-amount').element.value).toBe('343.68')
    expect(wrapper.get('#email-billing-month').attributes('readonly')).toBeDefined()
    expect(wrapper.find('#email-user-search').exists()).toBe(false)
    expect(listEmailPreferences).not.toHaveBeenCalled()
    await wrapper.get('#email-amount').setValue('42.35')
    expect(wrapper.get('[data-test="email-summary"]').text()).toContain('42.35 USD')
    await wrapper.get('[data-test="send-ledger-email"]').trigger('click')
    await flushPromises()
    expect(sendManualEmail).toHaveBeenCalledWith({ month: '2026-09', user_id: 7, amount: 42.35, request_id: expect.any(String) })
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it.each(['0', '-2', '1.001'])('rejects invalid amount %s', async value => {
    const wrapper = await open()
    await wrapper.get('#email-amount').setValue(value)
    expect(wrapper.get('[data-test="send-ledger-email"]').attributes('disabled')).toBeDefined()
  })

  it('blocks exhausted quota and permits updating the shared daily cap', async () => {
    getEmailQuota.mockResolvedValue({ date: '2026-09-28', daily_limit: 5, used: 5 })
    const wrapper = await open()
    await wrapper.get('#email-amount').setValue('42')
    expect(wrapper.get('[data-test="send-ledger-email"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#email-daily-limit').setValue(8)
    getEmailQuota.mockResolvedValue({ date: '2026-09-28', daily_limit: 8, used: 5 })
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(setEmailDailyLimit).toHaveBeenCalledWith(8)
    expect(wrapper.get('[data-test="send-ledger-email"]').attributes('disabled')).toBeUndefined()
  })

  it('retains the draft on failure and blocks duplicate clicks while sending', async () => {
    let reject!: (reason: unknown) => void
    sendManualEmail.mockReturnValue(new Promise((_, fail) => { reject = fail }))
    const wrapper = await open()
    await wrapper.get('#email-amount').setValue('42')
    await wrapper.get('[data-test="send-ledger-email"]').trigger('click')
    await wrapper.get('[data-test="send-ledger-email"]').trigger('click')
    expect(sendManualEmail).toHaveBeenCalledTimes(1)
    reject({ reason: 'MONTHLY_LEDGER_EMAIL_DAILY_LIMIT' })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('limitReached')
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.get<HTMLInputElement>('#email-amount').element.value).toBe('42')
  })

  it('enforces separate create and update permissions', async () => {
    canAdmin.mockReturnValue(false)
    const wrapper = await open()
    expect(wrapper.find('#email-daily-limit').exists()).toBe(false)
    await wrapper.get('#email-amount').setValue('42')
    expect(wrapper.get('[data-test="send-ledger-email"]').attributes('disabled')).toBeDefined()
  })
})
