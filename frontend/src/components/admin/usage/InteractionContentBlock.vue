<template>
  <article class="interaction-block min-w-0 py-4" :data-kind="block.kind" :data-role="block.role">
    <div class="mb-2 flex min-w-0 flex-wrap items-center gap-2 text-xs">
      <span class="min-w-0 break-all font-semibold" :class="roleColor">{{ label('roles', block.role) }}</span>
      <span v-if="block.kind !== 'text'" class="text-gray-500 dark:text-gray-400">{{ label('kinds', block.kind) }}</span>
      <span v-if="block.name" class="break-all font-mono font-medium">{{ block.name }}</span>
      <span v-if="block.group" class="break-all text-gray-500">{{ block.group }}</span>
      <span v-if="block.status" class="break-all text-gray-500">{{ label('statuses', block.status) }}</span>
      <button type="button" class="ml-auto flex h-7 w-7 shrink-0 items-center justify-center rounded hover:bg-gray-100 dark:hover:bg-dark-700"
        :title="t(`${base}.copy`)" :aria-label="t(`${base}.copy`)" @click="copy">
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>
    </div>
    <p v-if="copyFailed" role="alert" class="mb-2 text-xs text-red-600">{{ t(`${base}.copyFailed`) }}</p>
    <p v-if="block.call_id" class="mb-2 break-all font-mono text-xs text-gray-500">{{ block.call_id }}</p>
    <div v-if="block.sender || block.recipient" class="mb-2 flex min-w-0 flex-wrap items-center gap-2 text-xs text-gray-500">
      <span v-if="block.sender" class="break-all font-mono" :title="t(`${base}.sender`)">{{ block.sender }}</span>
      <Icon v-if="block.sender && block.recipient" name="arrowRight" size="sm" />
      <span v-if="block.recipient" class="break-all font-mono" :title="t(`${base}.recipient`)">{{ block.recipient }}</span>
    </div>
    <component :is="collapsible ? 'details' : 'div'" :open="collapsible ? expanded : undefined" @toggle="onToggle">
      <summary v-if="collapsible" class="cursor-pointer text-sm text-gray-600 dark:text-gray-300">{{ t(`${base}.content`) }} <span class="text-xs text-gray-400">({{ content.length }})</span></summary>
      <div v-if="!collapsible || expanded" :class="{ 'mt-3': collapsible }">
        <div v-if="block.children?.length" class="min-w-0 divide-y divide-gray-100 dark:divide-dark-700">
          <InteractionContentBlock
            v-for="(child, index) in block.children.slice(0, childLimit)"
            :key="`${child.source}-${index}`"
            :block="child"
            :linked="Boolean(child.call_id && resultIds?.has(child.call_id))"
            :result-ids="resultIds"
          />
          <button v-if="block.children.length > childLimit" type="button" class="btn btn-secondary my-3" @click="childLimit += 100">
            {{ t(`${base}.more`, { count: block.children.length - childLimit }) }}
          </button>
          <details v-if="block.data !== undefined && block.kind !== 'tool_result'" class="mt-2 text-xs text-gray-500">
            <summary class="cursor-pointer">{{ t(`${base}.metadata`) }}</summary>
            <pre class="interaction-data mt-2">{{ JSON.stringify(block.data, null, 2) }}</pre>
          </details>
        </div>
        <div v-else-if="isMarkdown" class="interaction-markdown" v-html="html" />
        <pre v-else-if="content" class="interaction-data">{{ content }}</pre>
        <p v-else class="text-sm text-gray-500">{{ t(`${base}.opaque`) }}</p>
      </div>
    </component>
    <p v-if="isTool" class="mt-2 text-xs text-gray-500">{{ t(`${base}.${linked ? 'resultPresent' : 'resultAbsent'}`) }}</p>
    <details class="mt-2 text-xs text-gray-400">
      <summary class="cursor-pointer">{{ t(`${base}.source`) }}</summary>
      <code class="mt-1 block break-all">{{ block.source }}</code>
    </details>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import type { InteractionBlock } from '@/api/admin/usage'

const props = defineProps<{ block: InteractionBlock; linked?: boolean; resultIds?: ReadonlySet<string> }>()
const { t, te } = useI18n()
const base = 'admin.usage.interaction.reader'
const expanded = ref(false)
const copied = ref(false)
const copyFailed = ref(false)
const childLimit = ref(100)
watch(() => props.block, () => {
  expanded.value = false
  copied.value = false
  copyFailed.value = false
  childLimit.value = 100
})
const onToggle = (event: Event) => {
  if (event.target === event.currentTarget) expanded.value = (event.target as HTMLDetailsElement).open
}
const label = (section: string, value: string) => te(`${base}.${section}.${value}`) ? t(`${base}.${section}.${value}`) : value
const isTool = computed(() => ['tool_call', 'custom_tool_call'].includes(props.block.kind))
const blockContent = (block: InteractionBlock): string => {
  if (block.text) return block.text
  if (block.children?.length) return block.children.map(blockContent).filter(Boolean).join('\n\n')
  if (typeof block.data === 'string') return block.data
  if (Array.isArray(block.data) && block.data.every(part => part?.type === 'text' && typeof part.text === 'string')) {
    return block.data.map(part => part.text).join('\n\n')
  }
  return block.data === undefined ? '' : JSON.stringify(block.data, null, 2)
}
const content = computed(() => blockContent(props.block))
const isMarkdown = computed(() => ['text', 'reasoning', 'refusal'].includes(props.block.kind)
  || (props.block.kind === 'tool_result' && (typeof props.block.data === 'string'
    || (Array.isArray(props.block.data) && props.block.data.every(part => part?.type === 'text')))))
const collapsible = computed(() => ['system', 'developer'].includes(props.block.role)
  || ['reasoning', 'tool_definitions', 'unknown', 'metadata', 'compaction'].includes(props.block.kind) || content.value.length > 6000)
const roleColor = computed(() => props.block.role === 'user' ? 'text-emerald-700 dark:text-emerald-400'
  : props.block.role === 'assistant' ? 'text-gray-900 dark:text-gray-100' : 'text-gray-500 dark:text-gray-400')
const escapeHtml = (value: string) => value.replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character] ?? character)
const markdown = new Marked({ renderer: {
  image: ({ href, text }) => href.startsWith('data:')
    ? escapeHtml(text || 'Image')
    : `<a href="${escapeHtml(href)}">${escapeHtml(text || 'Image')}</a>`,
} })
const html = computed(() => DOMPurify.sanitize(markdown.parse(content.value, { async: false, gfm: true }), {
  ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'del', 'code', 'pre', 'blockquote', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a', 'hr'],
  ALLOWED_ATTR: ['href', 'title'],
}))
const copy = async () => {
  copied.value = false
  copyFailed.value = false
  try { await navigator.clipboard.writeText(content.value); copied.value = true } catch { copyFailed.value = true }
}
</script>

<style scoped>
.interaction-data { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; line-height: 1.7; max-height: 36rem; overflow: auto; }
.interaction-markdown { font-size: 14px; line-height: 1.75; overflow-wrap: anywhere; min-width: 0; }
.interaction-markdown :deep(p) { margin: .5rem 0; }
.interaction-markdown :deep(h1), .interaction-markdown :deep(h2), .interaction-markdown :deep(h3), .interaction-markdown :deep(h4) { font-size: 16px; font-weight: 600; margin: 1rem 0 .5rem; }
.interaction-markdown :deep(pre) { overflow-x: auto; max-width: 100%; padding: 12px; background: rgb(127 127 127 / 8%); border-radius: 4px; font-size: 12px; }
.interaction-markdown :deep(code) { font-size: 12px; }
.interaction-markdown :deep(ul), .interaction-markdown :deep(ol) { padding-left: 1.5rem; list-style: revert; }
.interaction-markdown :deep(blockquote) { border-left: 2px solid #9ca3af; padding-left: 12px; }
.interaction-markdown :deep(table) { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; }
.interaction-markdown :deep(td), .interaction-markdown :deep(th) { border: 1px solid rgb(127 127 127 / 30%); padding: 6px 10px; }
.interaction-markdown :deep(a) { text-decoration: underline; color: #059669; }
</style>
