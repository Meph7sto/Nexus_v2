<template>
  <BaseDialog :show="show" :title="t('admin.monthlyLedger.email.title')" :show-close-button="!batchSaving" :close-on-escape="!batchSaving" @close="$emit('close')">
    <form class="mb-4 flex gap-2" @submit.prevent="search">
      <input v-model="query" class="input min-w-0 flex-1" :disabled="batchSaving" :placeholder="t('admin.monthlyLedger.searchPlaceholder')" :aria-label="t('common.search')" />
      <button class="btn btn-secondary" type="submit" :disabled="loading || batchSaving">{{ t('common.search') }}</button>
    </form>
    <div v-if="canUpdate" class="mb-3 flex flex-wrap items-center gap-3 border-b border-[var(--nx-border)] pb-3">
      <label class="flex items-center gap-2 text-sm">
        <input type="checkbox" data-test="email-select-page" :checked="allVisibleSelected" :indeterminate="someVisibleSelected && !allVisibleSelected" :disabled="loading || batchSaving || !users.length" @change="toggleVisible(($event.target as HTMLInputElement).checked)" />
        {{ t('admin.monthlyLedger.email.selectPage') }}
      </label>
      <span class="text-sm text-[var(--nx-subtle)]">{{ t('admin.monthlyLedger.email.selectedCount', { count: selectedCount }) }}</span>
      <button v-if="selectedCount" type="button" class="btn btn-ghost px-2" :disabled="batchSaving" :title="t('admin.monthlyLedger.bulk.clear')" :aria-label="t('admin.monthlyLedger.bulk.clear')" @click="clearSelection"><Icon name="x" size="sm" /></button>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary" data-test="email-batch-enable" :disabled="!selectedCount || loading || batchSaving || saving.size > 0" @click="batchToggle(true)"><Icon name="check" size="sm" />{{ t('admin.monthlyLedger.email.batchEnable') }}</button>
        <button type="button" class="btn btn-secondary" data-test="email-batch-disable" :disabled="!selectedCount || loading || batchSaving || saving.size > 0" @click="batchToggle(false)"><Icon name="x" size="sm" />{{ t('admin.monthlyLedger.email.batchDisable') }}</button>
      </div>
    </div>
    <div v-if="loading" class="py-6 text-center">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="py-4 text-[var(--nx-danger)]" role="alert">{{ error }}</div>
    <ul v-else class="divide-y divide-[var(--nx-border)]">
      <li v-for="user in users" :key="user.user_id" class="flex items-center justify-between gap-4 py-3">
        <input v-if="canUpdate" type="checkbox" :data-test="`email-select-${user.user_id}`" :checked="isSelected(user.user_id)" :disabled="batchSaving" :aria-label="`${t('admin.monthlyLedger.email.selectUser')} ${user.email}`" @change="toggleSelection(user.user_id)" />
        <div class="min-w-0 flex-1">
          <div class="truncate font-medium" :title="user.email">{{ user.email }}</div>
          <div class="truncate text-xs text-[var(--nx-subtle)]">{{ user.username || `#${user.user_id}` }}</div>
          <div v-if="user.enabled && user.effective_month" class="text-xs text-[var(--nx-subtle)]">{{ t('admin.monthlyLedger.email.effectiveMonth', { month: user.effective_month }) }}</div>
        </div>
        <label class="flex shrink-0 items-center gap-2 text-sm">
          <input type="checkbox" :checked="user.enabled" :disabled="!canUpdate || batchSaving || saving.has(user.user_id)" :aria-label="`${t('admin.monthlyLedger.email.title')} ${user.email}`" :data-test="`email-user-${user.user_id}`" @change="toggle(user, $event)" />
          {{ t(user.enabled ? 'admin.monthlyLedger.email.enabled' : 'admin.monthlyLedger.email.disabled') }}
        </label>
      </li>
      <li v-if="!users.length" class="py-6 text-center">{{ t('admin.monthlyLedger.noData') }}</li>
    </ul>
    <div class="mt-4 flex items-center justify-between gap-3">
      <button type="button" class="btn btn-secondary" :disabled="loading || batchSaving || page <= 1" @click="changePage(-1)">{{ t('common.back') }}</button>
      <span class="text-sm">{{ page }} / {{ Math.max(1, Math.ceil(total / 20)) }}</span>
      <button type="button" class="btn btn-secondary" :disabled="loading || batchSaving || page * 20 >= total" @click="changePage(1)">{{ t('common.next') }}</button>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useTableSelection } from '@/composables/useTableSelection'
import { adminAPI } from '@/api/admin'
import type { MonthlyLedgerEmailPreference } from '@/api/admin/monthlyLedger'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const canUpdate = computed(() => auth.canAdmin('monthly_ledger', 'update'))
const query = ref('')
const page = ref(1)
const total = ref(0)
const users = ref<MonthlyLedgerEmailPreference[]>([])
const loading = ref(false)
const error = ref('')
const saving = ref(new Set<number>())
const batchSaving = ref(false)
const { selectedIds, selectedCount, allVisibleSelected, isSelected, toggle: toggleSelection, toggleVisible, clear: clearSelection } = useTableSelection({ rows: users, getId: user => user.user_id })
const someVisibleSelected = computed(() => users.value.some(user => isSelected(user.user_id)))
let request = 0

async function load() {
  const id = ++request
  loading.value = true
  error.value = ''
  try {
    const result = await adminAPI.monthlyLedger.listEmailPreferences({ q: query.value.trim(), page: page.value, page_size: 20 })
    if (id !== request) return
    users.value = result.items
    total.value = result.total
  } catch {
    if (id === request) error.value = t('admin.monthlyLedger.loadFailed')
  } finally {
    if (id === request) loading.value = false
  }
}

function search() { if (batchSaving.value) return; clearSelection(); page.value = 1; void load() }
function changePage(delta: number) { page.value += delta; void load() }
async function batchToggle(enabled: boolean) {
  if (!canUpdate.value || batchSaving.value || loading.value || saving.value.size || !selectedCount.value) return
  batchSaving.value = true
  try {
    const result = await adminAPI.monthlyLedger.setEmailPreferences([...selectedIds.value], enabled)
    clearSelection()
    app.showSuccess(t('admin.monthlyLedger.email.batchSaved', { count: result.updated_count }))
    await load()
  } catch {
    app.showError(t('admin.monthlyLedger.saveFailed'))
  } finally { batchSaving.value = false }
}
async function toggle(user: MonthlyLedgerEmailPreference, event: Event) {
  (event.target as HTMLInputElement).checked = user.enabled
  if (!canUpdate.value || batchSaving.value || saving.value.has(user.user_id)) return
  saving.value.add(user.user_id)
  const enabled = !user.enabled
  try {
    await adminAPI.monthlyLedger.setEmailPreference(user.user_id, enabled)
    user.enabled = enabled
    app.showSuccess(t('admin.monthlyLedger.saved'))
    await load()
  } catch {
    app.showError(t('admin.monthlyLedger.saveFailed'))
  } finally {
    saving.value.delete(user.user_id)
  }
}
watch(() => props.show, show => {
  if (show) { clearSelection(); page.value = 1; query.value = ''; void load() }
  else { request++ }
})
</script>
