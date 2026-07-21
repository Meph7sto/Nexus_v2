import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LeaderboardView from '../LeaderboardView.vue'

const { getRanking, showError } = vi.hoisted(() => ({
  getRanking: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api', () => ({
  usageAPI: { getRanking },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en' },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const IconStub = { template: '<span />' }
const DateRangePickerStub = {
  template: `
    <div>
      <button data-testid="range-31" @click="$emit('change', { startDate: '2026-03-08', endDate: '2026-04-07' })">31</button>
      <button data-testid="range-32" @click="$emit('change', { startDate: '2026-03-08', endDate: '2026-04-08' })">32</button>
    </div>
  `,
}
const PaginationStub = {
  props: ['page', 'total', 'pageSize', 'showJump'],
  template: `
    <div data-testid="pagination" :data-show-jump="showJump">
      <button data-testid="jump-page" @click="$emit('update:page', 9)">jump</button>
      <button data-testid="page-size" @click="$emit('update:pageSize', 200)">size</button>
    </div>
  `,
}

describe('LeaderboardView', () => {
  beforeEach(() => {
    getRanking.mockReset()
    showError.mockReset()
    getRanking.mockResolvedValue({
      items: [
        {
          rank: 1,
          nickname: 'a***e',
          email: 'a****@example.test',
          requests: 3,
          total_tokens: 1200,
          total_actual_cost: 0.42,
        },
      ],
      total: 100,
      page: 1,
      page_size: 20,
      pages: 5,
    })
  })

  const mountView = () => mount(LeaderboardView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        DateRangePicker: DateRangePickerStub,
        Pagination: PaginationStub,
        Icon: IconStub,
      },
    },
  })

  it('loads the cost leaderboard by default', async () => {
    mountView()
    await flushPromises()

    expect(getRanking).toHaveBeenCalledWith(expect.objectContaining({
      rank_by: 'cost',
      page: 1,
      page_size: 20,
    }))
  })

  it('switches to token ranking and resets the page', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="jump-page"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('leaderboard.tokens'))!.trigger('click')
    await flushPromises()

    expect(getRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      rank_by: 'tokens',
      page: 1,
    }))
  })

  it('accepts a 31-day natural-date range and rejects a 32-day range before requesting', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="range-31"]').trigger('click')
    await flushPromises()
    expect(getRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-03-08',
      end_date: '2026-04-07',
      page: 1,
    }))

    const callsBeforeInvalidRange = getRanking.mock.calls.length
    await wrapper.find('[data-testid="range-32"]').trigger('click')
    await flushPromises()

    expect(getRanking).toHaveBeenCalledTimes(callsBeforeInvalidRange)
    expect(wrapper.text()).toContain('leaderboard.rangeTooLarge')
  })

  it('caps a pagination page-size change at 100', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="page-size"]').trigger('click')
    await flushPromises()

    expect(getRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      page_size: 100,
    }))
  })

  it('shows specific retry messages for rate limiting and query timeouts', async () => {
    const wrapper = mountView()
    await flushPromises()
    getRanking.mockRejectedValueOnce({ status: 429 })

    await wrapper.find('.leaderboard-refresh').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('leaderboard.rateLimited')
    expect(wrapper.text()).toContain('leaderboard.rateLimited')

    getRanking.mockRejectedValueOnce({ status: 504 })
    await wrapper.find('.leaderboard-refresh').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('leaderboard.timedOut')
    expect(wrapper.text()).toContain('leaderboard.timedOut')
  })
})
