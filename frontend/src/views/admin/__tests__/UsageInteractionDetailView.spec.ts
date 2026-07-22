import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UsageInteractionDetailView from '../UsageInteractionDetailView.vue'

const { getInteraction, getInteractionRaw, push, rawPermission, route, stepUpRun } = vi.hoisted(() => ({
  getInteraction: vi.fn(),
  getInteractionRaw: vi.fn(),
  push: vi.fn(),
  stepUpRun: vi.fn(),
  rawPermission: { value: true },
  route: {
    params: { id: '42' },
    query: { return: '/admin/usage?page=2' },
  },
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    getInteraction,
    getInteractionRaw,
  },
  default: {
    getInteraction,
    getInteractionRaw,
  },
}))

vi.mock('@/composables/useAdminPermissionGate', () => ({
  useAdminPermissionGate: () => ({
    can: () => rawPermission,
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    prompt: vi.fn(),
    onVerified: vi.fn(),
    onCancel: vi.fn(),
    run: stepUpRun,
  }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const interactionWithoutRaw = {
  exists: true,
  interaction: {
    id: 7,
    usage_log_id: 42,
    request_id: 'req-42',
    created_at: '2026-07-07T00:00:00Z',
    capture_status: 'complete',
    request_content: { messages: [{ role: 'user', content: 'preserved input' }] },
    response_content: { choices: [{ message: { content: 'preserved output' } }] },
    request_parameters: { temperature: 0.2 },
    routing_context: { upstream_model: 'gpt-5.3-codex' },
    raw_available: true,
    raw_request_json: null,
    raw_response_json: null,
    redaction_applied: true,
    redaction_keys: ['authorization'],
  },
}

const mountView = () => mount(UsageInteractionDetailView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Icon: true,
      TotpStepUpDialog: true,
    },
  },
})

describe('UsageInteractionDetailView', () => {
  beforeEach(() => {
    getInteraction.mockReset()
    getInteractionRaw.mockReset()
    stepUpRun.mockReset()
    push.mockReset()
    rawPermission.value = true
    getInteraction.mockResolvedValue(interactionWithoutRaw)
    stepUpRun.mockImplementation((action: () => Promise<unknown>) => action())
    getInteractionRaw.mockResolvedValue({
      exists: true,
      interaction: {
        ...interactionWithoutRaw.interaction,
        raw_request_json: { messages: [{ role: 'user', content: 'raw request' }] },
        raw_response_json: { choices: [{ message: { content: 'raw output' } }] },
      },
    })
  })

  it('shows the captured output from the safe endpoint before raw data is requested', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getInteraction).toHaveBeenCalledWith(42)
    expect(getInteractionRaw).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('preserved input')
    await wrapper.get('[data-test="usage-interaction-tab-output"]').trigger('click')
    expect(wrapper.text()).toContain('preserved output')
    expect(wrapper.text()).not.toContain('raw output')
  })

  it('uses the dedicated raw endpoint only after a permitted user opens the raw tab', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="usage-interaction-tab-raw"]').trigger('click')
    await flushPromises()

    expect(getInteractionRaw).toHaveBeenCalledWith(42)
    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('raw request')
    expect(wrapper.text()).toContain('raw output')
  })

  it('does not expose the raw tab without its independent permission', async () => {
    rawPermission.value = false
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="usage-interaction-tab-raw"]').exists()).toBe(false)
  })

  it('returns to the usage list route supplied by the query string', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="usage-interaction-back"]').trigger('click')

    expect(push).toHaveBeenCalledWith('/admin/usage?page=2')
  })
})
