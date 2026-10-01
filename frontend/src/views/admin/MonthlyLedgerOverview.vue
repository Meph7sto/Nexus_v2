<template>
  <section :aria-label="t('admin.monthlyLedger.income.overview')" aria-live="polite">
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }} <button class="btn btn-ghost" @click="load">{{ t('admin.monthlyLedger.refresh') }}</button></p>
    <div class="summary-grid" :aria-busy="loading">
      <div class="summary-card" data-test="summary-usage">
        <span class="summary-icon summary-icon-neutral"><Icon name="chart" size="md" /></span>
        <div class="min-w-0">
          <span class="summary-label">{{ t('admin.monthlyLedger.summary.actualUsage') }}</span>
          <strong class="summary-value">{{ relaySummary ? `$${relaySummary.usage_amount.toFixed(2)}` : '—' }}</strong>
          <span class="summary-detail">{{ t('admin.monthlyLedger.summary.actualUsageHint') }}</span>
        </div>
      </div>
      <div v-for="metric in metrics" :key="metric.key" class="summary-card" :class="metric.featured ? 'summary-card-featured' : ''" :data-test="`summary-${metric.label}`">
        <span class="summary-icon" :class="`summary-icon-${metric.label}`"><Icon :name="metric.icon" size="md" /></span>
        <div class="min-w-0">
          <span class="summary-label">{{ t(`admin.monthlyLedger.summary.${metric.label}`) }}</span>
          <strong class="summary-value" :class="metric.valueClass">{{ loading || !summary ? '—' : `$${summary[metric.key].toFixed(2)}` }}</strong>
          <span class="summary-detail">{{ t('admin.monthlyLedger.summary.combined') }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { incomeAPI, type IncomeSummary } from '@/api/admin/monthlyLedgerIncome'
import type { MonthlyLedgerSummary } from '@/api/admin/monthlyLedger'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ month: string; revision: number; relaySummary?: MonthlyLedgerSummary | null }>()
const { t } = useI18n()
const summary = ref<IncomeSummary | null>(null)
const loading = ref(false)
const error = ref('')
const metrics = [
  { key: 'receivable_amount', label: 'receivable', icon: 'creditCard', featured: true, valueClass: '' },
  { key: 'paid_amount', label: 'paid', icon: 'checkCircle', featured: true, valueClass: 'text-[var(--nx-success)]' },
  { key: 'outstanding_amount', label: 'outstanding', icon: 'clock', featured: false, valueClass: 'text-[var(--nx-warning)]' },
  { key: 'overpaid_amount', label: 'overpaid', icon: 'arrowUp', featured: false, valueClass: '' },
] as const
let sequence = 0
async function load() {
  const request = ++sequence
  summary.value = null
  if (!props.month) { loading.value = false; return }
  loading.value = true; error.value = ''
  try { const result = await incomeAPI.overview(props.month); if (request === sequence) summary.value = result }
  catch { if (request === sequence) { summary.value = null; error.value = t('admin.monthlyLedger.loadFailed') } }
  finally { if (request === sequence) loading.value = false }
}
watch(() => [props.month, props.revision], load, { immediate: true })
</script>
<style scoped src="./monthlyLedger.css"></style>
