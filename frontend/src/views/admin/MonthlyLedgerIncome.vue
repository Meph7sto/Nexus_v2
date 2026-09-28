<template>
  <section class="space-y-4" :aria-label="tr('other')">
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="page = 1; load()">
      <label class="w-full min-w-0 sm:min-w-[220px] sm:flex-1 sm:basis-0"><span class="input-label">{{ t('common.search') }}</span><input v-model="query" class="input w-full" :placeholder="tr('search')"></label>
      <label><span class="input-label">{{ tr('category') }}</span><select v-model="category" class="input" @change="page = 1; load()"><option value="">{{ tr('allCategories') }}</option><option v-for="c in categories" :key="c">{{ c }}</option></select></label>
      <label><span class="input-label">{{ t('admin.monthlyLedger.statusFilter') }}</span><select v-model="status" class="input" @change="page = 1; load()"><option value="">{{ t('admin.monthlyLedger.allStatuses') }}</option><option v-for="s in statuses" :key="s" :value="s">{{ t(`admin.monthlyLedger.status.${s}`) }}</option></select></label>
      <button class="btn btn-secondary" :title="t('common.search')"><Icon name="search" size="sm" /></button>
      <button type="button" class="btn btn-secondary" @click="showSchedules = true"><Icon name="clock" size="sm" />{{ tr('schedules') }}</button>
      <button v-if="canCreate" type="button" class="btn btn-primary" data-test="add-income" @click="editEntry()"><Icon name="plus" size="sm" />{{ tr('add') }}</button>
      <button type="button" class="btn btn-ghost" :title="t('admin.monthlyLedger.refresh')" :disabled="loading" @click="load(); emit('changed')"><Icon name="refresh" size="sm" /></button>
    </form>
    <div v-if="summary" class="flex flex-wrap gap-x-8 gap-y-2 border-b border-[var(--nx-border)] pb-3 text-sm">
      <span>{{ tr('cost') }} <strong class="ml-2 tabular-nums">{{ usd(summary.cost) }}</strong></span>
      <span>{{ tr('profit') }} <strong class="ml-2 tabular-nums" :class="summary.profit < 0 ? 'text-red-600' : 'text-[var(--nx-success)]'">{{ usd(summary.profit) }}</strong></span>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <div class="overflow-x-auto" :aria-busy="loading">
      <table class="income-table">
        <thead><tr><th v-for="key in columns" :key="key">{{ tr(key) }}</th><th>{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-if="loading"><td :colspan="columns.length + 1" class="text-center">{{ t('common.loading') }}</td></tr>
          <template v-else>
            <tr v-for="row in rows" :key="row.id">
              <td class="max-w-56"><span class="break-words font-medium">{{ row.title }}</span><Icon v-if="row.schedule_id" name="clock" size="sm" class="ml-1 inline" :title="tr('recurring')" /></td>
              <td class="max-w-40 break-words">{{ row.customer }}</td><td class="max-w-32 break-words">{{ row.category }}</td><td class="whitespace-nowrap">{{ row.due_date }}</td>
              <td>{{ usd(row.amount) }}</td><td>{{ usd(row.paid_amount) }}</td><td>{{ usd(row.cost) }}</td><td :class="row.profit < 0 ? 'text-red-600' : ''">{{ usd(row.profit) }}</td>
              <td class="whitespace-nowrap">{{ t(`admin.monthlyLedger.status.${row.status}`) }}</td>
              <td><div class="flex gap-1">
                <button class="btn btn-ghost px-2" :title="t('admin.monthlyLedger.payments.title')" @click="openPayments(row)"><Icon name="creditCard" size="sm" /></button>
                <button v-if="canUpdate" class="btn btn-ghost px-2" :title="t('common.edit')" @click="editEntry(row)"><Icon name="edit" size="sm" /></button>
                <button v-if="canDelete" class="btn btn-ghost px-2" :title="row.paid_amount > 0 ? tr('hasPayments') : t('common.delete')" :disabled="row.paid_amount > 0" @click="confirmDelete = { kind: 'entry', id: row.id }"><Icon name="trash" size="sm" /></button>
              </div></td>
            </tr>
            <tr v-if="!rows.length"><td :colspan="columns.length + 1" class="py-10 text-center text-[var(--nx-subtle)]">{{ tr('empty') }}</td></tr>
          </template>
        </tbody>
      </table>
    </div>
    <Pagination v-if="total" :page="page" :page-size="pageSize" :total="total" @update:page="page = $event; load()" @update:page-size="pageSize = $event; page = 1; load()" />
  </section>
  <BaseDialog :show="entryOpen" :title="editingID ? tr('edit') : tr('add')" @close="!saving && (entryOpen = false)">
    <form class="space-y-4" @submit.prevent="saveEntry">
      <fieldset :disabled="saving" class="space-y-4">
      <IncomeFieldsForm :model-value="entryForm" category-list-id="income-entry-categories" @update:model-value="Object.assign(entryForm, $event)" />
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label><span class="input-label">{{ tr('billingDate') }}</span><input v-model="entryForm.billing_date" name="billing_date" type="date" required class="input w-full" @change="syncDueDate"></label>
        <label><span class="input-label">{{ tr('dueDate') }}</span><input v-model="entryForm.due_date" name="due_date" type="date" required class="input w-full"></label>
      </div>
      <p v-if="formError" role="alert" class="text-sm text-red-600">{{ formError }}</p>
      <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="entryOpen = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div>
      </fieldset>
    </form>
  </BaseDialog>
  <BaseDialog :show="!!paymentEntry" :title="`${t('admin.monthlyLedger.payments.title')} · ${paymentEntry?.title || ''}`" width="wide" @close="!saving && (paymentEntry = null)">
    <p v-if="paymentError" role="alert" class="mb-3 text-sm text-red-600">{{ paymentError }}</p>
    <div class="overflow-x-auto">
      <table class="income-table"><thead><tr><th>{{ tr('paid') }}</th><th>{{ t('admin.monthlyLedger.payments.paidAt') }}</th><th>{{ t('admin.monthlyLedger.payments.note') }}</th><th>{{ t('admin.monthlyLedger.payments.operator') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
        <tbody><tr v-for="p in payments" :key="p.id"><td>{{ usd(p.amount) }}</td><td>{{ new Date(p.paid_at).toLocaleString() }}</td><td class="max-w-64 break-words">{{ p.note }}</td><td>{{ p.updated_by }}</td><td><div class="flex gap-1"><button v-if="canUpdate" class="btn btn-ghost px-2" :title="t('common.edit')" @click="editPayment(p)"><Icon name="edit" size="sm" /></button><button v-if="canDelete" class="btn btn-ghost px-2" :title="t('common.delete')" @click="confirmDelete = { kind: 'payment', id: p.id }"><Icon name="trash" size="sm" /></button></div></td></tr>
        <tr v-if="!payments.length"><td colspan="5">{{ t('admin.monthlyLedger.noPayments') }}</td></tr></tbody>
      </table>
    </div>
    <form v-if="paymentID ? canUpdate : canCreate" class="mt-5 space-y-3 border-t border-[var(--nx-border)] pt-4" @submit.prevent="savePayment">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label><span class="input-label">{{ t('admin.monthlyLedger.payments.amount') }}</span><input v-model.number="paymentForm.amount" type="number" min="0.01" max="999999999999.99" step="0.01" required class="input w-full"></label>
        <label><span class="input-label">{{ t('admin.monthlyLedger.payments.paidAt') }}</span><input v-model="paymentForm.paid_at" type="datetime-local" required class="input w-full"></label>
      </div>
      <label class="block"><span class="input-label">{{ t('admin.monthlyLedger.payments.note') }}</span><input v-model="paymentForm.note" maxlength="500" class="input w-full"></label>
      <div class="flex justify-end gap-2"><button v-if="paymentID" type="button" class="btn btn-secondary" @click="editPayment()">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ t(paymentID ? 'common.save' : 'admin.monthlyLedger.payments.add') }}</button></div>
    </form>
  </BaseDialog>
  <BaseDialog :show="!!confirmDelete" :title="t('common.delete')" width="narrow" :z-index="70" @close="!saving && (confirmDelete = null)">
    <p>{{ tr('deleteConfirm') }}</p>
    <p v-if="deleteError" role="alert" class="mt-3 text-red-600">{{ deleteError }}</p>
    <template #footer><button class="btn btn-secondary" :disabled="saving" @click="confirmDelete = null">{{ t('common.cancel') }}</button><button class="btn btn-danger" :disabled="saving" @click="remove">{{ t('common.delete') }}</button></template>
  </BaseDialog>
  <IncomeSchedulesDialog :show="showSchedules" @close="showSchedules = false" @changed="changed" />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { incomeAPI, type IncomeEntry, type IncomeEntryInput, type IncomePayment, type IncomeSummary } from '@/api/admin/monthlyLedgerIncome'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import IncomeFieldsForm from './IncomeFieldsForm.vue'
import IncomeSchedulesDialog from './IncomeSchedulesDialog.vue'
const props = defineProps<{ month: string }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const tr = (key: string) => t(`admin.monthlyLedger.income.${key}`)
const auth = useAuthStore()
const canCreate = computed(() => auth.canAdmin('monthly_ledger', 'create'))
const canUpdate = computed(() => auth.canAdmin('monthly_ledger', 'update'))
const canDelete = computed(() => auth.canAdmin('monthly_ledger', 'delete'))
const statuses = ['unpaid', 'partial', 'settled', 'overpaid']
const columns = ['title', 'customer', 'category', 'dueDate', 'amount', 'paid', 'cost', 'profit', 'status']
const rows = ref<IncomeEntry[]>([]), categories = ref<string[]>([]), summary = ref<IncomeSummary | null>(null)
const query = ref(''), category = ref(''), status = ref(''), error = ref('')
const page = ref(1), pageSize = ref(20), total = ref(0), loading = ref(false)
const entryOpen = ref(false), editingID = ref(0), saving = ref(false), formError = ref(''), showSchedules = ref(false)
const localDateTime = () => { const now = new Date(); return new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 16) }
const blank = (): IncomeEntryInput => ({ title: '', customer: '', category: tr('categories.other'), note: '', amount: 0, cost: 0, billing_date: localDateTime().slice(0, 10), due_date: localDateTime().slice(0, 10) })
const entryForm = reactive(blank())
let previousBilling = entryForm.billing_date
const usd = (n: number) => `$${n.toFixed(2)}`
const message = (e: unknown) => (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('admin.monthlyLedger.saveFailed')
let sequence = 0
async function load() {
  const seq = ++sequence; loading.value = true; error.value = ''
  try { const result = await incomeAPI.list({ month: props.month, q: query.value, category: category.value, status: status.value, page: page.value, page_size: pageSize.value }); if (seq !== sequence) return
    rows.value = result.items; total.value = result.total; categories.value = result.categories; summary.value = result.summary
    if (!rows.value.length && total.value > 0 && page.value > 1) { page.value = Math.ceil(total.value / pageSize.value); await load() }
  } catch (e) { if (seq === sequence) { error.value = message(e); rows.value = []; total.value = 0; summary.value = null } }
  finally { if (seq === sequence) loading.value = false }
}
watch(() => props.month, () => { page.value = 1; load() }, { immediate: true })
async function changed() { emit('changed'); await load() }
function editEntry(row?: IncomeEntry) { Object.assign(entryForm, row || blank()); previousBilling = entryForm.billing_date; editingID.value = row?.id || 0; formError.value = ''; entryOpen.value = true }
function syncDueDate() { if (entryForm.due_date === previousBilling) entryForm.due_date = entryForm.billing_date; previousBilling = entryForm.billing_date }
async function saveEntry() { if (saving.value) return; saving.value = true; formError.value = ''; try { await incomeAPI.saveEntry(editingID.value, entryForm); entryOpen.value = false; await changed() } catch (e) { formError.value = message(e) } finally { saving.value = false } }
const paymentEntry = ref<IncomeEntry | null>(null), payments = ref<IncomePayment[]>([]), paymentID = ref(0), paymentError = ref('')
const paymentForm = reactive({ amount: 0, paid_at: localDateTime(), note: '' })
let paymentSequence = 0
async function openPayments(row: IncomeEntry) { const seq = ++paymentSequence; paymentEntry.value = row; payments.value = []; paymentError.value = ''; editPayment(); try { const result = await incomeAPI.payments(row.id); if (seq === paymentSequence && paymentEntry.value?.id === row.id) payments.value = result } catch (e) { if (seq === paymentSequence) paymentError.value = message(e) } }
function editPayment(p?: IncomePayment) { paymentID.value = p?.id || 0; const d = p ? new Date(p.paid_at) : new Date(); Object.assign(paymentForm, { amount: p?.amount || 0, paid_at: new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16), note: p?.note || '' }) }
async function savePayment() {
  if (saving.value || !paymentEntry.value) return
  saving.value = true; paymentError.value = ''
  try {
    await incomeAPI.savePayment(paymentEntry.value.id, paymentID.value, { ...paymentForm, paid_at: new Date(paymentForm.paid_at).toISOString() })
    editPayment()
    await changed()
    payments.value = await incomeAPI.payments(paymentEntry.value.id)
  } catch (e) { paymentError.value = message(e) }
  finally { saving.value = false }
}
const confirmDelete = ref<{ kind: 'entry' | 'payment'; id: number } | null>(null), deleteError = ref('')
watch(confirmDelete, () => { deleteError.value = '' })
async function remove() {
  if (!confirmDelete.value || saving.value) return
  saving.value = true
  try {
    const { kind, id } = confirmDelete.value
    if (kind === 'entry') await incomeAPI.deleteEntry(id)
    else { await incomeAPI.deletePayment(id); if (paymentID.value === id) editPayment() }
    confirmDelete.value = null
    await changed()
    if (kind === 'payment' && paymentEntry.value) {
      try { payments.value = await incomeAPI.payments(paymentEntry.value.id) }
      catch (e) { paymentError.value = message(e) }
    }
  } catch (e) { deleteError.value = message(e) }
  finally { saving.value = false }
}
</script>

<style scoped>
.income-table { width: 100%; min-width: 960px; border-collapse: collapse; font-size: 0.8125rem; }
.income-table th { text-align: left; font-weight: 500; color: var(--nx-subtle); white-space: nowrap; }
.income-table th, .income-table td { padding: 0.75rem 0.625rem; border-bottom: 1px solid var(--nx-border); font-variant-numeric: tabular-nums; }
.income-table td { vertical-align: middle; }
</style>
