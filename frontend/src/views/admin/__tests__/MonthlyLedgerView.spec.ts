import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import MonthlyLedgerView from '../MonthlyLedgerView.vue'

const { list, listPayments, setMultiplier, createPayment, updatePayment, deletePayment, showError, showSuccess, canAdmin } = vi.hoisted(() => ({
  list: vi.fn(),
  listPayments: vi.fn(),
  setMultiplier: vi.fn(),
  createPayment: vi.fn(),
  updatePayment: vi.fn(),
  deletePayment: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  canAdmin: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    monthlyLedger: { list, listPayments, setMultiplier, createPayment, updatePayment, deletePayment },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ canAdmin }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }
})

const response = (canRecordPayments = true) => ({
  items: [{
    user_id: 7,
    email: 'customer@example.test',
    username: 'Customer',
    deleted: false,
    usage_amount: 600,
    multiplier: 1,
    receivable_amount: 600,
    paid_amount: 0,
    outstanding_amount: 600,
    overpaid_amount: 0,
    status: 'unpaid',
    payment_count: 0,
    last_paid_at: null,
  }],
  summary: {
    usage_amount: 600,
    receivable_amount: 600,
    paid_amount: 0,
    outstanding_amount: 600,
    overpaid_amount: 0,
    user_count: 1,
    unpaid_count: 1,
    partial_count: 0,
    settled_count: 0,
    overpaid_count: 0,
    waived_count: 0,
  },
  total: 1,
  page: 1,
  page_size: 20,
  pages: 1,
  month: '2026-07',
  current_month: '2026-08',
  can_record_payments: canRecordPayments,
})

const mountView = () => mount(MonthlyLedgerView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Pagination: true,
      Icon: true,
      BaseDialog: {
        props: ['show'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>',
      },
      ConfirmDialog: true,
    },
  },
})

describe('MonthlyLedgerView', () => {
  beforeEach(() => {
    list.mockReset().mockResolvedValue(response())
    listPayments.mockReset().mockResolvedValue([])
    setMultiplier.mockReset().mockResolvedValue({ multiplier: 0.5 })
    createPayment.mockReset().mockResolvedValue({ id: 1 })
    updatePayment.mockReset().mockResolvedValue({ id: 1 })
    deletePayment.mockReset().mockResolvedValue({ id: 1 })
    showError.mockReset()
    showSuccess.mockReset()
    canAdmin.mockReset().mockReturnValue(true)
  })

  it('loads the backend default month and renders the billing amounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }))
    expect(wrapper.text()).toContain('customer@example.test')
    expect(wrapper.text()).toContain('$600.00')
    expect(wrapper.find<HTMLInputElement>('[data-test="ledger-month"]').element.value).toBe('2026-07')
    expect(wrapper.find('[data-test="summary-receivable"]').text()).toContain('$600.00')
    expect(wrapper.find('[data-test="summary-paid"]').text()).toContain('$0.00')
  })

  it('sets a per-user monthly multiplier from a quick option', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="edit-multiplier-7"]').trigger('click')
    await wrapper.find('[data-test="multiplier-preset-0.5"]').trigger('click')
    await wrapper.find('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()

    expect(setMultiplier).toHaveBeenCalledWith('2026-07', 7, 0.5)
  })

  it('records a payment against the selected completed month', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    const amount = wrapper.find<HTMLInputElement>('[data-test="payment-amount"]')
    expect(amount.element.value).toBe('600')
    await wrapper.find('form.payment-form').trigger('submit')
    await flushPromises()

    expect(createPayment).toHaveBeenCalledWith('2026-07', 7, expect.objectContaining({ amount: 600 }))
  })

  it('disables adding payments for the current open month', async () => {
    list.mockResolvedValue(response(false))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="add-payment"]').attributes('disabled')).toBeDefined()
  })

  it('lets an update-only administrator edit an existing payment', async () => {
    canAdmin.mockImplementation((_resource: string, action: string) => action === 'view' || action === 'update')
    listPayments.mockResolvedValue([{
      id: 9,
      user_id: 7,
      billing_month: '2026-07',
      amount: 125,
      paid_at: '2026-08-01T12:00:00Z',
      note: 'first payment',
      created_by: 2,
      created_by_email: 'admin@example.test',
      updated_by: 2,
      updated_by_email: 'admin@example.test',
      created_at: '2026-08-01T12:00:00Z',
      updated_at: '2026-08-01T12:00:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="add-payment"]').exists()).toBe(false)

    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-test="edit-payment-9"]').trigger('click')
    await wrapper.find('form.payment-form').trigger('submit')
    await flushPromises()

    expect(updatePayment).toHaveBeenCalledWith(9, expect.objectContaining({ amount: 125 }))
    expect(createPayment).not.toHaveBeenCalled()
  })

  it('validates the payment time before creating a payment', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.find<HTMLInputElement>('[data-test="payment-time"]').setValue('')
    await wrapper.find('form.payment-form').trigger('submit')

    expect(createPayment).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.monthlyLedger.payments.invalidPaidAt')
  })
})
