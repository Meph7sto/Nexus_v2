import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import MonthlyLedgerView from '../MonthlyLedgerView.vue'
import MonthlyLedgerSendEmailDialog from '../MonthlyLedgerSendEmailDialog.vue'
import MonthlyLedgerOverview from '../MonthlyLedgerOverview.vue'
import MonthlyLedgerIncome from '../MonthlyLedgerIncome.vue'

const { list, listPayments, setSettlement, setMultiplier, setMultipliers, createPayment, updatePayment, deletePayment, showError, showSuccess, canAdmin } = vi.hoisted(() => ({
  setSettlement: vi.fn(),
  list: vi.fn(),
  listPayments: vi.fn(),
  setMultiplier: vi.fn(),
  setMultipliers: vi.fn(),
  createPayment: vi.fn(),
  updatePayment: vi.fn(),
  deletePayment: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  canAdmin: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    monthlyLedger: { list, listPayments, setSettlement, setMultiplier, setMultipliers, createPayment, updatePayment, deletePayment },
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
    manually_settled: false,
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

const mountView = (paginationStub: unknown = true) => mount(MonthlyLedgerView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Pagination: paginationStub,
      Icon: true,
      MonthlyLedgerOverview: true,
      MonthlyLedgerIncome: true,
      MonthlyLedgerSendEmailDialog: true,
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
  it('shares one monthly overview across both tabs and refreshes it when other income changes', async () => {
    const wrapper = mountView()
    await flushPromises()
    const overview = wrapper.getComponent(MonthlyLedgerOverview)
    const revision = overview.props('revision')
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    expect(wrapper.findAllComponents(MonthlyLedgerOverview)).toHaveLength(1)
    expect(overview.props('relaySummary').usage_amount).toBe(600)
    wrapper.getComponent(MonthlyLedgerIncome).vm.$emit('changed')
    await flushPromises()
    expect(overview.props('revision')).toBe(revision + 1)
    await wrapper.findAll('[role="tab"]')[0].trigger('click')
    expect(wrapper.findAllComponents(MonthlyLedgerOverview)).toHaveLength(1)
    wrapper.unmount()
  })
  it('opens email from the row with its recipient and the selected month', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="open-send-email"]').exists()).toBe(false)
    await wrapper.get('[data-test="send-email-7"]').trigger('click')
    const dialog = wrapper.getComponent(MonthlyLedgerSendEmailDialog)
    expect(dialog.props('show')).toBe(true)
    expect(dialog.props('initialMonth')).toBe('2026-07')
    expect(dialog.props('recipient')).toMatchObject({ user_id: 7, email: 'customer@example.test', outstanding_amount: 600 })
  })

  it('hides row email actions without create permission and disables deleted users', async () => {
    canAdmin.mockImplementation((_resource, action) => action !== 'create')
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="send-email-7"]').exists()).toBe(false)
    wrapper.unmount()
    canAdmin.mockReturnValue(true)
    const data = response(); data.items[0].deleted = true
    list.mockResolvedValue(data)
    const deleted = mountView()
    await flushPromises()
    expect(deleted.get('[data-test="send-email-7"]').attributes('disabled')).toBeDefined()
  })
  beforeEach(() => {
    setSettlement.mockReset().mockImplementation(async (_month, _userID, settled) => ({ manually_settled: settled }))
    list.mockReset().mockResolvedValue(response())
    listPayments.mockReset().mockResolvedValue([])
    setMultiplier.mockReset().mockResolvedValue({ multiplier: 0.5 })
    setMultipliers.mockReset().mockResolvedValue({ billing_month: '2026-07', multiplier: 0.5, updated_count: 1 })
    createPayment.mockReset().mockResolvedValue({ id: 1 })
    updatePayment.mockReset().mockResolvedValue({ id: 1 })
    deletePayment.mockReset().mockResolvedValue({ id: 1 })
    showError.mockReset()
    showSuccess.mockReset()
    canAdmin.mockReset().mockReturnValue(true)
  })

  it('sets and cancels manual settlement without changing payment drafts', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="payment-amount"]').setValue(1000)
    const settled = response()
    settled.items[0].manually_settled = true
    settled.items[0].status = 'settled'
    settled.items[0].outstanding_amount = 0
    settled.summary.outstanding_amount = 0
    settled.summary.settled_count = 1
    list.mockResolvedValue(settled)
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    await flushPromises()
    expect(setSettlement).toHaveBeenLastCalledWith('2026-07', 7, true)
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get<HTMLInputElement>('[data-test="payment-amount"]').element.value).toBe('1000')
    expect(wrapper.getComponent(MonthlyLedgerOverview).props('relaySummary').outstanding_amount).toBe(0)
    expect(createPayment).not.toHaveBeenCalled()
    list.mockResolvedValue(response())
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    await flushPromises()
    expect(setSettlement).toHaveBeenLastCalledWith('2026-07', 7, false)
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('aria-checked')).toBe('false')
  })

  it('keeps the old settlement state when saving fails', async () => {
    setSettlement.mockRejectedValue(new Error('save failed'))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('aria-checked')).toBe('false')
    expect(showError).toHaveBeenCalled()
  })

  it('blocks duplicate settlement requests and does not update another open user', async () => {
    let resolveSave!: (value: { manually_settled: boolean }) => void
    setSettlement.mockImplementation(() => new Promise(resolve => { resolveSave = resolve }))
    const data = response()
    data.items.push({ ...data.items[0], user_id: 8, email: 'second@example.test' })
    list.mockResolvedValue(data)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    expect(setSettlement).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-test="close-dialog"]').trigger('click')
    await wrapper.get('[data-test="payments-8"]').trigger('click')
    resolveSave({ manually_settled: true })
    await flushPromises()
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('aria-checked')).toBe('false')
  })

  it.each([false, true])('disables settlement when open month or no update permission: %s', async (openMonth) => {
    list.mockResolvedValue(response(!openMonth))
    canAdmin.mockImplementation((_resource, action) => openMonth || action !== 'update')
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    expect(setSettlement).not.toHaveBeenCalled()
  })

  it('refreshes the active user independently when settlement removes it from the filtered list', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="payments-7"]').trigger('click')
    await flushPromises()
    const settled = response()
    settled.items[0].manually_settled = true
    settled.items[0].status = 'settled'
    list.mockResolvedValueOnce({ ...response(), items: [], total: 0 }).mockResolvedValueOnce(settled)
    await wrapper.get('[data-test="manual-settlement"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2026-07', user_id: 7, status: '' }))
    expect(wrapper.get('[data-test="manual-settlement"]').attributes('aria-checked')).toBe('true')
  })

  it('loads the backend default month and renders the billing amounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }))
    expect(wrapper.text()).toContain('customer@example.test')
    expect(wrapper.text()).toContain('$600.00')
    expect(wrapper.find<HTMLInputElement>('[data-test="ledger-month"]').element.value).toBe('2026-07')
    expect(wrapper.findAllComponents(MonthlyLedgerOverview)).toHaveLength(1)
    expect(wrapper.getComponent(MonthlyLedgerOverview).props('relaySummary')).toMatchObject({ usage_amount: 600, receivable_amount: 600, paid_amount: 0 })
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

  it.each(['single', 'batch'])('rejects an empty %s multiplier but allows an explicit zero', async (mode) => {
    const wrapper = mountView()
    await flushPromises()
    if (mode === 'batch') {
      await wrapper.get('[data-test="select-user-7"]').trigger('change')
      await wrapper.get('[data-test="open-bulk-multiplier"]').trigger('click')
    } else {
      await wrapper.get('[data-test="edit-multiplier-7"]').trigger('click')
    }
    await wrapper.get('#custom-multiplier').setValue('')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    expect(setMultiplier).not.toHaveBeenCalled()
    expect(setMultipliers).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.monthlyLedger.multiplier.invalid')

    await wrapper.get('[data-test="multiplier-preset-0"]').trigger('click')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()
    if (mode === 'batch') expect(setMultipliers).toHaveBeenCalledWith('2026-07', [7], 0)
    else expect(setMultiplier).toHaveBeenCalledWith('2026-07', 7, 0)
    wrapper.unmount()
  })

  it('discards stale selection context after a month fails to load and recovers on retry', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    list.mockRejectedValueOnce(new Error('month unavailable'))
    await wrapper.get('[data-test="ledger-month"]').setValue('2026-06')
    await flushPromises()
    expect(wrapper.find('[data-test="select-all-results"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="open-bulk-multiplier"]').attributes('disabled')).toBeDefined()

    list.mockResolvedValue({ ...response(), month: '2026-06', items: [{ ...response().items[0], user_id: 8 }] })
    await wrapper.get('#ledger-search').trigger('keyup.enter')
    await flushPromises()
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2026-06', page_size: 200 }))
    await wrapper.get('[data-test="open-bulk-multiplier"]').trigger('click')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()
    expect(setMultipliers).toHaveBeenCalledWith('2026-06', [8], 1)
    wrapper.unmount()
  })

  it.each([false, true])('preserves a newer multiplier dialog when an old save settles (failure: %s)', async (fails) => {
    let resolveSave!: () => void
    let rejectSave!: (error: Error) => void
    setMultiplier.mockImplementationOnce(() => new Promise<void>((resolve, reject) => {
      resolveSave = resolve
      rejectSave = reject
    }))
    list.mockResolvedValue({ ...response(), items: [response().items[0], { ...response().items[0], user_id: 8 }], total: 2 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="edit-multiplier-7"]').trigger('click')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await wrapper.get('[data-test="close-dialog"]').trigger('click')
    await wrapper.get('[data-test="edit-multiplier-8"]').trigger('click')
    await wrapper.get('#custom-multiplier').setValue('0.8')
    if (fails) rejectSave(new Error('old save failed'))
    else resolveSave()
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('#custom-multiplier').element.value).toBe('0.8')
    expect(wrapper.text()).not.toContain('old save failed')
    expect(wrapper.get('[data-test="save-multiplier"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()
    expect(setMultiplier).toHaveBeenLastCalledWith('2026-07', 8, 0.8)
    wrapper.unmount()
  })

  it('keeps partial selections across pages and updates all selected users', async () => {
    const secondRow = { ...response().items[0], user_id: 8, email: 'second@example.test' }
    list
      .mockResolvedValueOnce({ ...response(), total: 2, pages: 2 })
      .mockResolvedValueOnce({ ...response(), items: [secondRow], total: 2, page: 2, pages: 2 })
      .mockResolvedValue(response())
    const PaginationStub = {
      emits: ['update:page'],
      template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>',
    }
    const wrapper = mountView(PaginationStub)
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="select-user-8"]').trigger('change')
    await wrapper.get('[data-test="open-bulk-multiplier"]').trigger('click')
    await wrapper.get('[data-test="multiplier-preset-0.5"]').trigger('click')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()

    expect(setMultipliers).toHaveBeenCalledWith('2026-07', [7, 8], 0.5)
    expect(showSuccess).toHaveBeenCalledWith('admin.monthlyLedger.bulk.saved')
    expect(wrapper.get('[data-test="open-bulk-multiplier"]').attributes('disabled')).toBeDefined()
  })

  it('selects every row on the current page from the bulk action bar', async () => {
    list.mockResolvedValue({
      ...response(),
      items: [response().items[0], { ...response().items[0], user_id: 8, email: 'second@example.test' }],
      total: 3,
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-current-page"]').trigger('click')

    expect(wrapper.get<HTMLInputElement>('[data-test="select-user-7"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-test="select-user-8"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-test="select-visible"]').element.checked).toBe(true)
  })

  it('selects every result matching the current month, search, and status filters', async () => {
    const makeRow = (userID: number) => ({
      ...response().items[0],
      user_id: userID,
      email: `customer-${userID}@example.test`,
      status: 'partial' as const,
    })
    list.mockImplementation(async (params: { page?: number; page_size?: number }) => {
      if (params.page_size === 200) {
        const start = ((params.page || 1) - 1) * 200 + 1
        const end = Math.min((params.page || 1) * 200, 201)
        return {
          ...response(),
          items: Array.from({ length: end - start + 1 }, (_, index) => makeRow(start + index)),
          total: 201,
          page: params.page || 1,
          page_size: 200,
          pages: 2,
        }
      }
      return { ...response(), items: [makeRow(1)], total: 201, pages: 11 }
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#ledger-search').setValue('customer')
    await wrapper.get('#ledger-status').setValue('partial')
    await flushPromises()
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    const selectionCalls = list.mock.calls.filter(([params]) => params.page_size === 200)
    expect(selectionCalls).toHaveLength(2)
    expect(selectionCalls[0]?.[0]).toEqual(expect.objectContaining({
      month: '2026-07',
      q: 'customer',
      status: 'partial',
      page: 1,
      page_size: 200,
      sort_by: 'user',
      sort_order: 'asc',
    }))
    expect(wrapper.find('[data-test="select-all-results"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="open-bulk-multiplier"]').attributes('disabled')).toBeUndefined()
  })

  it('does not include an unsubmitted search draft in select-all requests', async () => {
    list.mockImplementation(async (params: { page_size?: number }) => ({
      ...response(),
      total: 1,
      page_size: params.page_size || 20,
    }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#ledger-search').setValue('not-applied')
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    const selectionCall = list.mock.calls.find(([params]) => params.page_size === 200)
    expect(selectionCall?.[0]).toEqual(expect.objectContaining({ month: '2026-07', q: undefined }))
  })

  it('keeps the existing partial selection when selecting all results fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    list.mockImplementation(async (params: { page_size?: number }) => {
      if (params.page_size === 200) throw new Error('load all failed')
      return { ...response(), total: 2, pages: 1 }
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-test="select-user-7"]').element.checked).toBe(true)
    expect(showError).toHaveBeenCalledWith('admin.monthlyLedger.bulk.selectAllFailed')
    consoleError.mockRestore()
  })

  it('clears the selection when filters are applied', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('#ledger-search').setValue('customer')
    await wrapper.get('#ledger-search').trigger('keyup.enter')
    await flushPromises()

    expect(wrapper.get('[data-test="open-bulk-multiplier"]').attributes('disabled')).toBeDefined()
  })

  it('clears the selection when the billing month changes', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('[data-test="ledger-month"]').setValue('2026-06')
    await flushPromises()

    expect(wrapper.get('[data-test="open-bulk-multiplier"]').attributes('disabled')).toBeDefined()
  })

  it('keeps the batch dialog and selection open when the update fails', async () => {
    setMultipliers.mockRejectedValue(new Error('batch failed'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('[data-test="open-bulk-multiplier"]').trigger('click')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('batch failed')
    expect(wrapper.get<HTMLInputElement>('[data-test="select-user-7"]').element.checked).toBe(true)
    expect(wrapper.get('[data-test="save-multiplier"]').exists()).toBe(true)
  })

  it('validates a custom batch multiplier before submitting', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-user-7"]').trigger('change')
    await wrapper.get('[data-test="open-bulk-multiplier"]').trigger('click')
    await wrapper.get('#custom-multiplier').setValue('0.12345')
    await wrapper.get('[data-test="save-multiplier"]').trigger('click')

    expect(setMultipliers).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.monthlyLedger.multiplier.invalid')
  })

  it('hides selection and bulk controls without monthly ledger update permission', async () => {
    canAdmin.mockImplementation((_resource: string, action: string) => action === 'view')
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="ledger-bulk-actions"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="select-user-7"]').exists()).toBe(false)
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
