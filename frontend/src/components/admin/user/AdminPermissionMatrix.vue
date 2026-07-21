<template>
  <fieldset class="space-y-2">
    <legend class="input-label">{{ t('admin.users.adminPermissions') }}</legend>
    <div class="overflow-hidden rounded-md border border-[var(--nx-border)]">
      <div
        v-for="definition in grantableDefinitions"
        :key="definition.resource"
        class="border-b border-[var(--nx-border)] px-3 py-2 last:border-b-0"
      >
        <div class="mb-2 text-sm font-medium text-[var(--nx-text)]">{{ definition.label }}</div>
        <div class="flex flex-wrap gap-x-4 gap-y-2">
          <label
            v-for="action in definition.actions"
            :key="action"
            class="inline-flex cursor-pointer items-center gap-1.5 text-sm text-[var(--nx-text-muted)]"
          >
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-[var(--nx-border)] text-[var(--nx-accent)] focus:ring-[var(--nx-accent)]"
              :checked="hasAction(definition.resource, action)"
              @change="toggleAction(definition.resource, action, ($event.target as HTMLInputElement).checked)"
            />
            <span>{{ actionLabel(action) }}</span>
          </label>
        </div>
      </div>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  AdminPermission,
  AdminPermissionAction,
  AdminPermissionDefinition,
  AdminPermissionResource,
} from '@/types'

const props = defineProps<{
  modelValue: AdminPermission[]
  definitions: AdminPermissionDefinition[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: AdminPermission[]]
}>()

const { t } = useI18n()

const grantableDefinitions = computed(() =>
  props.definitions.filter((definition) => !definition.super_admin_only)
)

function hasAction(resource: AdminPermissionResource, action: AdminPermissionAction): boolean {
  return props.modelValue.some(
    (permission) => permission.resource === resource && permission.actions.includes(action)
  )
}

function toggleAction(
  resource: AdminPermissionResource,
  action: AdminPermissionAction,
  checked: boolean,
): void {
  const byResource = new Map<AdminPermissionResource, Set<AdminPermissionAction>>()
  for (const permission of props.modelValue) {
    byResource.set(permission.resource, new Set(permission.actions))
  }

  const actions = byResource.get(resource) ?? new Set<AdminPermissionAction>()
  if (checked) {
    actions.add(action)
    if (action !== 'view') {
      actions.add('view')
    }
    byResource.set(resource, actions)
  } else if (action === 'view') {
    byResource.delete(resource)
  } else {
    actions.delete(action)
    if (actions.size > 0) {
      byResource.set(resource, actions)
    }
  }

  const actionOrder: AdminPermissionAction[] = ['view', 'create', 'update', 'delete', 'export', 'execute']
  const next = Array.from(byResource.entries())
    .filter(([, actions]) => actions.size > 0)
    .map(([nextResource, actions]) => ({
      resource: nextResource,
      actions: actionOrder.filter((candidate) => actions.has(candidate)),
    }))
    .sort((left, right) => left.resource.localeCompare(right.resource))
  emit('update:modelValue', next)
}

function actionLabel(action: AdminPermissionAction): string {
  return action.charAt(0).toUpperCase() + action.slice(1)
}
</script>
