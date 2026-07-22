import type {
  AdminPermission,
  AdminPermissionAction,
  AdminPermissionDefinition,
  AdminPermissionResource,
  UserRole,
} from '@/types'

export interface AdminPermissionPrincipal {
  role?: UserRole
  admin_permissions?: AdminPermission[]
}

export interface AdminRoutePermission {
  resource: AdminPermissionResource
  action: AdminPermissionAction
}

export interface AdminViewRoute extends AdminRoutePermission {
  name: string
  path: string
}

const allActions: AdminPermissionAction[] = ['view', 'create', 'update', 'delete', 'export', 'execute']

// This static mirror is intentionally limited to identifiers and route metadata
// required before the permissions-directory endpoint can be requested. The
// server remains the source for editor labels and allowed actions.
export const ADMIN_PERMISSION_DEFINITIONS: readonly AdminPermissionDefinition[] = [
  { resource: 'dashboard', label: 'Dashboard', actions: ['view', 'execute'], super_admin_only: false },
  { resource: 'ops', label: 'Operations', actions: allActions, super_admin_only: false },
  { resource: 'alert_rules', label: 'Alert Rules', actions: allActions, super_admin_only: false },
  { resource: 'alert_events', label: 'Alert Events', actions: ['view', 'update'], super_admin_only: false },
  { resource: 'alert_silences', label: 'Alert Silences', actions: ['view', 'create', 'delete'], super_admin_only: false },
  { resource: 'auth_cache_invalidation', label: 'Auth Cache Invalidation', actions: ['view', 'execute'], super_admin_only: false },
  { resource: 'users', label: 'Users', actions: allActions, super_admin_only: false },
  { resource: 'user_attributes', label: 'User Attributes', actions: allActions, super_admin_only: false },
  { resource: 'api_keys', label: 'API Keys', actions: ['view', 'update', 'execute'], super_admin_only: false },
  { resource: 'groups', label: 'Groups', actions: allActions, super_admin_only: false },
  { resource: 'accounts', label: 'Accounts', actions: allActions, super_admin_only: false },
  { resource: 'announcements', label: 'Announcements', actions: allActions, super_admin_only: false },
  { resource: 'proxies', label: 'Proxies', actions: allActions, super_admin_only: false },
  { resource: 'redeem_codes', label: 'Redeem Codes', actions: allActions, super_admin_only: false },
  { resource: 'promo_codes', label: 'Promo Codes', actions: allActions, super_admin_only: false },
  { resource: 'channels', label: 'Channels', actions: allActions, super_admin_only: false },
  { resource: 'channel_monitor', label: 'Channel Monitor', actions: allActions, super_admin_only: false },
  { resource: 'subscriptions', label: 'Subscriptions', actions: allActions, super_admin_only: false },
  { resource: 'usage', label: 'Usage', actions: ['view', 'create', 'execute'], super_admin_only: false },
  { resource: 'usage_interactions', label: 'Usage Interactions', actions: ['view'], super_admin_only: false },
  { resource: 'usage_interaction_raw', label: 'Usage Interaction Raw', actions: ['view'], super_admin_only: true },
  { resource: 'risk_control', label: 'Risk Control', actions: allActions, super_admin_only: false },
  { resource: 'prompt_audit', label: 'Prompt Audit', actions: allActions, super_admin_only: false },
  { resource: 'audit_logs', label: 'Audit Logs', actions: ['view', 'execute'], super_admin_only: false },
  { resource: 'affiliates', label: 'Affiliates', actions: allActions, super_admin_only: false },
  { resource: 'payment_dashboard', label: 'Payment Dashboard', actions: ['view'], super_admin_only: false },
  { resource: 'payment_orders', label: 'Payment Orders', actions: ['view', 'execute'], super_admin_only: false },
  { resource: 'payment_plans', label: 'Payment Plans', actions: allActions, super_admin_only: false },
  { resource: 'payment_providers', label: 'Payment Providers', actions: allActions, super_admin_only: false },
  { resource: 'payment_settings', label: 'Payment Settings', actions: ['view', 'update'], super_admin_only: false },
  { resource: 'data_management', label: 'Data Management', actions: allActions, super_admin_only: false },
  { resource: 'backups', label: 'Backups', actions: allActions, super_admin_only: false },
  { resource: 'settings', label: 'Settings', actions: allActions, super_admin_only: true },
  { resource: 'system', label: 'System', actions: allActions, super_admin_only: true },
  { resource: 'error_passthrough_rules', label: 'Error Passthrough Rules', actions: allActions, super_admin_only: false },
  { resource: 'tls_fingerprint_profiles', label: 'TLS Fingerprint Profiles', actions: allActions, super_admin_only: false },
  { resource: 'scheduled_tests', label: 'Scheduled Tests', actions: allActions, super_admin_only: false },
  { resource: 'pages', label: 'Pages', actions: ['view', 'update'], super_admin_only: false },
  { resource: 'admin_permissions', label: 'Admin Permissions', actions: allActions, super_admin_only: true },
]

export const ADMIN_ROUTE_PERMISSIONS: Readonly<Record<string, AdminRoutePermission>> = {
  AdminDashboard: { resource: 'dashboard', action: 'view' },
  AdminOps: { resource: 'ops', action: 'view' },
  AdminAuditLogs: { resource: 'audit_logs', action: 'view' },
  AdminUsers: { resource: 'users', action: 'view' },
  AdminGroups: { resource: 'groups', action: 'view' },
  AdminChannels: { resource: 'channels', action: 'view' },
  AdminChannelMonitor: { resource: 'channel_monitor', action: 'view' },
  AdminSubscriptions: { resource: 'subscriptions', action: 'view' },
  AdminAccounts: { resource: 'accounts', action: 'view' },
  AdminOpenAIQuotaSummary: { resource: 'accounts', action: 'view' },
  AdminAnnouncements: { resource: 'announcements', action: 'view' },
  AdminProxies: { resource: 'proxies', action: 'view' },
  AdminRedeem: { resource: 'redeem_codes', action: 'view' },
  AdminPromoCodes: { resource: 'promo_codes', action: 'view' },
  AdminSettings: { resource: 'settings', action: 'view' },
  AdminRiskControl: { resource: 'risk_control', action: 'view' },
  AdminPromptAudit: { resource: 'prompt_audit', action: 'view' },
  AdminUsage: { resource: 'usage', action: 'view' },
  AdminUsageInteraction: { resource: 'usage_interactions', action: 'view' },
  AdminAffiliateInvites: { resource: 'affiliates', action: 'view' },
  AdminAffiliateRebates: { resource: 'affiliates', action: 'view' },
  AdminAffiliateTransfers: { resource: 'affiliates', action: 'view' },
  AdminPaymentDashboard: { resource: 'payment_dashboard', action: 'view' },
  AdminOrders: { resource: 'payment_orders', action: 'view' },
  AdminPaymentPlans: { resource: 'payment_plans', action: 'view' },
}

export const ADMIN_VIEW_ROUTES: readonly AdminViewRoute[] = [
  { name: 'AdminDashboard', path: '/admin/dashboard', ...ADMIN_ROUTE_PERMISSIONS.AdminDashboard },
  { name: 'AdminOps', path: '/admin/ops', ...ADMIN_ROUTE_PERMISSIONS.AdminOps },
  { name: 'AdminUsers', path: '/admin/users', ...ADMIN_ROUTE_PERMISSIONS.AdminUsers },
  { name: 'AdminGroups', path: '/admin/groups', ...ADMIN_ROUTE_PERMISSIONS.AdminGroups },
  { name: 'AdminAccounts', path: '/admin/accounts', ...ADMIN_ROUTE_PERMISSIONS.AdminAccounts },
  { name: 'AdminOpenAIQuotaSummary', path: '/admin/openai-quota-summary', ...ADMIN_ROUTE_PERMISSIONS.AdminOpenAIQuotaSummary },
  { name: 'AdminChannels', path: '/admin/channels/pricing', ...ADMIN_ROUTE_PERMISSIONS.AdminChannels },
  { name: 'AdminChannelMonitor', path: '/admin/channels/monitor', ...ADMIN_ROUTE_PERMISSIONS.AdminChannelMonitor },
  { name: 'AdminSubscriptions', path: '/admin/subscriptions', ...ADMIN_ROUTE_PERMISSIONS.AdminSubscriptions },
  { name: 'AdminAnnouncements', path: '/admin/announcements', ...ADMIN_ROUTE_PERMISSIONS.AdminAnnouncements },
  { name: 'AdminProxies', path: '/admin/proxies', ...ADMIN_ROUTE_PERMISSIONS.AdminProxies },
  { name: 'AdminRedeem', path: '/admin/redeem', ...ADMIN_ROUTE_PERMISSIONS.AdminRedeem },
  { name: 'AdminPromoCodes', path: '/admin/promo-codes', ...ADMIN_ROUTE_PERMISSIONS.AdminPromoCodes },
  { name: 'AdminRiskControl', path: '/admin/risk-control', ...ADMIN_ROUTE_PERMISSIONS.AdminRiskControl },
  { name: 'AdminPromptAudit', path: '/admin/prompt-audit', ...ADMIN_ROUTE_PERMISSIONS.AdminPromptAudit },
  { name: 'AdminUsage', path: '/admin/usage', ...ADMIN_ROUTE_PERMISSIONS.AdminUsage },
  { name: 'AdminAuditLogs', path: '/admin/audit-logs', ...ADMIN_ROUTE_PERMISSIONS.AdminAuditLogs },
  { name: 'AdminAffiliateInvites', path: '/admin/affiliates/invites', ...ADMIN_ROUTE_PERMISSIONS.AdminAffiliateInvites },
  { name: 'AdminPaymentDashboard', path: '/admin/orders/dashboard', ...ADMIN_ROUTE_PERMISSIONS.AdminPaymentDashboard },
  { name: 'AdminOrders', path: '/admin/orders', ...ADMIN_ROUTE_PERMISSIONS.AdminOrders },
  { name: 'AdminPaymentPlans', path: '/admin/orders/plans', ...ADMIN_ROUTE_PERMISSIONS.AdminPaymentPlans },
  { name: 'AdminSettings', path: '/admin/settings', ...ADMIN_ROUTE_PERMISSIONS.AdminSettings },
]

function isKnownResource(resource: AdminPermissionResource): boolean {
  return ADMIN_PERMISSION_DEFINITIONS.some((definition) => definition.resource === resource)
}

export function isAdminLike(role?: UserRole): boolean {
  return role === 'admin' || role === 'super_admin'
}

export function canAdmin(
  principal: AdminPermissionPrincipal | null | undefined,
  resource: AdminPermissionResource,
  action: AdminPermissionAction,
): boolean {
  if (!principal || !isKnownResource(resource)) {
    return false
  }
  if (principal.role === 'super_admin') {
    return true
  }
  if (principal.role !== 'admin') {
    return false
  }

  const grant = principal.admin_permissions?.find((permission) => permission.resource === resource)
  if (!grant || !grant.actions.includes(action)) {
    return false
  }
  return action === 'view' || grant.actions.includes('view')
}

export function getFirstAllowedAdminRoute(principal: AdminPermissionPrincipal | null | undefined): string {
  return ADMIN_VIEW_ROUTES.find((route) => canAdmin(principal, route.resource, route.action))?.path ?? '/dashboard'
}
