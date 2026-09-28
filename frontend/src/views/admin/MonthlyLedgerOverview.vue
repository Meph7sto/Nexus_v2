<template>
  <section :aria-label="t('admin.monthlyLedger.income.overview')" aria-live="polite">
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }} <button class="btn btn-ghost" @click="load">{{ t('admin.monthlyLedger.refresh') }}</button></p>
    <div v-else class="grid grid-cols-2 gap-x-5 gap-y-4 border-y border-[var(--nx-border)] py-4 lg:grid-cols-4" :aria-busy="loading">
      <div v-for="key in keys" :key="key" class="min-w-0">
        <span class="text-xs text-[var(--nx-subtle)]">{{ t(`admin.monthlyLedger.summary.${labels[key]}`) }}</span>
        <strong class="mt-1 block break-words text-xl tabular-nums">{{ loading || !summary ? '—' : `$${summary[key].toFixed(2)}` }}</strong>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { incomeAPI, type IncomeSummary } from '@/api/admin/monthlyLedgerIncome'
const props = defineProps<{ month: string; revision: number }>()
const { t } = useI18n()
const summary = ref<IncomeSummary | null>(null)
const loading = ref(false)
const error = ref('')
const keys = ['receivable_amount', 'paid_amount', 'outstanding_amount', 'overpaid_amount'] as const
const labels = { receivable_amount: 'receivable', paid_amount: 'paid', outstanding_amount: 'outstanding', overpaid_amount: 'overpaid' }
let sequence = 0
async function load() {
  const request = ++sequence
  if (!props.month) return
  loading.value = true; error.value = ''
  try { const result = await incomeAPI.overview(props.month); if (request === sequence) summary.value = result }
  catch { if (request === sequence) { summary.value = null; error.value = t('admin.monthlyLedger.loadFailed') } }
  finally { if (request === sequence) loading.value = false }
}
watch(() => [props.month, props.revision], load, { immediate: true })
</script>
