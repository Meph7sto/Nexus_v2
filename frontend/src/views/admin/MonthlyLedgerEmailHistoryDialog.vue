<template>
  <BaseDialog :show="!!recipient" :title="tr('historyTitle')" width="wide" @close="$emit('close')">
    <p class="mb-3 break-all text-sm">{{ recipient?.email }}</p>
    <div class="flex items-center justify-between border-b border-[var(--nx-border)] pb-3">
      <span>{{ month }}</span><strong>{{ tr('sentCount') }}: {{ history?.sent_count ?? '—' }}</strong>
      <button type="button" class="btn btn-ghost" :title="t('admin.monthlyLedger.refresh')" :disabled="loading" @click="load"><Icon name="refresh" size="sm" /></button>
    </div>
    <p class="mt-3 text-xs text-[var(--nx-subtle)]">{{ tr('historyCoverage') }}</p>
    <p v-if="error" role="alert" class="my-3 text-red-600">{{ error }}</p>
    <p v-if="loading" class="py-6">{{ t('common.loading') }}</p>
    <template v-else-if="history">
      <p v-if="!history.items.length" class="py-6 text-[var(--nx-subtle)]">{{ tr('noHistory') }}</p>
      <div v-for="record in history.items" :key="record.id" class="border-b border-[var(--nx-border)] py-4">
        <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
          <strong>${{ record.amount.toFixed(2) }}</strong>
          <span>{{ tr(record.source_type === 'monthly_ledger_manual' ? 'manual' : 'automatic') }}</span>
          <span>{{ tr(`deliveryStatus.${record.status}`) }}</span>
          <time>{{ new Date(record.sent_at || record.created_at).toLocaleString() }}</time>
          <button v-if="record.html" type="button" class="btn btn-ghost" :aria-expanded="selectedID === record.id" @click="selectedID = selectedID === record.id ? null : record.id"><Icon name="eye" size="sm" />{{ tr('previewTitle') }}</button>
        </div>
        <p class="mt-2 break-all text-sm">{{ record.recipient }}</p>
        <p class="mt-1 break-words font-medium">{{ record.subject || tr('contentUnavailable') }}</p>
        <iframe v-if="selectedID === record.id && record.html" :title="tr('previewTitle')" :srcdoc="safePreview(record.html)" sandbox="" referrerpolicy="no-referrer" class="mt-3 h-[420px] w-full border border-[var(--nx-border)] bg-white" />
      </div>
      <Pagination v-if="history.total" :page="page" :page-size="20" :total="history.total" :show-page-size-selector="false" @update:page="page = $event; load()" />
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { MonthlyLedgerEmailHistory } from '@/api/admin/monthlyLedger'
import { extractApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ month: string; recipient: { user_id: number; email: string } | null }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const tr = (key: string) => t(`admin.monthlyLedger.email.${key}`)
const history = ref<MonthlyLedgerEmailHistory | null>(null)
const page = ref(1), loading = ref(false), error = ref(''), selectedID = ref<number | null>(null)
let revision = 0
function safePreview(html: string) {
  return '<meta http-equiv="Content-Security-Policy" content="default-src \'none\'; style-src \'unsafe-inline\'; img-src data:; base-uri \'none\'; form-action \'none\'">' + DOMPurify.sanitize(html, { WHOLE_DOCUMENT: true, FORBID_TAGS: ['script', 'iframe', 'form', 'base', 'meta', 'link'] })
}
async function load() {
  const id = ++revision
  if (!props.recipient) return
  loading.value = true; error.value = ''; history.value = null; selectedID.value = null
  try {
    const result = await adminAPI.monthlyLedger.listEmailHistory(props.month, props.recipient.user_id, page.value)
    if (id === revision) history.value = result
  } catch (err) { if (id === revision) error.value = extractApiErrorMessage(err, t('admin.monthlyLedger.loadFailed')) }
  finally { if (id === revision) loading.value = false }
}
watch(() => [props.recipient?.user_id, props.month], () => { page.value = 1; void load() }, { immediate: true })
</script>
