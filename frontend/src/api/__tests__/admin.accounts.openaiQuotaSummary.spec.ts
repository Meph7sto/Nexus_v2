import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import { getOpenAIQuotaSummary } from '@/api/admin/accounts'

describe('admin OpenAI quota summary API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('requests the quota summary endpoint with the selected filters', async () => {
    const summary = {
      projection_at: '2026-07-20T12:00:00Z',
      generated_at: '2026-07-20T12:00:01Z',
      groups: []
    }
    const params = {
      projection_at: '2026-07-20T14:00:00Z',
      group: '12',
      type: 'plus'
    }
    get.mockResolvedValueOnce({ data: summary })

    await expect(getOpenAIQuotaSummary(params)).resolves.toEqual(summary)
    expect(get).toHaveBeenCalledWith('/admin/openai/quota-summary', { params })
  })
})
