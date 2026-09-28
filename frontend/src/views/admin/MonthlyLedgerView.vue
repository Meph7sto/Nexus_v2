<template>
  <AppLayout>
    <div class="ledger-page space-y-5">
      <section class="ledger-toolbar" aria-label="Monthly ledger filters">
        <div class="flex min-w-0 flex-wrap items-end gap-3">
          <div>
            <label for="ledger-month" class="input-label">{{ t('admin.monthlyLedger.month') }}</label>
            <div class="month-control">
              <button
                type="button"
                class="month-arrow"
                :title="t('admin.monthlyLedger.previousMonth')"
                @click="shiftMonth(-1)"
              >
                <Icon name="chevronLeft" size="sm" />
              </button>
              <input
                id="ledger-month"
                v-model="selectedMonth"
                data-test="ledger-month"
                type="month"
                :max="currentMonth || undefined"
                class="month-input"
                @change="changeMonth"
              />
              <button
                type="button"
                class="month-arrow"
                :title="t('admin.monthlyLedger.nextMonth')"
                :disabled="!selectedMonth || !currentMonth || selectedMonth >= currentMonth"
                @click="shiftMonth(1)"
              >
                <Icon name="chevronRight" size="sm" />
              </button>
            </div>
          </div>

          <template v-if="activeTab === 'relay'">
          <div class="min-w-[220px] flex-1 sm:max-w-sm">
            <label for="ledger-search" class="input-label">{{ t('common.search') }}</label>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nx-subtle)]" />
              <input
                id="ledger-search"
                v-model.trim="searchQuery"
                class="input w-full pl-9"
                :placeholder="t('admin.monthlyLedger.searchPlaceholder')"
                @keyup.enter="applyFilters"
              />
            </div>
          </div>

          <div class="w-44">
            <label for="ledger-status" class="input-label">{{ t('admin.monthlyLedger.statusFilter') }}</label>
            <select id="ledger-status" v-model="statusFilter" class="input w-full" @change="applyFilters">
              <option value="">{{ t('admin.monthlyLedger.allStatuses') }}</option>
              <option v-for="status in ledgerStatuses" :key="status" :value="status">
                {{ t(`admin.monthlyLedger.status.${status}`) }}
              </option>
            </select>
          </div>

          <button type="button" class="btn btn-secondary" @click="applyFilters">
            <Icon name="search" size="sm" />
            {{ t('common.search') }}
          </button>
          <button type="button" class="btn btn-secondary" data-test="open-email-notifications" @click="showEmailDialog = true">
            <Icon name="mail" size="sm" />
            {{ t('admin.monthlyLedger.email.title') }}
          </button>
          <button
            type="button"
            class="btn btn-ghost px-2.5"
            :title="t('admin.monthlyLedger.refresh')"
            :disabled="loading"
            @click="loadLedger"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
          </template>
        </div>

        <p v-if="activeTab === 'relay' && selectedMonth && selectedMonth === currentMonth" class="open-month-notice">
          <Icon name="infoCircle" size="sm" />
          {{ t('admin.monthlyLedger.currentMonthHint') }}
        </p>
      </section>

      <MonthlyLedgerOverview :month="selectedMonth" :revision="overviewRevision" />
      <div class="flex gap-2 border-b border-[var(--nx-border)] pb-2" role="tablist" :aria-label="t('admin.monthlyLedger.title')">
        <button v-for="tab in (['relay', 'other'] as const)" :key="tab" type="button" role="tab" :aria-selected="activeTab === tab" :class="['btn', activeTab === tab ? 'btn-primary' : 'btn-ghost']" @click="activeTab = tab">{{ t(`admin.monthlyLedger.income.${tab}`) }}</button>
      </div>
      <MonthlyLedgerIncome v-if="activeTab === 'other' && selectedMonth" :month="selectedMonth" @changed="overviewRevision++" />
      <template v-if="activeTab === 'relay'">
      <section class="summary-grid" aria-label="Monthly ledger summary">
        <div
          v-for="metric in summaryMetrics"
          :key="metric.key"
          class="summary-card"
          :class="metric.featured ? 'summary-card-featured' : ''"
          :data-test="`summary-${metric.key}`"
        >
          <span class="summary-icon" :class="metric.iconClass">
            <Icon :name="metric.icon" size="md" :stroke-width="2" />
          </span>
          <div class="min-w-0">
            <span class="summary-label">{{ metric.label }}</span>
            <strong class="summary-value" :class="metric.valueClass">{{ formatUSD(metric.value) }}</strong>
            <span class="summary-detail">{{ metric.detail }}</span>
          </div>
        </div>
      </section>

      <section v-if="canUpdate" class="bulk-action-bar" data-test="ledger-bulk-actions">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span v-if="allResultsSelected" class="text-sm font-semibold text-[var(--nx-text)]">
            {{ t('admin.monthlyLedger.bulk.selectedAll', { count: selIds.length }) }}
          </span>
          <span v-else class="text-sm font-semibold text-[var(--nx-text)]">
            {{ t('admin.monthlyLedger.bulk.selected', { count: selIds.length }) }}
          </span>
          <button
            v-if="rows.length > 0 && !allVisibleSelected"
            type="button"
            class="bulk-link"
            data-test="select-current-page"
            @click="selectCurrentPage"
          >
            {{ t('admin.monthlyLedger.bulk.selectCurrentPage') }}
          </button>
          <button
            v-if="!allResultsSelected && total > selIds.length"
            type="button"
            class="bulk-link"
            data-test="select-all-results"
            :disabled="selectingAllResults || loading"
            @click="handleSelectAllResults"
          >
            {{ selectingAllResults
              ? t('admin.monthlyLedger.bulk.selectingAll')
              : t('admin.monthlyLedger.bulk.selectAllResults', { count: total }) }}
          </button>
          <button
            v-if="selIds.length > 0"
            type="button"
            class="bulk-link"
            data-test="clear-selection"
            @click="clearSelection"
          >
            {{ t('admin.monthlyLedger.bulk.clear') }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-primary shrink-0"
          data-test="open-bulk-multiplier"
          :disabled="selIds.length === 0"
          @click="openBulkMultiplier"
        >
          <Icon name="edit" size="sm" />
          {{ t('admin.monthlyLedger.bulk.action') }}
        </button>
      </section>

      <section class="ledger-table-shell">
        <div class="overflow-x-auto">
          <table class="ledger-table">
            <thead>
              <tr>
                <th v-if="canUpdate" class="selection-cell text-center">
                  <input
                    type="checkbox"
                    class="selection-checkbox"
                    data-test="select-visible"
                    :checked="allVisibleSelected"
                    :aria-label="t('admin.monthlyLedger.bulk.selectCurrentPage')"
                    @change="toggleSelectAllVisible"
                  />
                </th>
                <th class="text-left">{{ t('admin.monthlyLedger.columns.user') }}</th>
                <th v-for="column in sortableColumns" :key="column.key" class="text-right">
                  <button type="button" class="sort-button" @click="changeSort(column.key)">
                    {{ t(column.label) }}
                    <Icon
                      v-if="sortBy === column.key"
                      :name="sortOrder === 'asc' ? 'chevronUp' : 'chevronDown'"
                      size="xs"
                    />
                  </button>
                </th>
                <th class="text-center">{{ t('admin.monthlyLedger.columns.status') }}</th>
                <th class="text-left">{{ t('admin.monthlyLedger.columns.lastPaidAt') }}</th>
                <th class="text-right">{{ t('admin.monthlyLedger.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td :colspan="canUpdate ? 10 : 9" class="empty-row">
                  <span class="inline-flex items-center gap-2"><Icon name="refresh" size="sm" class="animate-spin" />{{ t('common.loading') }}</span>
                </td>
              </tr>
              <tr v-else-if="rows.length === 0">
                <td :colspan="canUpdate ? 10 : 9" class="empty-row">{{ t('admin.monthlyLedger.noData') }}</td>
              </tr>
              <tr v-for="row in rows" v-else :key="row.user_id" :class="isSelected(row.user_id) ? 'ledger-row-selected' : ''">
                <td v-if="canUpdate" class="selection-cell text-center">
                  <input
                    type="checkbox"
                    class="selection-checkbox"
                    :data-test="`select-user-${row.user_id}`"
                    :checked="isSelected(row.user_id)"
                    :aria-label="row.email"
                    @change="toggleSelection(row.user_id)"
                  />
                </td>
                <td>
                  <div class="max-w-[260px]">
                    <div class="flex items-center gap-2">
                      <span class="truncate font-medium text-[var(--nx-text)]" :title="row.email">{{ row.email }}</span>
                      <span v-if="row.deleted" class="deleted-badge">{{ t('admin.monthlyLedger.deletedUser') }}</span>
                    </div>
                    <div class="mt-0.5 truncate text-xs text-[var(--nx-subtle)]">
                      {{ row.username || `#${row.user_id}` }}<span v-if="row.username"> · #{{ row.user_id }}</span>
                    </div>
                  </div>
                </td>
                <td class="money-cell">{{ formatUSD(row.usage_amount) }}</td>
                <td class="money-cell">
                  <button
                    v-if="canUpdate"
                    type="button"
                    class="multiplier-button"
                    :data-test="`edit-multiplier-${row.user_id}`"
                    @click="openMultiplier(row)"
                  >
                    {{ formatMultiplier(row.multiplier) }}×
                    <Icon name="edit" size="xs" />
                  </button>
                  <span v-else>{{ formatMultiplier(row.multiplier) }}×</span>
                </td>
                <td class="money-cell font-semibold text-[var(--nx-text)]">{{ formatUSD(row.receivable_amount) }}</td>
                <td class="money-cell text-[var(--nx-success)]">{{ formatUSD(row.paid_amount) }}</td>
                <td class="money-cell">
                  <span v-if="row.outstanding_amount > 0" class="text-[var(--nx-warning)]">{{ formatUSD(row.outstanding_amount) }}</span>
                  <span v-else-if="row.overpaid_amount > 0" class="text-[var(--nx-success)]">+{{ formatUSD(row.overpaid_amount) }}</span>
                  <span v-else class="text-[var(--nx-subtle)]">$0.00</span>
                </td>
                <td class="text-center">
                  <span class="status-badge" :class="statusClass(row.status)">
                    {{ t(`admin.monthlyLedger.status.${row.status}`) }}
                  </span>
                </td>
                <td class="whitespace-nowrap text-sm text-[var(--nx-muted)]">
                  {{ row.last_paid_at ? formatDateTime(row.last_paid_at) : '—' }}
                </td>
                <td>
                  <div class="flex justify-end gap-1">
                    <button
                      type="button"
                      class="icon-command"
                      :data-test="`payments-${row.user_id}`"
                      :title="t('admin.monthlyLedger.payments.title')"
                      @click="openPayments(row, false)"
                    >
                      <Icon name="eye" size="sm" />
                      <span v-if="row.payment_count" class="payment-count">{{ row.payment_count }}</span>
                    </button>
                    <button
                      v-if="canCreate"
                      type="button"
                      class="icon-command icon-command-primary"
                      data-test="add-payment"
                      :title="canRecordPayments ? t('admin.monthlyLedger.payments.add') : t('admin.monthlyLedger.payments.openMonthDisabled')"
                      :disabled="!canRecordPayments"
                      @click="openPayments(row, true)"
                    >
                      <Icon name="plus" size="sm" />
                    </button>
                    <button
                      v-if="canCreate"
                      type="button"
                      class="icon-command"
                      :data-test="`send-email-${row.user_id}`"
                      :title="t('admin.monthlyLedger.email.sendTitle')"
                      :aria-label="`${t('admin.monthlyLedger.email.sendTitle')} ${row.email}`"
                      :disabled="row.deleted || !row.email"
                      @click="openSendEmail(row)"
                    >
                      <Icon name="mail" size="sm" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="total > 0"
          :page="page"
          :total="total"
          :page-size="pageSize"
          @update:page="changePage"
          @update:page-size="changePageSize"
        />
      </section>
      </template>
    </div>
  </AppLayout>
  <MonthlyLedgerEmailDialog :show="showEmailDialog" @close="showEmailDialog = false" />
  <MonthlyLedgerSendEmailDialog :show="showSendEmailDialog" :initial-month="selectedMonth" :recipient="emailRow" @close="showSendEmailDialog = false" />

  <BaseDialog
    :show="showMultiplierDialog"
    :title="multiplierDialogTitle"
    width="narrow"
    @close="closeMultiplier"
  >
    <div v-if="editingMultiplierRow || multiplierMode === 'batch'" class="space-y-5">
      <div v-if="editingMultiplierRow" class="amount-comparison">
        <div>
          <span>{{ t('admin.monthlyLedger.multiplier.rawUsage') }}</span>
          <strong>{{ formatUSD(editingMultiplierRow.usage_amount) }}</strong>
        </div>
        <Icon name="arrowRight" size="sm" />
        <div class="text-right">
          <span>{{ t('admin.monthlyLedger.multiplier.preview') }}</span>
          <strong>{{ formatUSD(multiplierPreview) }}</strong>
        </div>
      </div>

      <div v-else class="batch-scope">
        <Icon name="users" size="md" />
        <span>{{ t('admin.monthlyLedger.bulk.scope', { month: selectedMonth, count: batchMultiplierUserIDs.length }) }}</span>
      </div>

      <div>
        <label class="input-label">{{ t('admin.monthlyLedger.columns.multiplier') }}</label>
        <div class="grid grid-cols-4 gap-2">
          <button
            v-for="preset in multiplierPresets"
            :key="preset"
            type="button"
            class="multiplier-preset"
            :class="Number(multiplierDraft) === preset ? 'multiplier-preset-active' : ''"
            :data-test="`multiplier-preset-${preset}`"
            @click="multiplierDraft = String(preset)"
          >
            {{ formatMultiplier(preset) }}×
          </button>
        </div>
        <label for="custom-multiplier" class="mt-3 block text-xs font-medium text-[var(--nx-muted)]">
          {{ t('admin.monthlyLedger.multiplier.custom') }}
        </label>
        <input
          id="custom-multiplier"
          v-model="multiplierDraft"
          type="number"
          min="0"
          step="0.0001"
          class="input mt-1 w-full"
        />
        <p v-if="multiplierError" class="mt-1 text-xs text-red-600">{{ multiplierError }}</p>
      </div>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="closeMultiplier">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-primary" data-test="save-multiplier" :disabled="savingMultiplier" @click="saveMultiplier">
        {{ t('common.save') }}
      </button>
    </template>
  </BaseDialog>

  <BaseDialog
    :show="showPaymentsDialog"
    :title="`${t('admin.monthlyLedger.payments.title')} · ${activeRow?.email || ''}`"
    width="wide"
    @close="closePayments"
  >
    <div class="mb-5 flex items-center justify-between gap-3 border-b border-[var(--nx-border)] pb-4">
      <label for="manual-settlement" class="text-sm font-medium text-[var(--nx-text)]">{{ t('admin.monthlyLedger.manuallySettled') }}</label>
      <Toggle
        id="manual-settlement"
        data-test="manual-settlement"
        :model-value="activeRow?.manually_settled ?? false"
        :disabled="!canUpdate || !canRecordPayments || savingSettlement"
        :aria-label="t('admin.monthlyLedger.manuallySettled')"
        :aria-busy="savingSettlement"
        class="disabled:cursor-not-allowed disabled:opacity-50"
        @update:model-value="saveSettlement"
      />
    </div>
    <div class="grid min-h-[340px] gap-6 md:grid-cols-[minmax(0,1fr)_minmax(280px,0.72fr)]">
      <div class="min-w-0">
        <div class="mb-3 flex items-center justify-between">
          <span class="text-xs font-semibold uppercase text-[var(--nx-muted)]">{{ selectedMonth }}</span>
          <span class="text-sm font-semibold text-[var(--nx-text)]">{{ formatUSD(activeRow?.paid_amount || 0) }}</span>
        </div>
        <div v-if="paymentsLoading" class="py-12 text-center text-sm text-[var(--nx-subtle)]">{{ t('common.loading') }}</div>
        <div v-else-if="payments.length === 0" class="payment-empty">{{ t('admin.monthlyLedger.noPayments') }}</div>
        <div v-else class="divide-y divide-[var(--nx-border)] border-y border-[var(--nx-border)]">
          <article v-for="payment in payments" :key="payment.id" class="payment-record">
            <div class="min-w-0">
              <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <strong class="text-base text-[var(--nx-text)]">{{ formatUSD(payment.amount) }}</strong>
                <span class="text-xs text-[var(--nx-muted)]">{{ formatDateTime(payment.paid_at) }}</span>
              </div>
              <p v-if="payment.note" class="mt-1 break-words text-sm text-[var(--nx-muted)]">{{ payment.note }}</p>
              <p class="mt-1 text-xs text-[var(--nx-subtle)]">
                {{ t('admin.monthlyLedger.payments.operator') }}: {{ payment.updated_by_email || payment.created_by_email || `#${payment.updated_by || payment.created_by}` }}
              </p>
            </div>
            <div class="flex gap-1">
              <button
                v-if="canUpdate"
                type="button"
                class="icon-command"
                :data-test="`edit-payment-${payment.id}`"
                :title="t('common.edit')"
                @click="editPayment(payment)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button v-if="canDelete" type="button" class="icon-command icon-command-danger" :title="t('common.delete')" @click="requestDeletePayment(payment)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </article>
        </div>
      </div>

      <form v-if="canRecordPayments && (canCreate || editingPayment)" class="payment-form" @submit.prevent="savePayment">
        <div class="flex items-center justify-between">
          <h4 class="text-sm font-semibold text-[var(--nx-text)]">
            {{ editingPayment ? t('admin.monthlyLedger.payments.edit') : t('admin.monthlyLedger.payments.add') }}
          </h4>
          <button v-if="editingPayment && canCreate" type="button" class="text-xs text-[var(--nx-accent)]" @click="resetPaymentForm">
            {{ t('admin.monthlyLedger.payments.add') }}
          </button>
        </div>
        <div>
          <label for="payment-amount" class="input-label">{{ t('admin.monthlyLedger.payments.amount') }}</label>
          <input id="payment-amount" v-model.number="paymentForm.amount" data-test="payment-amount" type="number" min="0.01" step="0.01" class="input w-full" />
        </div>
        <div>
          <label for="payment-time" class="input-label">{{ t('admin.monthlyLedger.payments.paidAt') }}</label>
          <input
            id="payment-time"
            v-model="paymentForm.paidAt"
            data-test="payment-time"
            type="datetime-local"
            :max="nowForInput"
            class="input w-full"
            @focus="refreshPaymentTimeLimit"
          />
        </div>
        <div>
          <label for="payment-note" class="input-label">{{ t('admin.monthlyLedger.payments.note') }}</label>
          <textarea id="payment-note" v-model="paymentForm.note" rows="3" maxlength="500" class="input w-full resize-none" :placeholder="t('admin.monthlyLedger.payments.notePlaceholder')"></textarea>
        </div>
        <p v-if="paymentError" class="text-xs text-red-600">{{ paymentError }}</p>
        <button type="submit" class="btn btn-primary w-full justify-center" data-test="save-payment" :disabled="savingPayment">
          {{ t('common.save') }}
        </button>
      </form>
      <div v-else-if="!canRecordPayments" class="payment-form flex items-center text-sm text-[var(--nx-muted)]">
        {{ t('admin.monthlyLedger.payments.openMonthDisabled') }}
      </div>
    </div>
  </BaseDialog>

  <ConfirmDialog
    :show="Boolean(deletingPayment)"
    :title="t('admin.monthlyLedger.payments.deleteTitle')"
    :message="t('admin.monthlyLedger.payments.deleteConfirm', { amount: formatUSD(deletingPayment?.amount || 0) })"
    :danger="true"
    @confirm="confirmDeletePayment"
    @cancel="deletingPayment = null"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  MonthlyLedgerPayment,
  MonthlyLedgerRow,
  MonthlyLedgerStatus,
  MonthlyLedgerSummary,
} from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useTableSelection } from '@/composables/useTableSelection'
import { fetchAllPaginatedIDs } from '@/utils/paginatedSelection'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MonthlyLedgerEmailDialog from './MonthlyLedgerEmailDialog.vue'
import MonthlyLedgerSendEmailDialog from './MonthlyLedgerSendEmailDialog.vue'
import MonthlyLedgerOverview from './MonthlyLedgerOverview.vue'
import MonthlyLedgerIncome from './MonthlyLedgerIncome.vue'
import Toggle from '@/components/common/Toggle.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const emptySummary = (): MonthlyLedgerSummary => ({
  usage_amount: 0,
  receivable_amount: 0,
  paid_amount: 0,
  outstanding_amount: 0,
  overpaid_amount: 0,
  user_count: 0,
  unpaid_count: 0,
  partial_count: 0,
  settled_count: 0,
  overpaid_count: 0,
  waived_count: 0,
})

const rows = ref<MonthlyLedgerRow[]>([])
const summary = ref<MonthlyLedgerSummary>(emptySummary())
const loading = ref(false)
const showEmailDialog = ref(false)
const showSendEmailDialog = ref(false)
const emailRow = ref<MonthlyLedgerRow | null>(null)
function openSendEmail(row: MonthlyLedgerRow) {
  emailRow.value = { ...row }
  showSendEmailDialog.value = true
}
const activeTab = ref<'relay' | 'other'>('relay')
const overviewRevision = ref(0)
const selectedMonth = ref('')
const currentMonth = ref('')
const canRecordPayments = ref(false)
const searchQuery = ref('')
const statusFilter = ref<MonthlyLedgerStatus | ''>('')
const appliedSearchQuery = ref('')
const appliedStatusFilter = ref<MonthlyLedgerStatus | ''>('')
const loadedFilterSnapshot = ref<{ month: string; q?: string; status: MonthlyLedgerStatus | '' }>({
  month: '',
  status: '',
})
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const sortBy = ref<'user' | 'usage_amount' | 'multiplier' | 'receivable_amount' | 'paid_amount' | 'outstanding_amount' | 'last_paid_at'>('outstanding_amount')
const sortOrder = ref<'asc' | 'desc'>('desc')
let listRequestSequence = 0

const canCreate = computed(() => authStore.canAdmin('monthly_ledger', 'create'))
const canUpdate = computed(() => authStore.canAdmin('monthly_ledger', 'update'))
const canDelete = computed(() => authStore.canAdmin('monthly_ledger', 'delete'))

const {
  selectedSet,
  selectedIds: selIds,
  allVisibleSelected,
  isSelected,
  setSelectedIds,
  toggle: toggleSelection,
  clear: clearSelectedIds,
  toggleVisible,
  selectVisible: selectCurrentPage,
} = useTableSelection<MonthlyLedgerRow>({
  rows,
  getId: row => row.user_id,
})
const selectingAllResults = ref(false)
const selectedAllResultIDs = ref<Set<number> | null>(null)
const selectionRequestVersion = ref(0)
const allResultsSelected = computed(() => {
  const snapshot = selectedAllResultIDs.value
  if (!snapshot || snapshot.size === 0 || snapshot.size !== selectedSet.value.size) return false
  return Array.from(snapshot).every(userID => selectedSet.value.has(userID))
})

const clearSelection = () => {
  selectionRequestVersion.value++
  selectingAllResults.value = false
  selectedAllResultIDs.value = null
  clearSelectedIds()
}

const toggleSelectAllVisible = (event: Event) => {
  toggleVisible((event.target as HTMLInputElement).checked)
}

const handleSelectAllResults = async () => {
  if (selectingAllResults.value || loading.value || total.value === 0) return
  const requestVersion = ++selectionRequestVersion.value
  const filterSnapshot = { ...loadedFilterSnapshot.value }
  selectingAllResults.value = true
  try {
    const ids = await fetchAllPaginatedIDs(
      (targetPage, targetPageSize) => adminAPI.monthlyLedger.list({
        ...filterSnapshot,
        page: targetPage,
        page_size: targetPageSize,
        sort_by: 'user',
        sort_order: 'asc',
      }),
      row => row.user_id,
      { pageSize: 200, incompleteError: 'Monthly ledger selection result is incomplete' }
    )
    if (requestVersion !== selectionRequestVersion.value) return
    setSelectedIds(ids)
    selectedAllResultIDs.value = new Set(ids)
  } catch (error) {
    if (requestVersion !== selectionRequestVersion.value) return
    console.error('Failed to select all monthly ledger results:', error)
    appStore.showError(t('admin.monthlyLedger.bulk.selectAllFailed'))
  } finally {
    if (requestVersion === selectionRequestVersion.value) selectingAllResults.value = false
  }
}

const ledgerStatuses: MonthlyLedgerStatus[] = ['unpaid', 'partial', 'settled', 'overpaid', 'waived']
const sortableColumns = [
  { key: 'usage_amount' as const, label: 'admin.monthlyLedger.columns.usage' },
  { key: 'multiplier' as const, label: 'admin.monthlyLedger.columns.multiplier' },
  { key: 'receivable_amount' as const, label: 'admin.monthlyLedger.columns.receivable' },
  { key: 'paid_amount' as const, label: 'admin.monthlyLedger.columns.paid' },
  { key: 'outstanding_amount' as const, label: 'admin.monthlyLedger.columns.balance' },
]

const summaryMetrics = computed(() => [
  { key: 'usage', label: t('admin.monthlyLedger.summary.usage'), value: summary.value.usage_amount, detail: t('admin.monthlyLedger.summary.users', { count: summary.value.user_count }), icon: 'chart' as const, iconClass: 'summary-icon-neutral' },
  { key: 'receivable', label: t('admin.monthlyLedger.summary.receivable'), value: summary.value.receivable_amount, detail: t('admin.monthlyLedger.summary.waivedUsers', { count: summary.value.waived_count }), icon: 'creditCard' as const, iconClass: 'summary-icon-receivable', featured: true },
  { key: 'paid', label: t('admin.monthlyLedger.summary.paid'), value: summary.value.paid_amount, detail: t('admin.monthlyLedger.summary.settledUsers', { count: summary.value.settled_count }), icon: 'checkCircle' as const, iconClass: 'summary-icon-paid', valueClass: 'text-[var(--nx-success)]', featured: true },
  { key: 'outstanding', label: t('admin.monthlyLedger.summary.outstanding'), value: summary.value.outstanding_amount, detail: t('admin.monthlyLedger.summary.outstandingUsers', { count: summary.value.unpaid_count + summary.value.partial_count }), icon: 'clock' as const, iconClass: 'summary-icon-outstanding', valueClass: 'text-[var(--nx-warning)]' },
  { key: 'overpaid', label: t('admin.monthlyLedger.summary.overpaid'), value: summary.value.overpaid_amount, detail: t('admin.monthlyLedger.summary.overpaidUsers', { count: summary.value.overpaid_count }), icon: 'arrowUp' as const, iconClass: 'summary-icon-overpaid' },
])

const formatUSD = (value: number) => `$${Number(value || 0).toFixed(2)}`
const formatMultiplier = (value: number) => Number(value).toLocaleString(undefined, { maximumFractionDigits: 4 })
const formatDateTime = (value: string) => new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
  year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
}).format(new Date(value))

const statusClass = (status: MonthlyLedgerStatus) => ({
  unpaid: 'status-unpaid',
  partial: 'status-partial',
  settled: 'status-settled',
  overpaid: 'status-overpaid',
  waived: 'status-waived',
}[status])

const errorMessage = (error: unknown, fallback: string) => {
  const candidate = error as { response?: { data?: { message?: string } }; message?: string }
  return candidate?.response?.data?.message || candidate?.message || fallback
}

const loadLedger = async () => {
  const sequence = ++listRequestSequence
  const requestParams = {
    ...(selectedMonth.value ? { month: selectedMonth.value } : {}),
    q: appliedSearchQuery.value || undefined,
    status: appliedStatusFilter.value,
    page: page.value,
    page_size: pageSize.value,
    sort_by: sortBy.value,
    sort_order: sortOrder.value,
  }
  loading.value = true
  try {
    const result = await adminAPI.monthlyLedger.list(requestParams)
    if (sequence !== listRequestSequence) return
    rows.value = result.items || []
    summary.value = result.summary || emptySummary()
    total.value = result.total || 0
    page.value = result.page || 1
    pageSize.value = result.page_size || pageSize.value
    selectedMonth.value = result.month
    currentMonth.value = result.current_month
    canRecordPayments.value = result.can_record_payments
    overviewRevision.value++
    loadedFilterSnapshot.value = {
      month: result.month,
      q: requestParams.q,
      status: requestParams.status,
    }
  } catch (error) {
    if (sequence !== listRequestSequence) return
    rows.value = []
    summary.value = emptySummary()
    appStore.showError(errorMessage(error, t('admin.monthlyLedger.loadFailed')))
  } finally {
    if (sequence === listRequestSequence) loading.value = false
  }
}

const applyFilters = () => {
  clearSelection()
  appliedSearchQuery.value = searchQuery.value
  appliedStatusFilter.value = statusFilter.value
  page.value = 1
  loadLedger()
}

const changeMonth = () => {
  clearSelection()
  appliedSearchQuery.value = searchQuery.value
  appliedStatusFilter.value = statusFilter.value
  page.value = 1
  loadLedger()
}

const shiftMonth = (offset: number) => {
  if (!selectedMonth.value) return
  const [year, month] = selectedMonth.value.split('-').map(Number)
  const target = new Date(year, month - 1 + offset, 1)
  const next = `${target.getFullYear()}-${String(target.getMonth() + 1).padStart(2, '0')}`
  if (currentMonth.value && next > currentMonth.value) return
  selectedMonth.value = next
  changeMonth()
}

const changePage = (value: number) => { page.value = value; loadLedger() }
const changePageSize = (value: number) => { pageSize.value = value; page.value = 1; loadLedger() }
const changeSort = (key: typeof sortBy.value) => {
  if (sortBy.value === key) sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  else { sortBy.value = key; sortOrder.value = 'desc' }
  page.value = 1
  loadLedger()
}

const showMultiplierDialog = ref(false)
const editingMultiplierRow = ref<MonthlyLedgerRow | null>(null)
const multiplierMode = ref<'single' | 'batch'>('single')
const batchMultiplierUserIDs = ref<number[]>([])
const multiplierDraft = ref('1')
const multiplierError = ref('')
const savingMultiplier = ref(false)
const multiplierPresets = [0, 0.5, 0.8, 1]

const decimalInteger = (value: number) => {
  if (!Number.isFinite(value) || value < 0) return null
  const [mantissa, exponentText = '0'] = value.toString().toLowerCase().split('e')
  const [whole, fraction = ''] = mantissa.split('.')
  const exponent = Number(exponentText)
  let digits = `${whole}${fraction}`.replace(/^0+(?=\d)/, '')
  let scale = fraction.length - exponent
  if (scale < 0) {
    digits += '0'.repeat(-scale)
    scale = 0
  }
  return { integer: BigInt(digits || '0'), scale }
}

const roundNonNegativeDecimalProduct = (left: number, right: number, places = 2) => {
  const leftDecimal = decimalInteger(left)
  const rightDecimal = decimalInteger(right)
  if (!leftDecimal || !rightDecimal) return 0
  const product = leftDecimal.integer * rightDecimal.integer
  const scale = leftDecimal.scale + rightDecimal.scale
  let rounded: bigint
  if (scale <= places) {
    rounded = product * (10n ** BigInt(places - scale))
  } else {
    const divisor = 10n ** BigInt(scale - places)
    rounded = product / divisor
    if ((product % divisor) * 2n >= divisor) rounded += 1n
  }
  return Number(rounded) / (10 ** places)
}

const multiplierPreview = computed(() => {
  const multiplier = Number(multiplierDraft.value)
  if (!editingMultiplierRow.value || !Number.isFinite(multiplier)) return 0
  return roundNonNegativeDecimalProduct(editingMultiplierRow.value.pricing_usage_amount, multiplier)
})

const multiplierDialogTitle = computed(() => multiplierMode.value === 'batch'
  ? t('admin.monthlyLedger.bulk.title')
  : t('admin.monthlyLedger.multiplier.title'))

const openMultiplier = (row: MonthlyLedgerRow) => {
  multiplierMode.value = 'single'
  batchMultiplierUserIDs.value = []
  editingMultiplierRow.value = row
  multiplierDraft.value = String(row.multiplier)
  multiplierError.value = ''
  showMultiplierDialog.value = true
}
const openBulkMultiplier = () => {
  if (!canUpdate.value || selIds.value.length === 0) return
  multiplierMode.value = 'batch'
  batchMultiplierUserIDs.value = [...selIds.value]
  editingMultiplierRow.value = null
  multiplierDraft.value = '1'
  multiplierError.value = ''
  showMultiplierDialog.value = true
}
const closeMultiplier = () => {
  showMultiplierDialog.value = false
  editingMultiplierRow.value = null
  batchMultiplierUserIDs.value = []
}
const saveMultiplier = async () => {
  if (!canUpdate.value) return
  const row = editingMultiplierRow.value
  const multiplier = Number(multiplierDraft.value)
  const scaled = multiplier * 10000
  const hasTarget = multiplierMode.value === 'batch' ? batchMultiplierUserIDs.value.length > 0 : Boolean(row)
  if (!hasTarget || !Number.isFinite(multiplier) || multiplier < 0 || Math.abs(scaled - Math.round(scaled)) > 1e-7) {
    multiplierError.value = t('admin.monthlyLedger.multiplier.invalid')
    return
  }
  savingMultiplier.value = true
  try {
    const isBatch = multiplierMode.value === 'batch'
    const targetCount = batchMultiplierUserIDs.value.length
    if (isBatch) {
      await adminAPI.monthlyLedger.setMultipliers(selectedMonth.value, batchMultiplierUserIDs.value, multiplier)
    } else if (row) {
      await adminAPI.monthlyLedger.setMultiplier(selectedMonth.value, row.user_id, multiplier)
    }
    closeMultiplier()
    if (isBatch) {
      clearSelection()
      appStore.showSuccess(t('admin.monthlyLedger.bulk.saved', { count: targetCount }))
    } else {
      appStore.showSuccess(t('admin.monthlyLedger.saved'))
    }
    await loadLedger()
  } catch (error) {
    multiplierError.value = errorMessage(error, t('admin.monthlyLedger.saveFailed'))
  } finally {
    savingMultiplier.value = false
  }
}

const savingSettlement = ref(false)
const showPaymentsDialog = ref(false)
const activeRow = ref<MonthlyLedgerRow | null>(null)
const payments = ref<MonthlyLedgerPayment[]>([])
const paymentsLoading = ref(false)
const editingPayment = ref<MonthlyLedgerPayment | null>(null)
const savingPayment = ref(false)
const paymentError = ref('')
const nowForInput = ref('')
const paymentForm = reactive({ amount: 0, paidAt: '', note: '' })
let paymentRequestSequence = 0

const paymentDialogKey = () => activeRow.value
  ? `${selectedMonth.value}:${activeRow.value.user_id}`
  : ''

const toDateTimeLocal = (date: Date) => {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

const refreshPaymentTimeLimit = () => {
  nowForInput.value = toDateTimeLocal(new Date())
}

const resetPaymentForm = () => {
  editingPayment.value = null
  paymentForm.amount = Number((activeRow.value?.outstanding_amount || 0).toFixed(2))
  refreshPaymentTimeLimit()
  paymentForm.paidAt = nowForInput.value
  paymentForm.note = ''
  paymentError.value = ''
}

const loadPayments = async () => {
  const row = activeRow.value
  const month = selectedMonth.value
  if (!row) return
  const key = `${month}:${row.user_id}`
  const sequence = ++paymentRequestSequence
  paymentsLoading.value = true
  try {
    const result = await adminAPI.monthlyLedger.listPayments(month, row.user_id)
    if (sequence !== paymentRequestSequence || key !== paymentDialogKey()) return
    payments.value = result
  } catch (error) {
    if (sequence !== paymentRequestSequence || key !== paymentDialogKey()) return
    payments.value = []
    appStore.showError(errorMessage(error, t('admin.monthlyLedger.loadFailed')))
  } finally {
    if (sequence === paymentRequestSequence && key === paymentDialogKey()) paymentsLoading.value = false
  }
}

const openPayments = async (row: MonthlyLedgerRow, startAdding: boolean) => {
  activeRow.value = row
  showPaymentsDialog.value = true
  resetPaymentForm()
  if (!startAdding && !canCreate.value) paymentForm.amount = 0
  await loadPayments()
}
const closePayments = () => {
  paymentRequestSequence++
  showPaymentsDialog.value = false
  activeRow.value = null
  payments.value = []
  paymentsLoading.value = false
  resetPaymentForm()
}

const refreshPaymentDialog = async (key: string, month: string, userID: number, resetForm = true) => {
  const paymentsRefresh = key === paymentDialogKey() ? loadPayments() : Promise.resolve()
  await Promise.all([paymentsRefresh, loadLedger()])
  if (key !== paymentDialogKey()) return
  let refreshedRow = rows.value.find((row) => row.user_id === userID)
  if (!refreshedRow) {
    try {
      const result = await adminAPI.monthlyLedger.list({
        month,
        user_id: userID,
        status: '',
        page: 1,
        page_size: 1,
      })
      if (key !== paymentDialogKey()) return
      refreshedRow = result.items.find((row) => row.user_id === userID)
    } catch (error) {
      if (key === paymentDialogKey()) appStore.showError(errorMessage(error, t('admin.monthlyLedger.loadFailed')))
      return
    }
  }
  if (refreshedRow) activeRow.value = refreshedRow
  if (resetForm) resetPaymentForm()
}
const saveSettlement = async (settled: boolean) => {
  const row = activeRow.value
  const month = selectedMonth.value
  const key = paymentDialogKey()
  if (!row || !canUpdate.value || !canRecordPayments.value || savingSettlement.value) return
  savingSettlement.value = true
  try {
    const result = await adminAPI.monthlyLedger.setSettlement(month, row.user_id, settled)
    if (key === paymentDialogKey() && activeRow.value) activeRow.value.manually_settled = result.manually_settled
    appStore.showSuccess(t('admin.monthlyLedger.saved'))
    await refreshPaymentDialog(key, month, row.user_id, false)
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.monthlyLedger.saveFailed')))
  } finally {
    savingSettlement.value = false
  }
}
const editPayment = (payment: MonthlyLedgerPayment) => {
  if (!canUpdate.value) return
  refreshPaymentTimeLimit()
  editingPayment.value = payment
  paymentForm.amount = payment.amount
  paymentForm.paidAt = toDateTimeLocal(new Date(payment.paid_at))
  paymentForm.note = payment.note || ''
  paymentError.value = ''
}

const savePayment = async () => {
  const row = activeRow.value
  const month = selectedMonth.value
  const key = paymentDialogKey()
  const isEditing = Boolean(editingPayment.value)
  if ((isEditing && !canUpdate.value) || (!isEditing && !canCreate.value)) return
  const amount = Number(paymentForm.amount)
  if (!row || !Number.isFinite(amount) || amount <= 0 || Math.abs(amount * 100 - Math.round(amount * 100)) > 1e-7) {
    paymentError.value = t('admin.monthlyLedger.payments.invalidAmount')
    return
  }
  const paidAt = new Date(paymentForm.paidAt)
  if (!paymentForm.paidAt || Number.isNaN(paidAt.getTime()) || paidAt.getTime() > Date.now()) {
    paymentError.value = t('admin.monthlyLedger.payments.invalidPaidAt')
    return
  }
  savingPayment.value = true
  const input = { amount, paid_at: paidAt.toISOString(), note: paymentForm.note.trim() }
  try {
    if (editingPayment.value) await adminAPI.monthlyLedger.updatePayment(editingPayment.value.id, input)
    else await adminAPI.monthlyLedger.createPayment(month, row.user_id, input)
    appStore.showSuccess(t('admin.monthlyLedger.saved'))
    await refreshPaymentDialog(key, month, row.user_id)
  } catch (error) {
    paymentError.value = errorMessage(error, t('admin.monthlyLedger.saveFailed'))
  } finally {
    savingPayment.value = false
  }
}

const deletingPayment = ref<MonthlyLedgerPayment | null>(null)
const requestDeletePayment = (payment: MonthlyLedgerPayment) => {
  if (canDelete.value) deletingPayment.value = payment
}
const confirmDeletePayment = async () => {
  if (!canDelete.value || !deletingPayment.value) return
  const row = activeRow.value
  const month = selectedMonth.value
  const key = paymentDialogKey()
  try {
    await adminAPI.monthlyLedger.deletePayment(deletingPayment.value.id)
    deletingPayment.value = null
    appStore.showSuccess(t('admin.monthlyLedger.saved'))
    if (row) await refreshPaymentDialog(key, month, row.user_id)
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.monthlyLedger.saveFailed')))
  }
}

onMounted(loadLedger)
</script>

<style scoped>
.ledger-page { color: var(--nx-text); }
.ledger-toolbar {
  padding: 16px;
  border: 1px solid var(--nx-border);
  border-radius: 6px;
  background: var(--nx-surface);
}
.month-control { display: grid; grid-template-columns: 36px minmax(132px, 1fr) 36px; }
.month-input, .month-arrow {
  height: 38px;
  border: 1px solid var(--nx-border);
  background: var(--nx-surface);
  color: var(--nx-text);
}
.month-input { min-width: 138px; border-left: 0; border-right: 0; padding: 0 10px; font-size: 14px; }
.month-arrow { display: grid; place-items: center; transition: background 150ms ease, color 150ms ease; }
.month-arrow:first-child { border-radius: 4px 0 0 4px; }
.month-arrow:last-child { border-radius: 0 4px 4px 0; }
.month-arrow:hover:not(:disabled) { background: var(--nx-bg); color: var(--nx-accent); }
.month-arrow:disabled { cursor: not-allowed; opacity: 0.4; }
.open-month-notice {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--nx-border);
  padding-top: 12px;
  color: var(--nx-warning);
  font-size: 13px;
}
.bulk-action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--nx-accent) 28%, var(--nx-border));
  border-radius: 6px;
  background: color-mix(in srgb, var(--nx-accent) 6%, var(--nx-surface));
}
.bulk-link {
  color: var(--nx-accent);
  font-size: 12px;
  font-weight: 650;
}
.bulk-link::before { content: '·'; margin-right: 8px; color: var(--nx-subtle); }
.bulk-link:hover:not(:disabled) { text-decoration: underline; }
.bulk-link:disabled { cursor: wait; opacity: 0.55; }
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 14px;
}
.summary-card {
  min-width: 0;
  min-height: 108px;
  display: flex;
  align-items: center;
  gap: 13px;
  padding: 17px;
  border: 1px solid var(--nx-border);
  border-radius: 6px;
  background: var(--nx-surface);
  box-shadow: 0 1px 2px rgba(17, 17, 17, 0.03);
}
.summary-card-featured { border-top: 3px solid var(--nx-accent); padding-top: 15px; }
.summary-card-featured:nth-child(3) { border-top-color: var(--nx-success); }
.summary-icon { display: grid; width: 42px; height: 42px; flex: 0 0 42px; place-items: center; border-radius: 7px; }
.summary-icon-neutral { background: rgba(71, 85, 105, 0.1); color: #475569; }
.summary-icon-receivable { background: rgba(255, 86, 0, 0.11); color: var(--nx-accent); }
.summary-icon-paid { background: rgba(22, 163, 74, 0.11); color: #15803d; }
.summary-icon-outstanding { background: rgba(202, 138, 4, 0.12); color: #a16207; }
.summary-icon-overpaid { background: rgba(3, 105, 161, 0.1); color: #0369a1; }
.summary-label, .summary-detail { display: block; color: var(--nx-subtle); font-size: 11px; font-weight: 600; text-transform: uppercase; }
.summary-value { display: block; margin-top: 6px; font-size: 20px; line-height: 1.2; letter-spacing: 0; white-space: nowrap; }
.summary-detail { margin-top: 5px; font-weight: 500; text-transform: none; }
.ledger-table-shell { border: 1px solid var(--nx-border); border-radius: 6px; background: var(--nx-surface); overflow: hidden; }
.ledger-table { width: 100%; min-width: 1080px; border-collapse: collapse; }
.ledger-table th { padding: 11px 14px; border-bottom: 1px solid var(--nx-border); background: var(--nx-bg); color: var(--nx-muted); font-size: 11px; font-weight: 700; text-transform: uppercase; }
.ledger-table td { padding: 13px 14px; border-bottom: 1px solid var(--nx-border); vertical-align: middle; }
.ledger-table tbody tr:last-child td { border-bottom: 0; }
.ledger-table tbody tr:hover { background: color-mix(in srgb, var(--nx-bg) 65%, transparent); }
.ledger-table tbody .ledger-row-selected { background: color-mix(in srgb, var(--nx-accent) 6%, var(--nx-surface)); }
.selection-cell { width: 46px; padding-left: 12px !important; padding-right: 8px !important; }
.selection-checkbox { width: 16px; height: 16px; cursor: pointer; border-radius: 3px; border-color: var(--nx-border); color: var(--nx-accent); }
.selection-checkbox:focus { box-shadow: 0 0 0 2px color-mix(in srgb, var(--nx-accent) 22%, transparent); }
.sort-button { margin-left: auto; display: inline-flex; align-items: center; gap: 4px; transition: color 150ms ease; }
.sort-button:hover { color: var(--nx-accent); }
.money-cell { white-space: nowrap; text-align: right; font-variant-numeric: tabular-nums; font-size: 14px; }
.deleted-badge, .payment-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--nx-surface-muted);
  color: var(--nx-muted);
  font-size: 10px;
  line-height: 18px;
  padding: 0 6px;
}
.multiplier-button { display: inline-flex; align-items: center; gap: 5px; border-bottom: 1px dashed var(--nx-muted); color: var(--nx-text); }
.multiplier-button:hover { border-color: var(--nx-accent); color: var(--nx-accent); }
.status-badge { display: inline-flex; min-width: 68px; justify-content: center; border-radius: 999px; padding: 4px 9px; font-size: 11px; font-weight: 700; }
.status-unpaid { background: rgba(220, 38, 38, 0.09); color: #b91c1c; }
.status-partial { background: rgba(202, 138, 4, 0.11); color: #a16207; }
.status-settled { background: rgba(22, 163, 74, 0.1); color: #15803d; }
.status-overpaid { background: rgba(3, 105, 161, 0.1); color: #0369a1; }
.status-waived { background: var(--nx-surface-muted); color: var(--nx-muted); }
.icon-command { position: relative; display: grid; width: 32px; height: 32px; place-items: center; border-radius: 4px; color: var(--nx-muted); transition: background 150ms ease, color 150ms ease; }
.icon-command:hover:not(:disabled) { background: var(--nx-bg); color: var(--nx-text); }
.icon-command-primary { color: var(--nx-accent); }
.icon-command-danger:hover { color: #dc2626; }
.icon-command:disabled { cursor: not-allowed; opacity: 0.35; }
.payment-count { position: absolute; right: -3px; top: -3px; min-width: 16px; height: 16px; padding: 0 4px; line-height: 16px; }
.empty-row { height: 180px; text-align: center; color: var(--nx-subtle); font-size: 14px; }
.amount-comparison { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 16px; padding: 14px; border: 1px solid var(--nx-border); border-radius: 6px; background: var(--nx-bg); }
.amount-comparison span { display: block; color: var(--nx-subtle); font-size: 11px; }
.amount-comparison strong { display: block; margin-top: 4px; font-size: 18px; }
.batch-scope {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--nx-border);
  border-radius: 6px;
  background: var(--nx-bg);
  color: var(--nx-muted);
  font-size: 13px;
}
.multiplier-preset { height: 38px; border: 1px solid var(--nx-border); border-radius: 4px; background: var(--nx-surface); color: var(--nx-muted); font-size: 13px; font-weight: 650; }
.multiplier-preset:hover, .multiplier-preset-active { border-color: var(--nx-accent); background: rgba(255, 86, 0, 0.07); color: var(--nx-accent); }
.payment-empty { display: grid; min-height: 230px; place-items: center; border: 1px dashed var(--nx-border); color: var(--nx-subtle); font-size: 13px; }
.payment-record { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 14px 4px; }
.payment-form { align-self: start; display: grid; gap: 16px; padding: 16px; border-left: 3px solid var(--nx-accent); background: var(--nx-bg); }
@media (max-width: 640px) {
  .bulk-action-bar { align-items: stretch; flex-direction: column; }
  .bulk-action-bar > .btn { justify-content: center; width: 100%; }
  .summary-grid { grid-template-columns: minmax(0, 1fr); }
  .payment-form { border-left: 0; border-top: 3px solid var(--nx-accent); }
}
</style>
