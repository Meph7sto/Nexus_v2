<template>
  <AppLayout>
    <section class="leaderboard-page" aria-labelledby="leaderboard-title">
      <header class="leaderboard-header">
        <h1 id="leaderboard-title">{{ t('leaderboard.title') }}</h1>
        <DateRangePicker
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @change="onDateRangeChange"
        />
      </header>

      <div class="leaderboard-toolbar">
        <div class="metric-tabs" role="tablist" :aria-label="t('leaderboard.metric')">
          <button
            type="button"
            :class="['metric-tab', rankBy === 'cost' && 'metric-tab-active']"
            role="tab"
            :aria-selected="rankBy === 'cost'"
            @click="setRankBy('cost')"
          >
            <Icon name="dollar" size="sm" />
            {{ t('leaderboard.cost') }}
          </button>
          <button
            type="button"
            :class="['metric-tab', rankBy === 'tokens' && 'metric-tab-active']"
            role="tab"
            :aria-selected="rankBy === 'tokens'"
            @click="setRankBy('tokens')"
          >
            <Icon name="chartBar" size="sm" />
            {{ t('leaderboard.tokens') }}
          </button>
        </div>

        <button
          type="button"
          class="leaderboard-refresh"
          :disabled="loading"
          :title="t('leaderboard.refresh')"
          @click="loadRanking"
        >
          <Icon name="refresh" size="sm" />
          {{ t('leaderboard.refresh') }}
        </button>
      </div>

      <p v-if="loadError" class="leaderboard-error" role="alert" aria-live="polite">
        {{ loadError }}
      </p>

      <section class="leaderboard-table-card" :aria-busy="loading">
        <header class="leaderboard-table-header">
          <h2>{{ t('leaderboard.tableTitle') }}</h2>
          <div class="leaderboard-total">
            <span>{{ t('leaderboard.totalUsers') }}</span>
            <strong>{{ formatInteger(pagination.total) }}</strong>
          </div>
        </header>

        <div class="leaderboard-table-scroll">
          <table class="leaderboard-table">
            <thead>
              <tr>
                <th>{{ t('leaderboard.columns.rank') }}</th>
                <th>{{ t('leaderboard.columns.user') }}</th>
                <th>{{ t('leaderboard.columns.email') }}</th>
                <th class="number-column">{{ t('leaderboard.columns.requests') }}</th>
                <th class="number-column">{{ t('leaderboard.columns.tokens') }}</th>
                <th class="number-column">{{ t('leaderboard.columns.cost') }}</th>
              </tr>
            </thead>
            <tbody v-if="loading">
              <tr v-for="index in 8" :key="index">
                <td colspan="6"><span class="skeleton-row" /></td>
              </tr>
            </tbody>
            <tbody v-else-if="rows.length">
              <tr v-for="row in rows" :key="`${row.rank}-${row.user_id ?? row.email}-${row.nickname}`">
                <td>
                  <span :class="['rank-pill', row.rank <= 3 && 'rank-pill-top']">#{{ row.rank }}</span>
                </td>
                <td>
                  <div class="user-cell">
                    <span class="avatar-mark" aria-hidden="true">{{ avatarText(row.nickname) }}</span>
                    <span class="user-name" :title="row.nickname">{{ row.nickname || t('common.unknown') }}</span>
                  </div>
                </td>
                <td class="muted-cell" :title="row.email">{{ row.email || t('common.notAvailable') }}</td>
                <td class="number-column">{{ formatInteger(row.requests) }}</td>
                <td class="number-column">{{ formatInteger(row.total_tokens) }}</td>
                <td class="number-column">{{ formatCurrency(row.total_actual_cost) }}</td>
              </tr>
            </tbody>
            <tbody v-else>
              <tr>
                <td colspan="6">
                  <div class="empty-panel">
                    <Icon name="chartBar" size="lg" />
                    <p>{{ t('leaderboard.empty') }}</p>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          :page-size-options="[20, 50, 100]"
          :show-jump="true"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { usageAPI } from '@/api'
import type { UsageRankingItem, UsageRankingMetric } from '@/api/usage'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const MAX_PAGE_SIZE = 100
const MAX_RANGE_DAYS = 31
const DAY_MILLIS = 24 * 60 * 60 * 1000

const { t, locale } = useI18n()
const appStore = useAppStore()

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - DAY_MILLIS)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const clampPageSize = (value: number): number => Math.min(Math.max(Math.floor(value) || 20, 1), MAX_PAGE_SIZE)

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const rankBy = ref<UsageRankingMetric>('cost')
const rows = ref<UsageRankingItem[]>([])
const loading = ref(false)
const loadError = ref('')

const pagination = reactive({
  page: 1,
  page_size: clampPageSize(getPersistedPageSize(20)),
  total: 0,
})

const formatInteger = (value: number): string => {
  const safeValue = Number.isFinite(value) ? value : 0
  return new Intl.NumberFormat(String(locale.value)).format(safeValue)
}

const formatCurrency = (value: number): string => {
  const safeValue = Number.isFinite(value) ? value : 0
  return new Intl.NumberFormat(String(locale.value), {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 4,
    maximumFractionDigits: 6,
  }).format(safeValue)
}

const avatarText = (value: string): string => {
  const first = Array.from(value.trim())[0]
  return first ? first.toUpperCase() : '#'
}

const parseCalendarDate = (value: string): number | null => {
  const parts = value.split('-').map(Number)
  if (parts.length !== 3 || parts.some((part) => !Number.isInteger(part))) return null

  const [year, month, day] = parts
  const date = new Date(Date.UTC(year, month - 1, day))
  if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null
  return date.getTime()
}

const dateRangeError = (): string | null => {
  const start = parseCalendarDate(startDate.value)
  const end = parseCalendarDate(endDate.value)
  if (start === null || end === null || end < start) return t('leaderboard.invalidRange')
  if (Math.floor((end - start) / DAY_MILLIS) + 1 > MAX_RANGE_DAYS) return t('leaderboard.rangeTooLarge')
  return null
}

const displayError = (message: string) => {
  loadError.value = message
  appStore.showError(message)
}

const errorMessageFor = (error: unknown): string => {
  const errorLike = error as { status?: number; response?: { status?: number } }
  const status = errorLike?.status ?? errorLike?.response?.status
  if (status === 429) return t('leaderboard.rateLimited')
  if (status === 504) return t('leaderboard.timedOut')
  return extractApiErrorMessage(error, t('leaderboard.failedToLoad'))
}

const loadRanking = async () => {
  const rangeError = dateRangeError()
  if (rangeError) {
    displayError(rangeError)
    return
  }

  loading.value = true
  loadError.value = ''
  try {
    const response = await usageAPI.getRanking({
      rank_by: rankBy.value,
      start_date: startDate.value,
      end_date: endDate.value,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    rows.value = response.items
    pagination.total = response.total
    pagination.page = Math.max(response.page, 1)
    pagination.page_size = clampPageSize(response.page_size)
  } catch (error) {
    rows.value = []
    pagination.total = 0
    displayError(errorMessageFor(error))
  } finally {
    loading.value = false
  }
}

const setRankBy = (metric: UsageRankingMetric) => {
  if (rankBy.value === metric) return
  rankBy.value = metric
  pagination.page = 1
  void loadRanking()
}

const onDateRangeChange = (range: { startDate: string; endDate: string }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  pagination.page = 1
  void loadRanking()
}

const handlePageChange = (page: number) => {
  pagination.page = Math.max(page, 1)
  void loadRanking()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = clampPageSize(pageSize)
  pagination.page = 1
  void loadRanking()
}

onMounted(() => {
  void loadRanking()
})
</script>

<style scoped>
.leaderboard-page {
  max-width: 1180px;
  margin: 0 auto;
}

.leaderboard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--nx-border);
}

.leaderboard-header h1 {
  color: var(--nx-text);
  font-size: 32px;
  font-weight: 500;
  line-height: 1.2;
}

.leaderboard-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.metric-tabs {
  display: inline-flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--nx-border);
  border-radius: 6px;
  background: var(--nx-surface-muted);
}

.metric-tab,
.leaderboard-refresh {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 14px;
  border: 1px solid transparent;
  border-radius: 4px;
  color: var(--nx-muted);
  font-size: 14px;
  font-weight: 500;
  transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;
}

.metric-tab:hover,
.leaderboard-refresh:hover:not(:disabled) {
  color: var(--nx-text);
}

.metric-tab-active {
  border-color: var(--nx-border);
  background: var(--nx-surface);
  color: var(--nx-text);
}

.leaderboard-refresh {
  background: var(--nx-text);
  color: var(--nx-surface);
}

.leaderboard-refresh:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.leaderboard-error {
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid rgba(190, 45, 45, 0.35);
  border-radius: 4px;
  background: rgba(190, 45, 45, 0.07);
  color: #9f1f1f;
  font-size: 14px;
}

.leaderboard-table-card {
  overflow: hidden;
  border: 1px solid var(--nx-border);
  border-radius: 8px;
  background: var(--nx-surface);
}

.leaderboard-table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--nx-border);
}

.leaderboard-table-header h2 {
  color: var(--nx-text);
  font-size: 18px;
  font-weight: 600;
}

.leaderboard-total {
  display: flex;
  align-items: baseline;
  gap: 8px;
  color: var(--nx-muted);
  font-size: 13px;
}

.leaderboard-total strong {
  color: var(--nx-text);
  font-size: 22px;
  font-variant-numeric: tabular-nums;
}

.leaderboard-table-scroll {
  overflow-x: auto;
}

.leaderboard-table {
  width: 100%;
  min-width: 760px;
  border-collapse: collapse;
  font-size: 14px;
}

.leaderboard-table th {
  padding: 12px 16px;
  border-bottom: 1px solid var(--nx-border);
  background: var(--nx-bg);
  color: var(--nx-muted);
  font-size: 12px;
  font-weight: 600;
  text-align: left;
  text-transform: uppercase;
}

.leaderboard-table td {
  padding: 14px 16px;
  border-bottom: 1px solid var(--nx-border);
  color: var(--nx-text);
  vertical-align: middle;
}

.leaderboard-table tbody tr:last-child td {
  border-bottom: 0;
}

.leaderboard-table tbody tr:hover {
  background: var(--nx-bg);
}

.number-column {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rank-pill {
  display: inline-flex;
  min-width: 48px;
  align-items: center;
  justify-content: center;
  padding: 4px 8px;
  border: 1px solid var(--nx-border);
  border-radius: 4px;
  background: var(--nx-surface-muted);
  color: var(--nx-muted);
  font-variant-numeric: tabular-nums;
}

.rank-pill-top {
  border-color: rgba(255, 86, 0, 0.28);
  background: rgba(255, 86, 0, 0.1);
  color: var(--nx-accent);
}

.user-cell {
  display: flex;
  min-width: 180px;
  align-items: center;
  gap: 10px;
}

.avatar-mark {
  display: inline-flex;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  background: var(--nx-text);
  color: var(--nx-surface);
  font-size: 13px;
  font-weight: 600;
}

.user-name,
.muted-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-name {
  min-width: 0;
}

.muted-cell {
  max-width: 260px;
  color: var(--nx-muted);
}

.skeleton-row {
  display: block;
  height: 20px;
  border-radius: 4px;
  background: var(--nx-surface-muted);
  animation: leaderboard-skeleton 1.1s ease-in-out infinite;
}

.empty-panel {
  display: flex;
  min-height: 180px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--nx-subtle);
}

@keyframes leaderboard-skeleton {
  0%,
  100% {
    opacity: 0.55;
  }

  50% {
    opacity: 1;
  }
}

@media (max-width: 768px) {
  .leaderboard-header,
  .leaderboard-toolbar,
  .leaderboard-table-header {
    align-items: stretch;
    flex-direction: column;
  }

  .metric-tabs,
  .leaderboard-refresh {
    width: 100%;
  }

  .metric-tab {
    flex: 1;
  }
}
</style>
