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
    pricing_usage_amount: 600,
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
        emits: ['close'],
        template: '<div v-if="show"><button data-test="close-dialog" @click="$emit(\'close\')">close</button><slot /><slot name="footer" /></div>',
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

  it.each([
    { pricingUsage: 1.005, displayUsage: 1.01, expected: '$0.50' },
    { pricingUsage: 2.01, displayUsage: 2.01, expected: '$1.01' },
    { pricingUsage: 0.29, displayUsage: 0.29, expected: '$0.15' },
  ])('previews $pricingUsage × 0.5 with decimal half-up rounding', async ({ pricingUsage, displayUsage, expected }) => {
    list.mockResolvedValue({
      ...response(),
      items: [{ ...response().items[0], usage_amount: displayUsage, pricing_usage_amount: pricingUsage }],
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="edit-multiplier-7"]').trigger('click')
    await wrapper.find('[data-test="multiplier-preset-0.5"]').trigger('click')

    const amounts = wrapper.findAll('.amount-comparison strong')
    expect(amounts[0].text()).toBe(`$${displayUsage.toFixed(2)}`)
    expect(amounts[1].text()).toBe(expected)
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

  it('refreshes the latest allowed payment time whenever the dialog opens', async () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date('2026-08-22T12:00:00'))
      const wrapper = mountView()
      await flushPromises()

      await wrapper.find('[data-test="payments-7"]').trigger('click')
      await flushPromises()
      const initialTime = wrapper.find<HTMLInputElement>('[data-test="payment-time"]')
      expect(initialTime.attributes('max')).toBe(initialTime.element.value)

      vi.setSystemTime(new Date('2026-08-22T14:00:00'))
      await wrapper.find('[data-test="payments-7"]').trigger('click')
      await flushPromises()
      const refreshedTime = wrapper.find<HTMLInputElement>('[data-test="payment-time"]')
      expect(refreshedTime.element.value).toBe('2026-08-22T14:00')
      expect(refreshedTime.attributes('max')).toBe(refreshedTime.element.value)

      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('ignores an older payment response after another user is opened', async () => {
    let resolveFirst!: (value: unknown[]) => void
    let resolveSecond!: (value: unknown[]) => void
    list.mockResolvedValue({
      ...response(),
      items: [
        response().items[0],
        { ...response().items[0], user_id: 8, email: 'second@example.test' },
      ],
      total: 2,
    })
    listPayments.mockImplementation((_month: string, userID: number) => new Promise((resolve) => {
      if (userID === 7) resolveFirst = resolve
      else resolveSecond = resolve
    }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await wrapper.find('[data-test="payments-8"]').trigger('click')
    resolveSecond([{ id: 80, user_id: 8, amount: 80, note: 'second payment', paid_at: '2026-08-01T12:00:00Z' }])
    await flushPromises()
    resolveFirst([{ id: 70, user_id: 7, amount: 70, note: 'stale first payment', paid_at: '2026-08-01T12:00:00Z' }])
    await flushPromises()

    expect(wrapper.text()).toContain('second payment')
    expect(wrapper.text()).not.toContain('stale first payment')
  })

  it('rebinds the active row before resetting the form after a partial payment', async () => {
    const updated = response()
    updated.items[0] = {
      ...updated.items[0],
      paid_amount: 200,
      outstanding_amount: 400,
      status: 'partial',
      payment_count: 1,
    }
    updated.summary = {
      ...updated.summary,
      paid_amount: 200,
      outstanding_amount: 400,
      unpaid_count: 0,
      partial_count: 1,
    }
    list.mockResolvedValueOnce(response())
      .mockResolvedValueOnce({ ...updated, items: [], total: 0 })
      .mockResolvedValue(updated)
    listPayments.mockResolvedValueOnce([]).mockResolvedValueOnce([{
      id: 1,
      user_id: 7,
      amount: 200,
      note: 'partial payment',
      paid_at: '2026-08-01T12:00:00Z',
    }])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.find<HTMLInputElement>('[data-test="payment-amount"]').setValue(200)
    await wrapper.find('form.payment-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.mb-3 .text-sm').text()).toBe('$200.00')
    expect(wrapper.find<HTMLInputElement>('[data-test="payment-amount"]').element.value).toBe('400')
    expect(list).toHaveBeenNthCalledWith(3, expect.objectContaining({ month: '2026-07', user_id: 7, status: '' }))
  })

  it('refreshes the ledger after a payment succeeds even if the dialog was closed in flight', async () => {
    let resolveCreate!: (value: { id: number }) => void
    createPayment.mockImplementation(() => new Promise((resolve) => { resolveCreate = resolve }))
    const updated = response()
    updated.items[0] = {
      ...updated.items[0],
      paid_amount: 200,
      outstanding_amount: 400,
      status: 'partial',
      payment_count: 1,
    }
    list.mockResolvedValueOnce(response()).mockResolvedValue(updated)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.find<HTMLInputElement>('[data-test="payment-amount"]').setValue(200)
    await wrapper.find('form.payment-form').trigger('submit')
    await wrapper.find('[data-test="close-dialog"]').trigger('click')
    resolveCreate({ id: 1 })
    await flushPromises()

    expect(list).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('$200.00')
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
