<template>
  <BaseDialog :show="show" :title="t('admin.monthlyLedger.email.sendTitle')" :show-close-button="!sending" :close-on-escape="!sending" @close="$emit('close')">
    <div class="space-y-5">
      <div class="flex flex-col gap-3 border-b border-[var(--nx-border)] pb-4 sm:flex-row sm:items-end">
        <div class="min-w-0 flex-1">
          <p v-if="quota" class="text-sm" data-test="email-quota">{{ t('admin.monthlyLedger.email.quota', { date: quota.date, used: quota.used, limit: quota.daily_limit }) }}</p>
          <p class="mt-1 text-xs text-[var(--nx-subtle)]">{{ t('admin.monthlyLedger.email.quotaHint') }}</p>
        </div>
        <form v-if="canUpdate" class="flex shrink-0 items-end gap-2" @submit.prevent="saveLimit">
          <div class="w-24">
            <label for="email-daily-limit" class="input-label">{{ t('admin.monthlyLedger.email.dailyLimit') }}</label>
            <input id="email-daily-limit" v-model.number="dailyLimit" type="number" min="0" max="2147483647" step="1" required class="input w-full" :disabled="savingLimit || sending || !quota" />
          </div>
          <button type="submit" class="btn btn-secondary px-3" :title="t('common.save')" :aria-label="t('common.save')" :disabled="savingLimit || sending || !quota">
            <Icon name="check" size="sm" />
          </button>
        </form>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label for="email-billing-month" class="input-label">{{ t('admin.monthlyLedger.month') }}</label>
          <input id="email-billing-month" :value="month" type="month" readonly class="input w-full" />
        </div>
        <div>
          <label for="email-amount" class="input-label">{{ t('admin.monthlyLedger.email.amount') }} (USD)</label>
          <input id="email-amount" v-model="amount" type="number" min="0.01" step="0.01" class="input w-full" :disabled="sending" />
        </div>
      </div>

      <div v-if="selected" class="border-t border-[var(--nx-border)] pt-3 text-sm" data-test="email-summary">
        <p class="mb-1 text-[var(--nx-subtle)]">{{ t('admin.monthlyLedger.email.recipient') }}</p>
        <p class="break-all">{{ selected.email }}</p>
        <p class="mt-1 font-medium">{{ month }} · {{ validAmount ? Number(amount).toFixed(2) : '—' }} USD</p>
      </div>
      <p v-if="error" role="alert" class="text-sm text-[var(--nx-danger)]">{{ error }}</p>
    </div>
    <template #footer>
      <button class="btn btn-secondary" type="button" :disabled="sending" @click="$emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" type="button" data-test="send-ledger-email" :disabled="!canSend || sending || savingLimit || !selected || !validMonth || !validAmount || !quota || quota.used >= quota.daily_limit" @click="send">
        <Icon name="mail" size="sm" />{{ t(sending ? 'admin.monthlyLedger.email.sending' : 'admin.monthlyLedger.email.send') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { MonthlyLedgerRow, MonthlyLedgerEmailQuota } from '@/api/admin/monthlyLedger'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ show: boolean; initialMonth: string; recipient: MonthlyLedgerRow | null }>()
const emit = defineEmits<{ close: []; sent: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const canSend = computed(() => auth.canAdmin('monthly_ledger', 'create'))
const canUpdate = computed(() => auth.canAdmin('monthly_ledger', 'update'))
const quota = ref<MonthlyLedgerEmailQuota | null>(null)
const dailyLimit = ref(5)
const month = ref('')
const amount = ref('')
const selected = ref<MonthlyLedgerRow | null>(null)
const sending = ref(false)
const savingLimit = ref(false)
const error = ref('')
let revision = 0
let requestID = ''
const validMonth = computed(() => /^(?!0000)\d{4}-(0[1-9]|1[0-2])$/.test(month.value))
const validAmount = computed(() => /^\d+(\.\d{1,2})?$/.test(String(amount.value)) && Number.isFinite(Number(amount.value)) && Number(amount.value) > 0)

function message(err: unknown) {
  const reason = (err as { reason?: string })?.reason
  if (reason === 'MONTHLY_LEDGER_EMAIL_DAILY_LIMIT') return t('admin.monthlyLedger.email.limitReached')
  return (err as { message?: string })?.message || t('admin.monthlyLedger.email.failed')
}

async function loadQuota() {
  const id = revision
  try {
    const result = await adminAPI.monthlyLedger.getEmailQuota()
    if (id !== revision) return
    quota.value = result
    dailyLimit.value = result.daily_limit
  } catch (err) {
    if (id === revision) { quota.value = null; error.value = message(err) }
  }
}

async function saveLimit() {
  if (!canUpdate.value || savingLimit.value || sending.value || !Number.isInteger(dailyLimit.value) || dailyLimit.value < 0 || dailyLimit.value > 2147483647) return
  savingLimit.value = true
  error.value = ''
  try {
    await adminAPI.monthlyLedger.setEmailDailyLimit(dailyLimit.value)
    await loadQuota()
    app.showSuccess(t('admin.monthlyLedger.saved'))
  } catch (err) { error.value = message(err) }
  finally { savingLimit.value = false }
}

async function send() {
  if (!canSend.value || sending.value || savingLimit.value || !selected.value || !validMonth.value || !validAmount.value || !quota.value || quota.value.used >= quota.value.daily_limit) return
  sending.value = true
  error.value = ''
  try {
    requestID ||= crypto.randomUUID()
    await adminAPI.monthlyLedger.sendManualEmail({ month: month.value, user_id: selected.value.user_id, amount: Number(amount.value), request_id: requestID })
    app.showSuccess(t('admin.monthlyLedger.email.sent'))
    emit('sent')
    emit('close')
  } catch (err) { error.value = message(err) }
  finally { await loadQuota(); sending.value = false }
}

watch([month, amount, () => selected.value?.user_id], () => { requestID = '' })
watch(() => props.show, async show => {
  if (!show) { revision++; return }
  month.value = props.initialMonth
  selected.value = props.recipient ? { ...props.recipient } : null
  amount.value = props.recipient ? props.recipient.outstanding_amount.toFixed(2) : ''
  quota.value = null
  error.value = ''
  requestID = ''
  await loadQuota()
}, { immediate: true })
</script>
