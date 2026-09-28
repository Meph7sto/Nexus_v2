<template>
  <div class="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2">
    <label v-for="key in textFields" :key="key" class="min-w-0">
      <span class="input-label">{{ t(`admin.monthlyLedger.income.${key}`) }}</span>
      <input :value="modelValue[key]" :name="key" :list="key === 'category' ? categoryListId : undefined" :maxlength="key === 'category' ? 100 : 200" required class="input w-full" @input="update(key, ($event.target as HTMLInputElement).value)">
    </label>
    <datalist :id="categoryListId"><option v-for="category in presets" :key="category" :value="category" /></datalist>
    <label v-for="key in moneyFields" :key="key" class="min-w-0">
      <span class="input-label">{{ t(`admin.monthlyLedger.income.${key}`) }} (USD)</span>
      <input :value="modelValue[key]" :name="key" type="number" :min="key === 'amount' ? 0.01 : 0" max="999999999999.99" step="0.01" required class="input w-full" @input="update(key, Number(($event.target as HTMLInputElement).value))">
    </label>
    <label class="sm:col-span-2"><span class="input-label">{{ t('admin.monthlyLedger.payments.note') }}</span><textarea :value="modelValue.note" name="note" maxlength="500" class="input w-full" rows="2" @input="update('note', ($event.target as HTMLTextAreaElement).value)" /></label>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { IncomeFields } from '@/api/admin/monthlyLedgerIncome'
const props = defineProps<{ modelValue: IncomeFields; categoryListId: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: IncomeFields] }>()
const { t } = useI18n()
const textFields = ['title', 'customer', 'category'] as const
const moneyFields = ['amount', 'cost'] as const
const presets = computed(() => ['topup', 'reset', 'other'].map(key => t(`admin.monthlyLedger.income.categories.${key}`)))
function update(key: keyof IncomeFields, value: string | number) { emit('update:modelValue', { ...props.modelValue, [key]: value }) }
</script>
