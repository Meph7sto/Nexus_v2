import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import Dialog from '../MonthlyLedgerEmailHistoryDialog.vue'
const { listEmailHistory } = vi.hoisted(() => ({ listEmailHistory: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { monthlyLedger: { listEmailHistory } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const record = { id: 1, recipient: 'test@example.test', amount: 42.35, subject: 'August invoice', html: '<h1>Pay $42.35</h1><script>alert(1)</script><img src="https://tracker.test/pixel">', status: 'sent', source_type: 'monthly_ledger_manual', sent_at: '2026-09-01T00:00:00Z' }
const mountDialog = () => mount(Dialog, { props: { month: '2026-08', recipient: { user_id: 7, email: 'test@example.test' } }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true, Pagination: true } } })
beforeEach(() => { vi.resetAllMocks(); listEmailHistory.mockResolvedValue({ items: [record], total: 1, sent_count: 1 }) })
describe('ledger email history', () => {
  it('loads scoped history and previews the saved body in an isolated frame', async () => {
    const wrapper = mountDialog(); await flushPromises()
    expect(listEmailHistory).toHaveBeenCalledWith('2026-08', 7, 1)
    expect(wrapper.text()).toContain('$42.35'); expect(wrapper.text()).toContain('August invoice')
    await wrapper.get('button[aria-expanded]').trigger('click')
    const frame = wrapper.get('iframe')
    expect(frame.attributes('sandbox')).toBe('')
    expect(frame.attributes('srcdoc')).toContain('Pay $42.35')
    expect(frame.attributes('srcdoc')).not.toContain('<script>')
    expect(frame.attributes('srcdoc')).toContain("img-src data:")
    wrapper.unmount()
  })
  it('does not invent a preview for legacy deliveries', async () => {
    listEmailHistory.mockResolvedValue({ items: [{ ...record, subject: '', html: '' }], total: 1, sent_count: 1 })
    const wrapper = mountDialog(); await flushPromises()
    expect(wrapper.text()).toContain('contentUnavailable'); expect(wrapper.find('button[aria-expanded]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('displays request failures', async () => {
    listEmailHistory.mockRejectedValue({ message: 'Request failed' })
    const wrapper = mountDialog(); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Request failed')
    wrapper.unmount()
  })
})
