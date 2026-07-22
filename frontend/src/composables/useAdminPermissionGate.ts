import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { AdminPermissionAction, AdminPermissionResource } from '@/types'

/**
 * Exposes reactive capability checks for one management resource. Components
 * use it for command visibility while the backend remains the enforcement
 * boundary for direct requests.
 */
export function useAdminPermissionGate(resource: AdminPermissionResource) {
  const authStore = useAuthStore()

  function can(action: AdminPermissionAction) {
    return computed(() => authStore.canAdmin(resource, action))
  }

  return {
    can,
  }
}
