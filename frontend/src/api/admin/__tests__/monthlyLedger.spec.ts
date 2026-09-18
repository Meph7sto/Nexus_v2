import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  remove: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: remove },
}))

import monthlyLedgerAPI from '../monthlyLedger'

describe('monthly ledger admin API', () => {
  it.each([true, false])('sends the settlement boolean unchanged: %s', async (settled) => {
    put.mockResolvedValue({ data: { manually_settled: settled } })
    expect(await monthlyLedgerAPI.setSettlement('2026-07', 7, settled)).toEqual({ manually_settled: settled })
    expect(put).toHaveBeenCalledWith('/admin/monthly-ledger/2026-07/users/7/settlement', { manually_settled: settled })
  })
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    remove.mockReset()
  })

  it('uses the month/user resource paths for multiplier and payments', async () => {
    get.mockResolvedValue({ data: [] })
    post.mockResolvedValue({ data: { id: 9 } })
    put.mockResolvedValue({ data: {} })
    remove.mockResolvedValue({ data: { id: 9 } })

    await monthlyLedgerAPI.list({ month: '2026-07', user_id: 7, page: 2 })
    await monthlyLedgerAPI.listPayments('2026-07', 7)
    await monthlyLedgerAPI.setMultiplier('2026-07', 7, 0.5)
    await monthlyLedgerAPI.setMultipliers('2026-07', [7, 11], 0.8)
    await monthlyLedgerAPI.createPayment('2026-07', 7, {
      amount: 300,
      paid_at: '2026-08-02T10:00:00Z',
      note: 'cash',
    })
    await monthlyLedgerAPI.updatePayment(9, {
      amount: 280,
      paid_at: '2026-08-02T10:00:00Z',
      note: 'corrected',
    })
    await monthlyLedgerAPI.deletePayment(9)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/monthly-ledger', { params: { month: '2026-07', user_id: 7, page: 2 } })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/monthly-ledger/2026-07/users/7/payments')
    expect(put).toHaveBeenNthCalledWith(1, '/admin/monthly-ledger/2026-07/users/7/multiplier', { multiplier: 0.5 })
    expect(put).toHaveBeenNthCalledWith(2, '/admin/monthly-ledger/2026-07/multipliers', { user_ids: [7, 11], multiplier: 0.8 })
    expect(post).toHaveBeenCalledWith('/admin/monthly-ledger/2026-07/users/7/payments', expect.objectContaining({ amount: 300 }))
    expect(put).toHaveBeenNthCalledWith(3, '/admin/monthly-ledger/payments/9', expect.objectContaining({ amount: 280 }))
    expect(remove).toHaveBeenCalledWith('/admin/monthly-ledger/payments/9')
  })
})
