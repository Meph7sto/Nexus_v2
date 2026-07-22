<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { opsAPI, type OpsStorageUsageItem, type OpsStorageUsageResponse } from '@/api/admin/ops'
import { useAdminPermissionGate } from '@/composables/useAdminPermissionGate'
import { formatBytes } from '@/utils/format'

interface Props {
  refreshKey?: number | string | null
  fullscreen?: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()
const { can } = useAdminPermissionGate('ops')
const canView = can('view')

const loading = ref(false)
const data = ref<OpsStorageUsageResponse | null>(null)
const errorMessage = ref('')
let controller: AbortController | null = null
let requestSequence = 0

const totalLabel = computed(() => formatStorageBytes(data.value?.total_used_bytes))

const visibleItems = computed(() => {
  const items = data.value?.items ?? []
  return [...items].sort((left, right) => itemRank(left) - itemRank(right) || left.label.localeCompare(right.label))
})

function itemRank(item: OpsStorageUsageItem): number {
  if (item.key === 'postgres_data') return 0
  if (item.key === 'docker') return 1
  if (item.key === 'postgres_db') return 2
  if (item.key === 'app_data') return 3
  return 10
}

function formatStorageBytes(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '-'
  return formatBytes(value, value >= 1024 * 1024 * 1024 ? 1 : 0)
}

function statusLabel(item: OpsStorageUsageItem): string {
  if (item.status === 'ok') return formatStorageBytes(item.used_bytes)
  if (item.status === 'unconfigured') return t('admin.ops.storage.status.unconfigured')
  return t('admin.ops.storage.status.unavailable')
}

function statusClass(item: OpsStorageUsageItem): string {
  if (item.status === 'ok') return 'text-[color:var(--nx-text)]'
  if (item.status === 'unconfigured') return 'text-[color:var(--nx-subtle)]'
  return 'text-[color:var(--nx-warning)]'
}

function isCancellationError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const candidate = error as { code?: unknown; name?: unknown }
  return candidate.code === 'ERR_CANCELED' || candidate.name === 'AbortError' || candidate.name === 'CanceledError'
}

function clearForMissingPermission() {
  requestSequence += 1
  controller?.abort()
  controller = null
  loading.value = false
  data.value = null
  errorMessage.value = ''
}

async function loadStorageUsage() {
  if (!canView.value) {
    clearForMissingPermission()
    return
  }

  controller?.abort()
  const nextController = new AbortController()
  controller = nextController
  const sequence = ++requestSequence
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await opsAPI.getStorageUsage({ signal: nextController.signal })
    if (sequence !== requestSequence) return
    data.value = response
  } catch (error: unknown) {
    if (sequence !== requestSequence || isCancellationError(error)) return
    const message = error instanceof Error ? error.message : ''
    errorMessage.value = message || t('admin.ops.storage.loadFailed')
  } finally {
    if (sequence === requestSequence) {
      loading.value = false
      if (controller === nextController) controller = null
    }
  }
}

watch(
  canView,
  (allowed, previous) => {
    if (!allowed) {
      clearForMissingPermission()
      return
    }
    if (previous === false) void loadStorageUsage()
  },
  { immediate: true }
)

watch(
  () => props.refreshKey,
  () => {
    void loadStorageUsage()
  },
  { immediate: true }
)

onUnmounted(() => {
  requestSequence += 1
  controller?.abort()
  controller = null
})
</script>

<template>
  <section
    v-if="canView"
    class="card min-w-0 p-3"
    :aria-busy="loading"
    data-testid="storage-card"
  >
    <div class="flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-1">
        <h3 class="truncate text-[10px] font-bold uppercase tracking-wide text-[color:var(--nx-subtle)]">
          {{ t('admin.ops.storage.title') }}
        </h3>
        <HelpTooltip v-if="!props.fullscreen" :content="t('admin.ops.storage.tooltip')" />
      </div>
      <span
        v-if="loading"
        data-testid="storage-refreshing"
        class="inline-flex shrink-0 items-center gap-1 text-[color:var(--nx-muted)]"
      >
        <span class="h-2.5 w-2.5 animate-spin rounded-full border border-[color:var(--nx-border-strong)] border-t-[#ff5600]" />
        <span class="sr-only">{{ t('admin.ops.storage.refreshing') }}</span>
      </span>
    </div>

    <div class="mt-1 text-lg font-black text-[color:var(--nx-text)]">
      <span v-if="loading && !data" data-testid="storage-loading" role="status" aria-live="polite">
        {{ t('admin.ops.storage.loading') }}
      </span>
      <span v-else>{{ totalLabel }}</span>
    </div>

    <p
      v-if="errorMessage"
      role="alert"
      class="mt-2 break-words rounded-[4px] border border-[#f0c4a4] bg-[#fff4ed] px-2 py-1 text-[10px] leading-4 text-[color:var(--nx-warning)]"
    >
      <span class="font-semibold">{{ t('admin.ops.storage.loadFailed') }}:</span>
      {{ errorMessage }}
    </p>

    <p
      v-else-if="!loading && !visibleItems.length"
      class="mt-2 text-[10px] leading-4 text-[color:var(--nx-muted)]"
    >
      {{ t('admin.ops.storage.empty') }}
    </p>

    <div v-else-if="visibleItems.length" class="mt-2 divide-y divide-[color:var(--nx-border)]">
      <div
        v-for="item in visibleItems"
        :key="item.key"
        :data-testid="`storage-item-${item.key}`"
        class="min-w-0 py-1.5 first:pt-0 last:pb-0"
        :title="item.error || item.path || item.label"
      >
        <div class="flex min-w-0 items-center justify-between gap-2">
          <span class="min-w-0 truncate text-[10px] text-[color:var(--nx-muted)]">{{ item.label }}</span>
          <span class="shrink-0 font-mono text-[10px] font-semibold" :class="statusClass(item)">
            {{ statusLabel(item) }}
          </span>
        </div>
        <p
          v-if="item.error"
          class="mt-0.5 break-words text-[10px] leading-4 text-[color:var(--nx-warning)]"
        >
          {{ item.error }}
        </p>
      </div>
    </div>
  </section>
</template>
