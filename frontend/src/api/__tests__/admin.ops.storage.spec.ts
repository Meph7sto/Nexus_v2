import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
  buildGatewayUrl: vi.fn(),
}))

import { getStorageUsage } from '@/api/admin/ops'

describe('admin Ops storage API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('requests the protected storage endpoint with the caller cancellation signal', async () => {
    const controller = new AbortController()
    const response = {
      generated_at: '2026-07-20T12:00:00Z',
      total_used_bytes: 1536,
      items: [],
    }
    get.mockResolvedValue({ data: response })

    await expect(getStorageUsage({ signal: controller.signal })).resolves.toEqual(response)

    expect(get).toHaveBeenCalledWith('/admin/ops/storage', {
      signal: controller.signal,
    })
  })
})
