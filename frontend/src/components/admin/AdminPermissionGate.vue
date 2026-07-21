<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { AdminPermissionAction, AdminPermissionResource } from '@/types'

const props = defineProps<{
  resource: AdminPermissionResource
  action: AdminPermissionAction
}>()

const authStore = useAuthStore()
const allowed = computed(() => authStore.canAdmin(props.resource, props.action))
</script>

<template>
  <template v-if="allowed">
    <slot />
  </template>
</template>
