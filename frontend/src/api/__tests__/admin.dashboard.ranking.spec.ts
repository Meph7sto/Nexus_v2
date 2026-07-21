import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getUserSpendingRanking } from '@/api/admin/dashboard'

describe('admin dashboard spending ranking api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('keeps the operational ranking endpoint and unredacted row contract', async () => {
    const response = {
      ranking: [{
        user_id: 7,
        email: 'operator-visible@example.test',
        actual_cost: 4.2,
        requests: 3,
        tokens: 900,
      }],
      total_actual_cost: 4.2,
      total_requests: 3,
      total_tokens: 900,
      start_date: '2026-07-01',
      end_date: '2026-07-07',
    }
    get.mockResolvedValue({ data: response })

    const result = await getUserSpendingRanking({
      start_date: '2026-07-01',
      end_date: '2026-07-07',
      limit: 12,
    })

    expect(get).toHaveBeenCalledWith('/admin/dashboard/users-ranking', {
      params: {
        start_date: '2026-07-01',
        end_date: '2026-07-07',
        limit: 12,
      },
    })
    expect(result.ranking[0]).toEqual(response.ranking[0])
  })
})
