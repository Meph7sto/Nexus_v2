import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthlyLedgerIncome from '../MonthlyLedgerIncome.vue'
import IncomeSchedulesDialog from '../IncomeSchedulesDialog.vue'
import MonthlyLedgerOverview from '../MonthlyLedgerOverview.vue'
const { api, canAdmin } = vi.hoisted(() => ({ api: { list: vi.fn(), saveEntry: vi.fn(), deleteEntry: vi.fn(), payments: vi.fn(), savePayment: vi.fn(), deletePayment: vi.fn(), schedules: vi.fn(), preview: vi.fn(), saveSchedule: vi.fn(), setState: vi.fn(), overview: vi.fn() }, canAdmin: vi.fn() }))
vi.mock('@/api/admin/monthlyLedgerIncome', () => ({ incomeAPI: api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ canAdmin }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const summary = { receivable_amount: 100, paid_amount: 40, outstanding_amount: 60, overpaid_amount: 0, cost: 120, profit: -20 }
const entry = { id: 7, title: 'Top-up', customer: 'External customer', category: 'Custom', amount: 100, cost: 120, profit: -20, paid_amount: 40, status: 'partial', due_date: '2026-09-30', billing_date: '2026-09-28', note: '', schedule_id: null }
const global = { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, Icon: true, Pagination: true } }
beforeEach(() => {
  vi.clearAllMocks(); canAdmin.mockReturnValue(true)
  api.list.mockResolvedValue({ items: [entry], total: 1, categories: ['Custom'], summary })
  api.schedules.mockResolvedValue([]); api.payments.mockResolvedValue([]); api.saveEntry.mockResolvedValue(entry); api.preview.mockResolvedValue({ count: 9 }); api.saveSchedule.mockResolvedValue({ id: 1 }); api.overview.mockResolvedValue(summary)
})
describe('other income', () => {
  it('shows actual relay usage separately from the combined monthly totals and refreshes after changes', async () => {
    const relaySummary = { usage_amount: 5550.33, receivable_amount: 3425.8, paid_amount: 75, outstanding_amount: 3301.22, overpaid_amount: 0, user_count: 16, unpaid_count: 15, partial_count: 0, settled_count: 1, waived_count: 0, overpaid_count: 0 }
    api.overview.mockResolvedValue({ ...summary, receivable_amount: 3565.8, paid_amount: 75, outstanding_amount: 3441.22 })
    const wrapper = mount(MonthlyLedgerOverview, { props: { month: '2026-09', revision: 0, relaySummary }, global })
    await flushPromises()
    expect(wrapper.findAll('.summary-card')).toHaveLength(5)
    expect(wrapper.get('[data-test="summary-usage"]').text()).toContain('$5550.33')
    expect(wrapper.get('[data-test="summary-receivable"]').text()).toContain('$3565.80')
    expect(wrapper.get('[data-test="summary-outstanding"]').text()).toContain('$3441.22')
    api.overview.mockResolvedValue({ ...summary, receivable_amount: 3600 })
    await wrapper.setProps({ revision: 1 })
    await flushPromises()
    expect(wrapper.get('[data-test="summary-receivable"]').text()).toContain('$3600.00')
    expect(api.overview).toHaveBeenLastCalledWith('2026-09')
    wrapper.unmount()
  })
  it('defaults new entries to the viewed month and shows structured API errors', async () => {
    const wrapper = mount(MonthlyLedgerIncome, { props: { month: '2020-08' }, global })
    await flushPromises()
    await wrapper.get('[data-test="add-income"]').trigger('click')
    expect((wrapper.get('input[name="billing_date"]').element as HTMLInputElement).value).toBe('2020-08-01')
    expect((wrapper.get('input[name="due_date"]').element as HTMLInputElement).value).toBe('2020-08-01')
    api.saveEntry.mockRejectedValueOnce({ status: 400, message: 'Billing date cannot be in the future' })
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Billing date cannot be in the future')
    expect(wrapper.find('input[name="billing_date"]').exists()).toBe(true)
    wrapper.unmount()
  })
  it('submits through the actual save button with native form validation', async () => {
    const wrapper = mount(MonthlyLedgerIncome, { props: { month: '2020-08' }, global, attachTo: document.body })
    await flushPromises()
    await wrapper.get('[data-test="add-income"]').trigger('click')
    const form = wrapper.findAll('form')[1]
    await form.get('button.btn-primary').trigger('click')
    expect(api.saveEntry).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('invalidFields')
    await wrapper.get('input[name="title"]').setValue('Support')
    await wrapper.get('input[name="customer"]').setValue('Customer')
    await wrapper.get('input[name="amount"]').setValue('12.34')
    await form.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(api.saveEntry).toHaveBeenCalledOnce()
    expect(api.saveEntry).toHaveBeenCalledWith(0, expect.objectContaining({ billing_date: '2020-08-01', amount: 12.34 }))
    expect(wrapper.find('input[name="billing_date"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('shows negative expected profit, supports current-month creation, and prevents deleting paid entries', async () => {
    const wrapper = mount(MonthlyLedgerIncome, { props: { month: '2026-09' }, global }); await flushPromises()
    expect(wrapper.text()).toContain('$-20.00')
    expect(wrapper.find('button[title="admin.monthlyLedger.income.hasPayments"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="add-income"]').trigger('click')
    await wrapper.get('input[name="title"]').setValue('Account reset'); await wrapper.get('input[name="customer"]').setValue('New customer'); await wrapper.get('input[name="amount"]').setValue(25)
    await wrapper.findAll('form')[1].trigger('submit'); await flushPromises()
    expect(api.saveEntry).toHaveBeenCalledWith(0, expect.objectContaining({ title: 'Account reset', customer: 'New customer', amount: 25, cost: 0 }))
    expect(wrapper.emitted('changed')).toBeTruthy(); wrapper.unmount()
  })
  it('applies month/category/status filters and respects read-only permissions', async () => {
    canAdmin.mockReturnValue(false)
    const wrapper = mount(MonthlyLedgerIncome, { props: { month: '2026-09' }, global }); await flushPromises()
    expect(wrapper.find('[data-test="add-income"]').exists()).toBe(false)
    await wrapper.findAll('select')[0].setValue('Custom'); await wrapper.findAll('select')[1].setValue('partial'); await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2026-09', category: 'Custom', status: 'partial', page: 1 }))
    await wrapper.setProps({ month: '2026-08' }); await flushPromises(); expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2026-08', page: 1 })); wrapper.unmount()
  })
  it('requires reviewing catch-up count before creating a recurring schedule', async () => {
    const wrapper = mount(IncomeSchedulesDialog, { props: { show: true }, global })
    await wrapper.findAll('button').find(b => b.text().includes('addSchedule'))!.trigger('click')
    await wrapper.get('input[name="title"]').setValue('Monthly support'); await wrapper.get('input[name="customer"]').setValue('External'); await wrapper.get('input[name="amount"]').setValue(100)
    await wrapper.get('form').trigger('submit'); await flushPromises(); expect(api.preview).toHaveBeenCalledOnce(); expect(api.saveSchedule).not.toHaveBeenCalled()
    await wrapper.get('form').trigger('submit'); await flushPromises(); expect(api.saveSchedule).toHaveBeenCalledOnce(); expect(wrapper.emitted('changed')).toBeTruthy(); wrapper.unmount()
  })
  it('clears stale month totals and reports an overview failure rather than showing zero', async () => {
    const wrapper = mount(MonthlyLedgerOverview, { props: { month: '2026-09', revision: 0 }, global }); await flushPromises(); expect(wrapper.text()).toContain('$100.00')
    api.overview.mockRejectedValue(new Error('offline')); await wrapper.setProps({ month: '2026-08' }); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('loadFailed'); expect(wrapper.text()).not.toContain('$100.00'); wrapper.unmount()
  })
})
