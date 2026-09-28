import { beforeEach, describe, expect, it, vi } from 'vitest'
const { get, post, put, remove } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), remove: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put, delete: remove } }))
import { incomeAPI } from '../monthlyLedgerIncome'
describe('income API', () => {
  beforeEach(() => { vi.clearAllMocks(); for (const fn of [get, post, put, remove]) fn.mockResolvedValue({ data: {} }) })
  it('keeps external income independent of relay user/month payment routes', async () => {
    const input = { amount: 20, paid_at: '2026-09-01T12:00:00Z', note: 'partial' }
    await incomeAPI.savePayment(7, 0, input); await incomeAPI.savePayment(7, 4, input); await incomeAPI.deletePayment(4)
    expect(post).toHaveBeenCalledWith('/admin/monthly-ledger/income-entries/7/payments', input)
    expect(put).toHaveBeenCalledWith('/admin/monthly-ledger/income-payments/4', input)
    expect(remove).toHaveBeenCalledWith('/admin/monthly-ledger/income-payments/4')
  })
  it('requests an unfiltered month overview and explicit schedule preview', async () => {
    await incomeAPI.overview('2026-09')
    expect(get).toHaveBeenCalledWith('/admin/monthly-ledger/overview', { params: { month: '2026-09' } })
    const input = { title: 'Service', customer: 'External', category: 'Custom', amount: 100, cost: 80, note: '', first_date: '2026-01-31', interval_months: 3, end_date: null }
    await incomeAPI.preview(input); expect(post).toHaveBeenCalledWith('/admin/monthly-ledger/income-schedules/preview', input)
    await incomeAPI.setState(1, 'paused'); expect(put).toHaveBeenCalledWith('/admin/monthly-ledger/income-schedules/1/state', { state: 'paused' })
  })
})
