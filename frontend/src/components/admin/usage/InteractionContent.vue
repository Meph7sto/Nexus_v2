<template>
  <section class="min-w-0" data-test="interaction-content">
    <header class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 py-3 dark:border-dark-700">
      <h2 class="text-sm font-semibold">{{ title }}</h2>
      <span class="text-xs text-gray-500">{{ document.protocol }} · {{ document.format }} · {{ document.blocks.length }}</span>
    </header>
    <div v-if="document.warnings?.length" role="status" class="my-3 break-words border-l-2 border-amber-500 bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
      <p v-for="warning in document.warnings" :key="warning">{{ warningLabel(warning) }}</p>
    </div>
    <p v-if="!document.blocks.length" class="py-6 text-sm text-gray-500">{{ t(`${base}.empty`) }}</p>
    <div class="divide-y divide-gray-200 dark:divide-dark-700">
      <InteractionContentBlock
        v-for="(block, index) in document.blocks.slice(0, limit)"
        :key="`${block.source}-${index}`"
        :block="block"
        :linked="isLinked(block)"
        :result-ids="resultIds"
      />
    </div>
    <button v-if="document.blocks.length > limit" type="button" class="btn btn-secondary my-4" @click="limit += 100">
      {{ t(`${base}.more`, { count: document.blocks.length - limit }) }}
    </button>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { InteractionBlock, InteractionDocument } from '@/api/admin/usage'
import InteractionContentBlock from './InteractionContentBlock.vue'

const props = defineProps<{ document: InteractionDocument; title: string; relatedBlocks?: InteractionBlock[] }>()
const { t, te } = useI18n()
const base = 'admin.usage.interaction.reader'
const limit = ref(100)
watch(() => props.document, () => { limit.value = 100 })
const resultIds = computed(() => {
  const ids = new Set<string>()
  const pending = [...(props.relatedBlocks ?? props.document.blocks)]
  while (pending.length) {
    const block = pending.pop()!
    if (block.kind === 'tool_result' && block.call_id) ids.add(block.call_id)
    if (block.children) pending.push(...block.children)
  }
  return ids
})
const isLinked = (block: InteractionBlock) => Boolean(block.call_id && resultIds.value.has(block.call_id))
const warningLabel = (warning: string) => {
  const key = `${base}.warnings.${warning.split(':')[0]}`
  return te(key) ? t(key) : warning
}
</script>
