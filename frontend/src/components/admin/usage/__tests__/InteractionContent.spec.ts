import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import InteractionContent from '../InteractionContent.vue'
import type { InteractionDocument } from '@/api/admin/usage'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, te: () => true }) }))

const document = (blocks: InteractionDocument['blocks']): InteractionDocument => ({
  protocol: 'responses', format: 'sse', warnings: [], ended: true, blocks,
})

describe('InteractionContent', () => {
  it('renders Markdown without executing HTML or loading external media', () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Output', document: document([
      { role: 'assistant', kind: 'text', source: '$', text: '# Answer\n**hello**\n<script>alert(1)</script><img src="https://tracking.test/a"><iframe src="https://tracking.test/b"></iframe>\n[x](javascript:alert(1))' },
    ]) } })
    expect(wrapper.find('strong').text()).toBe('hello')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('a').attributes('href')).toBeUndefined()
  })

  it('lazily opens system and reasoning content, keeping normal text readable', async () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Input', document: document([
      { role: 'system', kind: 'text', source: '$.system', text: 'system instruction' },
      { role: 'assistant', kind: 'reasoning', source: '$.summary', text: 'recorded summary' },
      { role: 'assistant', kind: 'text', source: '$.text', text: 'visible answer' },
    ]) } })
    expect(wrapper.text()).not.toContain('system instruction')
    expect(wrapper.text()).not.toContain('recorded summary')
    expect(wrapper.text()).toContain('visible answer')
    const details = wrapper.get('[data-role="system"] details')
    ;(details.element as HTMLDetailsElement).open = true
    await details.trigger('toggle')
    expect(wrapper.text()).toContain('system instruction')
  })

  it('only reports a result when an explicit matching call ID exists', () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Output', document: document([
      { role: 'assistant', kind: 'tool_call', name: 'read', call_id: 'a', data: { path: 'a' }, source: '$.a' },
      { role: 'assistant', kind: 'tool_call', name: 'read', call_id: 'b', data: { path: 'b' }, source: '$.b' },
      { role: 'tool', kind: 'tool_result', call_id: 'a', data: 'file text', source: '$.result' },
    ]) } })
    expect(wrapper.findAll('[data-kind="tool_call"]')[0].text()).toContain('reader.resultPresent')
    expect(wrapper.findAll('[data-kind="tool_call"]')[1].text()).toContain('reader.resultAbsent')
    expect(wrapper.get('[data-kind="tool_result"]').text()).toContain('file text')
  })

  it('renders warnings and batches long records without losing remaining blocks', async () => {
    const doc = document(Array.from({ length: 101 }, (_, i) => ({ role: 'user', kind: 'text', source: `$[${i}]`, text: `message ${i}` })))
    doc.warnings = ['truncated', 'invalid_event:5']
    const wrapper = mount(InteractionContent, { props: { title: 'Input', document: doc } })
    expect(wrapper.findAll('article')).toHaveLength(100)
    expect(wrapper.get('[role="status"]').text()).toContain('warnings.truncated')
    const more = wrapper.findAll('button').find(button => button.text().includes('reader.more'))!
    await more.trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(101)
  })

  it('copies readable text and reports clipboard failure', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mount(InteractionContent, { props: { title: 'Output', document: document([
      { role: 'assistant', kind: 'text', source: '$', text: 'hello' },
    ]) } })
    await wrapper.get('button').trigger('click')
    expect(writeText).toHaveBeenCalledWith('hello')
    writeText.mockRejectedValueOnce(new Error('denied'))
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('renders mixed tool results as separate safe blocks in their original order', () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Output', document: document([
      { role: 'tool', kind: 'tool_result', call_id: 'c', source: '$.result', children: [
        { role: 'tool', kind: 'text', source: '$.result[0]', text: '**Readable result**' },
        { role: 'tool', kind: 'attachment', source: '$.result[1]', data: { type: 'image_url', image_url: { url: 'data:image/png;base64,[4 characters]' } } },
        { role: 'tool', kind: 'text', source: '$.result[2]', text: 'After the image' },
      ] },
    ]) } })
    expect(wrapper.get('strong').text()).toBe('Readable result')
    const children = wrapper.get('[data-kind="tool_result"]').findAll('article')
    expect(children.map(child => child.attributes('data-kind'))).toEqual(['text', 'attachment', 'text'])
    expect(children[2].text()).toContain('After the image')
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('shows agent routing, keeps compaction opaque, and preserves unknown metadata', async () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Input', document: document([
      { role: 'agent', kind: 'agent_message', sender: '/root', recipient: '/root/helper', source: '$.agent', children: [
        { role: 'agent', kind: 'text', text: 'Retained task', source: '$.agent.content[0]' },
        { role: 'agent', kind: 'opaque', name: 'encrypted_content', source: '$.agent.content[1]' },
      ] },
      { role: 'system', kind: 'compaction', source: '$.compaction', data: { id: 'cmp_1', encrypted_content: '[opaque: 4 characters]' } },
      { role: 'user', kind: 'unknown', source: '$.future', data: { type: 'future_part', text: 'Extension body', confidence: 0.9 }, children: [
        { role: 'user', kind: 'text', text: 'Extension body', source: '$.future.text' },
      ] },
    ]) } })
    expect(wrapper.text()).toContain('/root/helper')
    expect(wrapper.text()).toContain('Retained task')
    expect(wrapper.get('[data-kind="opaque"]').text()).toContain('reader.opaque')
    const details = wrapper.get('[data-kind="unknown"] > details')
    ;(details.element as HTMLDetailsElement).open = true
    await details.trigger('toggle')
    expect(wrapper.get('[data-kind="unknown"]').text()).toContain('Extension body')
    expect(wrapper.get('[data-kind="unknown"]').text()).toContain('confidence')
    expect(wrapper.get('[data-kind="compaction"]').find('pre').exists()).toBe(false)
  })

  it('resets expanded content and batches nested blocks when the record changes', async () => {
    const wrapper = mount(InteractionContent, { props: { title: 'Input', document: document([
      { role: 'tool', kind: 'tool_result', source: '$.tool', children: Array.from({ length: 101 }, (_, i) => ({ role: 'tool', kind: 'text', text: `part ${i}`, source: `$[${i}]` })) },
    ]) } })
    expect(wrapper.findAll('article')).toHaveLength(101)
    const more = wrapper.findAll('button').find(button => button.text().includes('reader.more'))!
    await more.trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(102)
    await wrapper.setProps({ document: document([
      { role: 'system', kind: 'text', text: 'New private context', source: '$.tool' },
    ]) })
    expect(wrapper.text()).not.toContain('New private context')
  })
})
