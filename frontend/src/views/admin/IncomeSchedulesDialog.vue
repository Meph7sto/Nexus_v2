<template>
  <BaseDialog :show="show && !editing" :title="tr('schedules')" width="wide" @close="!saving && emit('close')">
    <div class="mb-4 flex justify-end"><button v-if="canCreate" class="btn btn-primary" @click="edit()"><Icon name="plus" size="sm" />{{ tr('addSchedule') }}</button></div>
    <p v-if="error" role="alert" class="mb-3 text-red-600">{{ error }}</p>
    <p v-if="loading">{{ t('common.loading') }}</p>
    <div v-else class="overflow-x-auto">
      <table class="w-full text-left text-sm"><thead><tr><th>{{ tr('title') }}</th><th>{{ tr('customer') }}</th><th>{{ tr('amount') }}</th><th>{{ tr('interval') }}</th><th>{{ tr('state') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
        <tbody><tr v-for="s in schedules" :key="s.id" class="border-b border-[var(--nx-border)]"><td class="max-w-52 break-words">{{ s.title }}</td><td class="max-w-40 break-words">{{ s.customer }}</td><td>${{ s.amount.toFixed(2) }}</td><td>{{ t('admin.monthlyLedger.income.everyMonths', { count: s.interval_months }) }}</td><td>{{ tr(s.state) }}</td><td><div v-if="canUpdate && s.state !== 'ended'" class="flex gap-1">
          <button class="btn btn-ghost px-2" :disabled="saving" :title="t('common.edit')" @click="edit(s)"><Icon name="edit" size="sm" /></button>
          <button class="btn btn-ghost" :disabled="saving" @click="setState(s, s.state === 'active' ? 'paused' : 'active')">{{ tr(s.state === 'active' ? 'pause' : 'resume') }}</button>
          <button class="btn btn-ghost" :disabled="saving" @click="ending = s">{{ tr('end') }}</button>
        </div></td></tr><tr v-if="!schedules.length"><td colspan="6" class="py-8 text-center">{{ tr('noSchedules') }}</td></tr></tbody>
      </table>
    </div>
  </BaseDialog>
  <BaseDialog :show="show && editing" :title="editingID ? tr('editSchedule') : tr('addSchedule')" @close="!saving && (editing = false)">
    <form class="space-y-4" @submit.prevent="submit">
      <fieldset :disabled="saving" class="space-y-4">
      <IncomeFieldsForm :model-value="form" category-list-id="income-schedule-categories" @update:model-value="Object.assign(form, $event)" />
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label><span class="input-label">{{ tr('firstDate') }}</span><input v-model="form.first_date" type="date" :disabled="!!editingID" required class="input w-full"></label>
        <label><span class="input-label">{{ tr('endDate') }}</span><input v-model="endDate" type="date" :min="form.first_date" class="input w-full"></label>
        <label><span class="input-label">{{ tr('interval') }}</span><input v-model.number="form.interval_months" type="number" min="1" max="1200" step="1" :disabled="!!editingID" required class="input w-full"></label>
        <div v-if="!editingID" class="flex flex-wrap items-end gap-2"><button v-for="n in [1, 3, 12]" :key="n" type="button" class="btn btn-secondary" @click="form.interval_months = n">{{ tr(n === 1 ? 'monthly' : n === 3 ? 'quarterly' : 'yearly') }}</button></div>
      </div>
      <p v-if="preview !== null" class="text-sm text-[var(--nx-subtle)]">{{ t('admin.monthlyLedger.income.preview', { count: preview }) }}</p>
      <p v-if="formError" role="alert" class="text-sm text-red-600">{{ formError }}</p>
      <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="editing = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ editingID || preview !== null ? t('common.save') : tr('previewAction') }}</button></div>
      </fieldset>
    </form>
  </BaseDialog>
  <BaseDialog :show="!!ending" :title="tr('end')" width="narrow" :z-index="70" @close="!saving && (ending = null)">
    <p>{{ tr('endConfirm') }}</p><p v-if="error" role="alert" class="mt-3 text-red-600">{{ error }}</p>
    <template #footer><button class="btn btn-secondary" :disabled="saving" @click="ending = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving" @click="ending && setState(ending, 'ended')">{{ t('common.confirm') }}</button></template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { incomeAPI, type IncomeSchedule, type IncomeScheduleInput } from '@/api/admin/monthlyLedgerIncome'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import IncomeFieldsForm from './IncomeFieldsForm.vue'
const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const tr = (key: string) => t(`admin.monthlyLedger.income.${key}`)
const auth = useAuthStore()
const canCreate = computed(() => auth.canAdmin('monthly_ledger', 'create'))
const canUpdate = computed(() => auth.canAdmin('monthly_ledger', 'update'))
const schedules = ref<IncomeSchedule[]>([]), loading = ref(false), saving = ref(false), error = ref(''), formError = ref('')
const editing = ref(false), editingID = ref(0), preview = ref<number | null>(null), ending = ref<IncomeSchedule | null>(null)
const endDate = ref('')
const blank = (): IncomeScheduleInput => { const now = new Date(); return { title: '', customer: '', category: tr('categories.other'), amount: 0, cost: 0, note: '', first_date: new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 10), interval_months: 1, end_date: null } }
const form = reactive(blank())
const message = (e: unknown) => (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('admin.monthlyLedger.saveFailed')
async function load() { loading.value = true; error.value = ''; try { schedules.value = await incomeAPI.schedules() } catch (e) { error.value = message(e) } finally { loading.value = false } }
watch(() => props.show, value => { if (value) { editing.value = false; load() } }, { immediate: true })
watch([form, endDate], () => { preview.value = null })
function edit(s?: IncomeSchedule) { Object.assign(form, s || blank()); endDate.value = s?.end_date || ''; editingID.value = s?.id || 0; formError.value = ''; preview.value = null; editing.value = true }
async function submit() { if (saving.value) return; saving.value = true; formError.value = ''; try { const input = { ...form, end_date: endDate.value || null }; if (!editingID.value && preview.value === null) { preview.value = (await incomeAPI.preview(input)).count; return } await incomeAPI.saveSchedule(editingID.value, input); editing.value = false; emit('changed'); await load() } catch (e) { formError.value = message(e) } finally { saving.value = false } }
async function setState(s: IncomeSchedule, state: IncomeSchedule['state']) { if (saving.value) return; saving.value = true; error.value = ''; try { await incomeAPI.setState(s.id, state); ending.value = null; emit('changed'); await load() } catch (e) { error.value = message(e) } finally { saving.value = false } }
</script>
<style scoped>
th, td { padding: 0.75rem 0.5rem; }
th { white-space: nowrap; font-weight: 500; color: var(--nx-subtle); }
</style>
