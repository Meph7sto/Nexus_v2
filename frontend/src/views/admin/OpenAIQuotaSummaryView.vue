<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 border-b pb-5 md:flex-row md:items-end md:justify-between" style="border-color: var(--nx-border)">
        <div class="min-w-0">
          <h1 class="page-title break-words">{{ t('admin.openAIQuotaSummary.title') }}</h1>
          <p class="page-description max-w-3xl">{{ t('admin.openAIQuotaSummary.description') }}</p>
        </div>

        <div v-if="canView" class="flex flex-wrap items-end gap-2">
          <div>
            <label class="sr-only" for="quota-summary-group">{{ t('admin.openAIQuotaSummary.allGroups') }}</label>
            <select id="quota-summary-group" v-model="selectedGroup" data-test="group-filter" class="input w-48">
              <option value="">{{ t('admin.openAIQuotaSummary.allGroups') }}</option>
              <option value="ungrouped">{{ t('admin.openAIQuotaSummary.ungrouped') }}</option>
              <option v-for="group in groupOptions" :key="group.id" :value="String(group.id)">
                {{ group.name }}
              </option>
            </select>
          </div>

          <div>
            <label class="sr-only" for="quota-summary-type">{{ t('admin.openAIQuotaSummary.allTypes') }}</label>
            <select id="quota-summary-type" v-model="selectedType" data-test="type-filter" class="input w-40">
              <option value="">{{ t('admin.openAIQuotaSummary.allTypes') }}</option>
              <option v-for="type in typeOptions" :key="type" :value="type">
                {{ planTypeLabel(type) }}
              </option>
            </select>
          </div>

          <div class="inline-flex overflow-hidden rounded border" style="border-color: var(--nx-border)">
            <button
              type="button"
              class="px-3 py-2 text-sm font-medium transition-colors"
              :class="projectionMode === 'current' ? 'text-white' : 'hover:bg-[var(--nx-surface-muted)]'"
              :style="projectionMode === 'current' ? { background: 'var(--nx-text)' } : { color: 'var(--nx-text)' }"
              :aria-pressed="projectionMode === 'current'"
              @click="projectionMode = 'current'"
            >
              {{ t('admin.openAIQuotaSummary.current') }}
            </button>
            <button
              type="button"
              data-test="projection-mode-hours"
              class="border-l px-3 py-2 text-sm font-medium transition-colors hover:bg-[var(--nx-surface-muted)]"
              :class="projectionMode === 'hours' ? 'text-white' : ''"
              :style="projectionMode === 'hours' ? { background: 'var(--nx-text)' } : { color: 'var(--nx-text)', borderColor: 'var(--nx-border)' }"
              :aria-pressed="projectionMode === 'hours'"
              @click="projectionMode = 'hours'"
            >
              {{ t('admin.openAIQuotaSummary.hoursLater') }}
            </button>
            <button
              type="button"
              data-test="projection-mode-days"
              class="border-l px-3 py-2 text-sm font-medium transition-colors hover:bg-[var(--nx-surface-muted)]"
              :class="projectionMode === 'days' ? 'text-white' : ''"
              :style="projectionMode === 'days' ? { background: 'var(--nx-text)' } : { color: 'var(--nx-text)', borderColor: 'var(--nx-border)' }"
              :aria-pressed="projectionMode === 'days'"
              @click="projectionMode = 'days'"
            >
              {{ t('admin.openAIQuotaSummary.daysLater') }}
            </button>
          </div>

          <div>
            <label class="sr-only" for="quota-summary-projection-amount">{{ t('admin.openAIQuotaSummary.projectionAmount') }}</label>
            <input
              id="quota-summary-projection-amount"
              v-model.number="projectionAmount"
              data-test="projection-amount"
              type="number"
              min="1"
              class="input w-24"
              :disabled="projectionMode === 'current'"
            >
          </div>

          <button type="button" data-test="refresh" class="btn btn-primary" :disabled="loading" @click="loadSummary">
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <section v-if="!canView" class="border px-5 py-8 text-center" style="background: var(--nx-surface); border-color: var(--nx-border)">
        <h2 class="text-base font-semibold" style="color: var(--nx-text)">{{ t('admin.openAIQuotaSummary.noPermission') }}</h2>
      </section>

      <template v-else>
        <dl class="grid overflow-hidden border text-sm sm:grid-cols-2" style="background: var(--nx-surface); border-color: var(--nx-border)">
          <div class="px-4 py-3 sm:border-r" style="border-color: var(--nx-border)">
            <dt class="text-xs font-medium uppercase" style="color: var(--nx-muted)">
              {{ t('admin.openAIQuotaSummary.projection') }}
            </dt>
            <dd class="mt-1 break-words" style="color: var(--nx-text)">{{ formatDateTime(summary?.projection_at) }}</dd>
          </div>
          <div class="border-t px-4 py-3 sm:border-t-0" style="border-color: var(--nx-border)">
            <dt class="text-xs font-medium uppercase" style="color: var(--nx-muted)">
              {{ t('admin.openAIQuotaSummary.generated') }}
            </dt>
            <dd class="mt-1 break-words" style="color: var(--nx-text)">{{ formatDateTime(summary?.generated_at) }}</dd>
          </div>
        </dl>

        <section v-if="loading && !summary" class="border px-5 py-8 text-center text-sm" style="background: var(--nx-surface); border-color: var(--nx-border); color: var(--nx-muted)">
          {{ t('common.loading') }}
        </section>

        <section v-else-if="errorMessage" data-test="summary-error" role="alert" class="border px-5 py-5 text-sm" style="background: var(--nx-surface); border-color: var(--nx-danger); color: var(--nx-danger)">
          {{ errorMessage }}
        </section>

        <section v-else-if="!summaryGroups.length" class="border px-5 py-8 text-center text-sm" style="background: var(--nx-surface); border-color: var(--nx-border); color: var(--nx-muted)">
          {{ t('common.noData') }}
        </section>

        <section
          v-for="group in summaryGroups"
          v-else
          :key="group.group_id ?? 'ungrouped'"
          class="overflow-hidden border"
          style="background: var(--nx-surface); border-color: var(--nx-border)"
        >
          <div class="flex flex-wrap items-start justify-between gap-3 border-b px-4 py-3" style="border-color: var(--nx-border)">
            <div class="min-w-0">
              <h2 class="break-words text-base font-semibold" style="color: var(--nx-text)">
                {{ group.ungrouped ? t('admin.openAIQuotaSummary.ungrouped') : group.group_name }}
              </h2>
              <p class="mt-0.5 text-xs" style="color: var(--nx-muted)">
                {{ group.ungrouped ? t('admin.openAIQuotaSummary.ungrouped') : `#${group.group_id}` }}
              </p>
            </div>
            <span class="shrink-0 text-xs" style="color: var(--nx-muted)">
              {{ t('admin.openAIQuotaSummary.rows', { count: group.rows.length }) }}
            </span>
          </div>

          <div class="overflow-x-auto">
            <table class="table min-w-[1100px] text-sm">
              <thead>
                <tr>
                  <th scope="col">{{ t('admin.openAIQuotaSummary.table.type') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.included') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.errors') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.inactive') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.other') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.missing5h') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.missing7d') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.avg5h') }}</th>
                  <th scope="col" class="text-right">{{ t('admin.openAIQuotaSummary.table.avg7d') }}</th>
                  <th scope="col">{{ t('admin.openAIQuotaSummary.table.next5hRecovery') }}</th>
                  <th scope="col">{{ t('admin.openAIQuotaSummary.table.next7dRecovery') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in group.rows" :key="`${group.group_id ?? 'ungrouped'}-${row.account_type}`">
                  <td class="whitespace-nowrap font-medium">{{ planTypeLabel(row.account_type) }}</td>
                  <td class="whitespace-nowrap text-right">{{ row.included_count }}</td>
                  <td class="whitespace-nowrap text-right">{{ row.error_count }}</td>
                  <td class="whitespace-nowrap text-right">{{ row.inactive_count }}</td>
                  <td class="whitespace-nowrap text-right">{{ row.other_excluded_count }}</td>
                  <td class="whitespace-nowrap text-right" :title="t('admin.openAIQuotaSummary.partialSnapshot')">
                    {{ row.missing_5h_snapshot_count }}
                    <span v-if="row.missing_5h_snapshot_count > 0" class="ml-1 text-xs" style="color: var(--nx-warning)">
                      {{ t('admin.openAIQuotaSummary.partialSnapshot') }}
                    </span>
                  </td>
                  <td class="whitespace-nowrap text-right" :title="t('admin.openAIQuotaSummary.partialSnapshot')">
                    {{ row.missing_7d_snapshot_count }}
                    <span v-if="row.missing_7d_snapshot_count > 0" class="ml-1 text-xs" style="color: var(--nx-warning)">
                      {{ t('admin.openAIQuotaSummary.partialSnapshot') }}
                    </span>
                  </td>
                  <td class="whitespace-nowrap text-right">{{ formatPercent(row.avg_5h_remaining_percent) }}</td>
                  <td class="whitespace-nowrap text-right">{{ formatPercent(row.avg_7d_remaining_percent) }}</td>
                  <td class="min-w-48">
                    <template v-if="row.earliest_5h_recovery">
                      <div class="font-medium">{{ formatDateTime(row.earliest_5h_recovery.reset_at) }}</div>
                      <div class="mt-0.5 text-xs" style="color: var(--nx-muted)">
                        {{ formatPercent(row.earliest_5h_recovery.remaining_before_percent) }} -> {{ formatPercent(row.earliest_5h_recovery.remaining_after_percent) }}
                      </div>
                    </template>
                    <span v-else style="color: var(--nx-subtle)">-</span>
                  </td>
                  <td class="min-w-48">
                    <template v-if="row.earliest_7d_recovery">
                      <div class="font-medium">{{ formatDateTime(row.earliest_7d_recovery.reset_at) }}</div>
                      <div class="mt-0.5 text-xs" style="color: var(--nx-muted)">
                        {{ formatPercent(row.earliest_7d_recovery.remaining_before_percent) }} -> {{ formatPercent(row.earliest_7d_recovery.remaining_after_percent) }}
                      </div>
                    </template>
                    <span v-else style="color: var(--nx-subtle)">-</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { accountsAPI, groupsAPI } from '@/api/admin'
import type {
  OpenAIQuotaSummaryParams,
  OpenAIQuotaSummaryResponse,
} from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { AdminGroup } from '@/types'

type ProjectionMode = 'current' | 'hours' | 'days'

interface GroupFilterOption {
  id: number
  name: string
}

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const projectionMode = ref<ProjectionMode>('current')
const projectionAmount = ref(1)
const selectedGroup = ref('')
const selectedType = ref('')
const groups = ref<AdminGroup[]>([])
const summary = ref<OpenAIQuotaSummaryResponse | null>(null)
const loading = ref(true)
const errorMessage = ref('')

const canView = computed(() => authStore.canAdmin('accounts', 'view'))
const summaryGroups = computed(() => summary.value?.groups ?? [])
const commonPlanTypes = ['free', 'plus', 'pro', 'team', 'enterprise', 'unknown']

const groupOptions = computed<GroupFilterOption[]>(() => {
  const options: GroupFilterOption[] = []
  const seen = new Set<number>()

  for (const group of groups.value) {
    if (group.platform !== 'openai') continue
    options.push({ id: group.id, name: group.name })
    seen.add(group.id)
  }

  for (const group of summaryGroups.value) {
    if (group.ungrouped || group.group_id == null || seen.has(group.group_id)) continue
    options.push({ id: group.group_id, name: group.group_name })
    seen.add(group.group_id)
  }

  return options
})

const typeOptions = computed(() => {
  const options = new Set(commonPlanTypes)
  if (selectedType.value) {
    options.add(selectedType.value)
  }
  for (const group of summaryGroups.value) {
    for (const row of group.rows) {
      const type = row.account_type.trim()
      if (type) {
        options.add(type)
      }
    }
  }
  return Array.from(options).sort((left, right) => planTypeLabel(left).localeCompare(planTypeLabel(right)))
})

function formatPercent(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) {
    return '-'
  }
  return `${value.toFixed(1)}%`
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function planTypeLabel(type: string): string {
  const normalized = type.trim()
  if (!normalized) {
    return '-'
  }
  if (normalized.toLowerCase() === 'unknown') {
    return t('admin.openAIQuotaSummary.unknownPlan')
  }
  return normalized
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function projectionParams(): OpenAIQuotaSummaryParams {
  if (projectionMode.value === 'current') {
    return {}
  }

  const projectionAt = new Date()
  const amount = Math.max(1, Number(projectionAmount.value) || 1)
  if (projectionMode.value === 'hours') {
    projectionAt.setHours(projectionAt.getHours() + amount)
  } else {
    projectionAt.setDate(projectionAt.getDate() + amount)
  }
  return { projection_at: projectionAt.toISOString() }
}

function summaryParams(): OpenAIQuotaSummaryParams {
  const params = projectionParams()
  if (selectedGroup.value) {
    params.group = selectedGroup.value
  }
  if (selectedType.value) {
    params.type = selectedType.value
  }
  return params
}

function messageFromError(error: unknown): string {
  return error instanceof Error && error.message ? error.message : t('admin.openAIQuotaSummary.loadFailed')
}

async function loadGroups(): Promise<void> {
  try {
    groups.value = await groupsAPI.getAllIncludingInactive()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : String(error))
  }
}

async function loadSummary(): Promise<void> {
  if (!canView.value) {
    return
  }

  loading.value = true
  errorMessage.value = ''
  try {
    summary.value = await accountsAPI.getOpenAIQuotaSummary(summaryParams())
  } catch (error) {
    errorMessage.value = messageFromError(error)
    appStore.showError(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!canView.value) {
    return
  }
  void loadGroups()
  void loadSummary()
})
</script>
