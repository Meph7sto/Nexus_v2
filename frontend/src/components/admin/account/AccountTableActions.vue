<template>
  <div class="flex flex-wrap items-center gap-3">
    <slot name="before"></slot>
    <button @click="$emit('refresh')" :disabled="loading" class="btn btn-secondary">
      <Icon name="refresh" size="md" :class="[loading ? 'animate-spin' : '']" />
    </button>
    <slot name="after"></slot>
    <slot name="beforeCreate"></slot>
    <AdminPermissionGate resource="accounts" action="create">
      <button @click="$emit('create')" class="btn btn-primary">{{ t('admin.accounts.createAccount') }}</button>
    </AdminPermissionGate>
    <slot name="afterCreate"></slot>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import AdminPermissionGate from '@/components/admin/AdminPermissionGate.vue'

defineProps(['loading'])
defineEmits(['refresh', 'create'])

const { t } = useI18n()
</script>
