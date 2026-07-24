package routes

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminRoutePermission is an exact route-to-capability mapping. The route path
// is Gin's full, parameterized path, not a request URL heuristic.
type AdminRoutePermission struct {
	Resource  service.AdminPermissionResource
	Action    service.AdminPermissionAction
	HumanOnly bool
}

var adminRoutePermissionManifest = buildAdminRoutePermissionManifest()

func buildAdminRoutePermissionManifest() map[string]AdminRoutePermission {
	manifest := make(map[string]AdminRoutePermission)
	add := func(resource service.AdminPermissionResource, action service.AdminPermissionAction, method string, paths ...string) {
		if !service.IsAdminPermissionRouteValid(resource, action) {
			panic(fmt.Sprintf("invalid admin route permission %s:%s", resource, action))
		}
		for _, path := range paths {
			key := adminRoutePermissionKey(method, path)
			if _, exists := manifest[key]; exists {
				panic(fmt.Sprintf("duplicate admin route permission: %s", key))
			}
			manifest[key] = AdminRoutePermission{Resource: resource, Action: action}
		}
	}
	humanOnly := func(method string, paths ...string) {
		for _, path := range paths {
			key := adminRoutePermissionKey(method, path)
			permission, ok := manifest[key]
			if !ok {
				panic(fmt.Sprintf("human-only route missing permission: %s", key))
			}
			permission.HumanOnly = true
			manifest[key] = permission
		}
	}

	const admin = "/api/v1/admin"

	add(service.AdminResourceDashboard, service.AdminActionView, http.MethodGet,
		admin+"/dashboard/snapshot-v2", admin+"/dashboard/stats", admin+"/dashboard/realtime",
		admin+"/dashboard/trend", admin+"/dashboard/models", admin+"/dashboard/groups",
		admin+"/dashboard/api-keys-trend", admin+"/dashboard/users-trend", admin+"/dashboard/users-ranking",
		admin+"/dashboard/user-breakdown")
	add(service.AdminResourceDashboard, service.AdminActionView, http.MethodPost,
		admin+"/dashboard/users-usage", admin+"/dashboard/api-keys-usage")
	add(service.AdminResourceDashboard, service.AdminActionExecute, http.MethodPost, admin+"/dashboard/aggregation/backfill")

	add(service.AdminResourceOps, service.AdminActionView, http.MethodGet,
		admin+"/ops/concurrency", admin+"/ops/user-concurrency", admin+"/ops/account-availability", admin+"/ops/realtime-traffic",
		admin+"/ops/email-notification/config", admin+"/ops/runtime/alert", admin+"/ops/runtime/logging",
		admin+"/ops/advanced-settings", admin+"/ops/settings/metric-thresholds", admin+"/ops/ws/qps",
		admin+"/ops/errors", admin+"/ops/errors/:id", admin+"/ops/request-errors", admin+"/ops/request-errors/:id",
		admin+"/ops/request-errors/:id/upstream-errors", admin+"/ops/ingress-rejections", admin+"/ops/ingress-rejections/health",
		admin+"/ops/upstream-errors", admin+"/ops/upstream-errors/:id", admin+"/ops/requests", admin+"/ops/system-logs",
		admin+"/ops/system-logs/health", admin+"/ops/storage", admin+"/ops/dashboard/snapshot-v2", admin+"/ops/dashboard/overview",
		admin+"/ops/dashboard/throughput-trend", admin+"/ops/dashboard/latency-histogram", admin+"/ops/dashboard/error-trend",
		admin+"/ops/dashboard/error-distribution", admin+"/ops/dashboard/openai-token-stats")
	add(service.AdminResourceOps, service.AdminActionUpdate, http.MethodPut,
		admin+"/ops/email-notification/config", admin+"/ops/runtime/alert", admin+"/ops/runtime/logging",
		admin+"/ops/advanced-settings", admin+"/ops/settings/metric-thresholds", admin+"/ops/errors/:id/resolve",
		admin+"/ops/request-errors/:id/resolve", admin+"/ops/upstream-errors/:id/resolve")
	add(service.AdminResourceOps, service.AdminActionExecute, http.MethodPost,
		admin+"/ops/runtime/logging/reset", admin+"/ops/system-logs/cleanup")
	add(service.AdminResourceAlertRules, service.AdminActionView, http.MethodGet, admin+"/ops/alert-rules")
	add(service.AdminResourceAlertRules, service.AdminActionCreate, http.MethodPost, admin+"/ops/alert-rules")
	add(service.AdminResourceAlertRules, service.AdminActionUpdate, http.MethodPut, admin+"/ops/alert-rules/:id")
	add(service.AdminResourceAlertRules, service.AdminActionDelete, http.MethodDelete, admin+"/ops/alert-rules/:id")
	add(service.AdminResourceAlertEvents, service.AdminActionView, http.MethodGet, admin+"/ops/alert-events", admin+"/ops/alert-events/:id")
	add(service.AdminResourceAlertEvents, service.AdminActionUpdate, http.MethodPut, admin+"/ops/alert-events/:id/status")
	add(service.AdminResourceAlertSilences, service.AdminActionCreate, http.MethodPost, admin+"/ops/alert-silences")
	add(service.AdminResourceAuthCacheInvalidation, service.AdminActionView, http.MethodGet, admin+"/ops/auth-cache-invalidation/health")

	add(service.AdminResourceUsers, service.AdminActionView, http.MethodGet,
		admin+"/users", admin+"/users/:id", admin+"/users/:id/usage",
		admin+"/users/:id/balance-history", admin+"/users/:id/rpm-status", admin+"/users/:id/platform-quotas")
	add(service.AdminResourceUsers, service.AdminActionCreate, http.MethodPost, admin+"/users")
	add(service.AdminResourceUsers, service.AdminActionUpdate, http.MethodPost,
		admin+"/users/:id/auth-identities", admin+"/users/:id/balance", admin+"/users/:id/replace-group",
		admin+"/users/batch-concurrency", admin+"/users/batch-limits")
	add(service.AdminResourceUsers, service.AdminActionUpdate, http.MethodPut, admin+"/users/:id", admin+"/users/:id/platform-quotas")
	add(service.AdminResourceUsers, service.AdminActionExecute, http.MethodPost, admin+"/users/:id/platform-quotas/reset")
	add(service.AdminResourceUsers, service.AdminActionDelete, http.MethodDelete, admin+"/users/:id")
	add(service.AdminResourceUserAttributes, service.AdminActionView, http.MethodGet,
		admin+"/users/:id/attributes", admin+"/user-attributes")
	add(service.AdminResourceUserAttributes, service.AdminActionView, http.MethodPost, admin+"/user-attributes/batch")
	add(service.AdminResourceUserAttributes, service.AdminActionCreate, http.MethodPost, admin+"/user-attributes")
	add(service.AdminResourceUserAttributes, service.AdminActionUpdate, http.MethodPut,
		admin+"/users/:id/attributes", admin+"/user-attributes/reorder", admin+"/user-attributes/:id")
	add(service.AdminResourceUserAttributes, service.AdminActionDelete, http.MethodDelete, admin+"/user-attributes/:id")
	add(service.AdminResourceAPIKeys, service.AdminActionView, http.MethodGet,
		admin+"/users/:id/api-keys", admin+"/groups/:id/api-keys")
	add(service.AdminResourceAPIKeys, service.AdminActionUpdate, http.MethodPut, admin+"/api-keys/:id")

	add(service.AdminResourceGroups, service.AdminActionView, http.MethodGet,
		admin+"/groups", admin+"/groups/all", admin+"/groups/usage-summary", admin+"/groups/capacity-summary",
		admin+"/groups/:id/models-list-candidates", admin+"/groups/:id", admin+"/groups/:id/stats",
		admin+"/groups/:id/rate-multipliers", admin+"/groups/:id/composite-routes")
	add(service.AdminResourceGroups, service.AdminActionCreate, http.MethodPost,
		admin+"/groups", admin+"/groups/:id/duplicate", admin+"/groups/:id/composite-routes")
	add(service.AdminResourceGroups, service.AdminActionExecute, http.MethodPost,
		admin+"/groups/:id/composite-routes/preview")
	add(service.AdminResourceGroups, service.AdminActionUpdate, http.MethodPut,
		admin+"/groups/sort-order", admin+"/groups/:id", admin+"/groups/:id/rate-multipliers", admin+"/groups/:id/rpm-overrides",
		admin+"/groups/:id/composite-routes/:route_id")
	add(service.AdminResourceGroups, service.AdminActionDelete, http.MethodDelete,
		admin+"/groups/:id", admin+"/groups/:id/rate-multipliers", admin+"/groups/:id/rpm-overrides",
		admin+"/groups/:id/composite-routes/:route_id")

	add(service.AdminResourceAccounts, service.AdminActionView, http.MethodGet,
		admin+"/accounts", admin+"/accounts/upstream-billing-probe/settings", admin+"/accounts/:id", admin+"/accounts/:id/stats",
		admin+"/accounts/:id/usage", admin+"/accounts/:id/today-stats", admin+"/accounts/:id/temp-unschedulable",
		admin+"/accounts/:id/models", admin+"/accounts/antigravity/default-model-mapping", admin+"/openai/accounts/:id/quota",
		admin+"/openai/quota-summary", admin+"/accounts/ollama-cloud-usage/settings", admin+"/accounts/:id/ollama-cloud-usage",
		admin+"/gemini/oauth/capabilities", admin+"/grok/accounts/:id/quota", admin+"/grok/runtime-sanity")
	add(service.AdminResourceAccounts, service.AdminActionExport, http.MethodGet, admin+"/accounts/data")
	add(service.AdminResourceAccounts, service.AdminActionCreate, http.MethodPost,
		admin+"/accounts", admin+"/accounts/batch", admin+"/accounts/data", admin+"/openai/create-from-oauth",
		admin+"/openai/create-from-codex-pat", admin+"/grok/oauth/create-from-oauth", admin+"/grok/sso-to-oauth")
	add(service.AdminResourceAccounts, service.AdminActionUpdate, http.MethodPut,
		admin+"/accounts/:id", admin+"/accounts/:id/upstream-billing-probe", admin+"/accounts/upstream-billing-probe/settings",
		admin+"/accounts/ollama-cloud-usage/settings", admin+"/accounts/:id/ollama-cloud-usage/session",
		admin+"/accounts/:id/ollama-cloud-usage/auto-refresh")
	add(service.AdminResourceAccounts, service.AdminActionUpdate, http.MethodDelete,
		admin+"/accounts/:id/ollama-cloud-usage/session")
	add(service.AdminResourceAccounts, service.AdminActionDelete, http.MethodDelete,
		admin+"/accounts/:id", admin+"/accounts/:id/temp-unschedulable")
	add(service.AdminResourceAccounts, service.AdminActionExecute, http.MethodPost,
		admin+"/accounts/upstream-billing-probe/batch", admin+"/accounts/:id/duplicate", admin+"/accounts/check-mixed-channel",
		admin+"/accounts/import/codex-session", admin+"/accounts/sync/crs", admin+"/accounts/sync/crs/preview",
		admin+"/accounts/:id/upstream-billing-probe", admin+"/accounts/:id/test", admin+"/accounts/:id/recover-state",
		admin+"/accounts/:id/refresh", admin+"/accounts/:id/apply-oauth-credentials", admin+"/accounts/:id/set-privacy",
		admin+"/accounts/:id/refresh-tier", admin+"/accounts/:id/clear-error", admin+"/accounts/:id/revert-proxy-fallback",
		admin+"/accounts/today-stats/batch", admin+"/accounts/:id/clear-rate-limit", admin+"/accounts/:id/reset-quota",
		admin+"/accounts/:id/schedulable", admin+"/accounts/models/sync-upstream-preview", admin+"/accounts/:id/models/sync-upstream",
		admin+"/accounts/batch-update-credentials", admin+"/accounts/batch-refresh-tier", admin+"/accounts/bulk-update",
		admin+"/accounts/batch-clear-error", admin+"/accounts/batch-refresh", admin+"/accounts/:id/shadow",
		admin+"/accounts/generate-auth-url", admin+"/accounts/generate-setup-token-url", admin+"/accounts/exchange-code",
		admin+"/accounts/exchange-setup-token-code", admin+"/accounts/cookie-auth", admin+"/accounts/setup-token-cookie-auth",
		admin+"/openai/generate-auth-url", admin+"/openai/exchange-code", admin+"/openai/refresh-token", admin+"/openai/accounts/:id/refresh",
		admin+"/openai/accounts/:id/reset-quota", admin+"/gemini/oauth/auth-url", admin+"/gemini/oauth/exchange-code",
		admin+"/antigravity/oauth/auth-url", admin+"/antigravity/oauth/exchange-code", admin+"/antigravity/oauth/refresh-token",
		admin+"/grok/oauth/auth-url", admin+"/grok/oauth/exchange-code", admin+"/grok/oauth/refresh-token",
		admin+"/grok/oauth/reconcile", admin+"/grok/accounts/:id/refresh", admin+"/grok/accounts/:id/reset-quota",
		admin+"/accounts/:id/ollama-cloud-usage/refresh")
	humanOnly(http.MethodPut, admin+"/accounts/:id/ollama-cloud-usage/session")
	humanOnly(http.MethodDelete, admin+"/accounts/:id/ollama-cloud-usage/session")

	add(service.AdminResourceAnnouncements, service.AdminActionView, http.MethodGet, admin+"/announcements", admin+"/announcements/:id", admin+"/announcements/:id/read-status")
	add(service.AdminResourceAnnouncements, service.AdminActionCreate, http.MethodPost, admin+"/announcements")
	add(service.AdminResourceAnnouncements, service.AdminActionUpdate, http.MethodPut, admin+"/announcements/:id")
	add(service.AdminResourceAnnouncements, service.AdminActionDelete, http.MethodDelete, admin+"/announcements/:id")

	add(service.AdminResourceProxies, service.AdminActionView, http.MethodGet,
		admin+"/proxies", admin+"/proxies/all", admin+"/proxies/:id", admin+"/proxies/:id/stats", admin+"/proxies/:id/accounts")
	add(service.AdminResourceProxies, service.AdminActionExport, http.MethodGet, admin+"/proxies/data")
	add(service.AdminResourceProxies, service.AdminActionCreate, http.MethodPost, admin+"/proxies", admin+"/proxies/data", admin+"/proxies/batch")
	add(service.AdminResourceProxies, service.AdminActionUpdate, http.MethodPut, admin+"/proxies/:id")
	add(service.AdminResourceProxies, service.AdminActionDelete, http.MethodDelete, admin+"/proxies/:id")
	add(service.AdminResourceProxies, service.AdminActionExecute, http.MethodPost,
		admin+"/proxies/:id/test", admin+"/proxies/:id/quality-check", admin+"/proxies/batch-delete")

	add(service.AdminResourceRedeemCodes, service.AdminActionView, http.MethodGet, admin+"/redeem-codes", admin+"/redeem-codes/stats", admin+"/redeem-codes/:id")
	add(service.AdminResourceRedeemCodes, service.AdminActionExport, http.MethodGet, admin+"/redeem-codes/export")
	add(service.AdminResourceRedeemCodes, service.AdminActionCreate, http.MethodPost, admin+"/redeem-codes/create-and-redeem", admin+"/redeem-codes/generate")
	add(service.AdminResourceRedeemCodes, service.AdminActionUpdate, http.MethodPost, admin+"/redeem-codes/batch-update", admin+"/redeem-codes/:id/expire")
	add(service.AdminResourceRedeemCodes, service.AdminActionDelete, http.MethodDelete, admin+"/redeem-codes/:id")
	add(service.AdminResourceRedeemCodes, service.AdminActionDelete, http.MethodPost, admin+"/redeem-codes/batch-delete")
	add(service.AdminResourcePromoCodes, service.AdminActionView, http.MethodGet, admin+"/promo-codes", admin+"/promo-codes/:id", admin+"/promo-codes/:id/usages")
	add(service.AdminResourcePromoCodes, service.AdminActionCreate, http.MethodPost, admin+"/promo-codes")
	add(service.AdminResourcePromoCodes, service.AdminActionUpdate, http.MethodPut, admin+"/promo-codes/:id")
	add(service.AdminResourcePromoCodes, service.AdminActionDelete, http.MethodDelete, admin+"/promo-codes/:id")

	add(service.AdminResourceSettings, service.AdminActionView, http.MethodGet,
		admin+"/settings", admin+"/settings/email-templates", admin+"/settings/email-templates/:event/:locale",
		admin+"/settings/admin-api-key", admin+"/settings/overload-cooldown", admin+"/settings/rate-limit-429-cooldown",
		admin+"/settings/stream-timeout", admin+"/settings/rectifier", admin+"/settings/beta-policy", admin+"/settings/web-search-emulation")
	add(service.AdminResourceSettings, service.AdminActionUpdate, http.MethodPut,
		admin+"/settings", admin+"/settings/email-templates/:event/:locale", admin+"/settings/overload-cooldown",
		admin+"/settings/rate-limit-429-cooldown", admin+"/settings/stream-timeout", admin+"/settings/rectifier",
		admin+"/settings/beta-policy", admin+"/settings/web-search-emulation")
	add(service.AdminResourceSettings, service.AdminActionExecute, http.MethodPost,
		admin+"/settings/test-smtp", admin+"/settings/send-test-email", admin+"/settings/email-template-preview",
		admin+"/settings/email-templates/:event/:locale/restore-official", admin+"/settings/admin-api-key/regenerate",
		admin+"/settings/web-search-emulation/test", admin+"/settings/web-search-emulation/reset-usage")
	add(service.AdminResourceSettings, service.AdminActionDelete, http.MethodDelete, admin+"/settings/admin-api-key")
	humanOnly(http.MethodGet, admin+"/settings/admin-api-key")
	humanOnly(http.MethodPost, admin+"/settings/admin-api-key/regenerate")
	humanOnly(http.MethodDelete, admin+"/settings/admin-api-key")

	add(service.AdminResourceDataManagement, service.AdminActionView, http.MethodGet,
		admin+"/data-management/agent/health", admin+"/data-management/config", admin+"/data-management/sources/:source_type/profiles",
		admin+"/data-management/s3/profiles", admin+"/data-management/backups", admin+"/data-management/backups/:job_id")
	add(service.AdminResourceDataManagement, service.AdminActionCreate, http.MethodPost,
		admin+"/data-management/sources/:source_type/profiles", admin+"/data-management/s3/profiles")
	add(service.AdminResourceDataManagement, service.AdminActionUpdate, http.MethodPut,
		admin+"/data-management/config", admin+"/data-management/sources/:source_type/profiles/:profile_id", admin+"/data-management/s3/profiles/:profile_id")
	add(service.AdminResourceDataManagement, service.AdminActionDelete, http.MethodDelete,
		admin+"/data-management/sources/:source_type/profiles/:profile_id", admin+"/data-management/s3/profiles/:profile_id")
	add(service.AdminResourceDataManagement, service.AdminActionExecute, http.MethodPost,
		admin+"/data-management/sources/:source_type/profiles/:profile_id/activate", admin+"/data-management/s3/test",
		admin+"/data-management/s3/profiles/:profile_id/activate", admin+"/data-management/backups")

	add(service.AdminResourceBackups, service.AdminActionView, http.MethodGet,
		admin+"/backups/s3-config", admin+"/backups/schedule", admin+"/backups", admin+"/backups/:id",
		admin+"/backups/image-storage")
	add(service.AdminResourceBackups, service.AdminActionExport, http.MethodGet, admin+"/backups/:id/download-url")
	add(service.AdminResourceBackups, service.AdminActionUpdate, http.MethodPut,
		admin+"/backups/s3-config", admin+"/backups/schedule", admin+"/backups/image-storage")
	add(service.AdminResourceBackups, service.AdminActionDelete, http.MethodDelete, admin+"/backups/:id")
	add(service.AdminResourceBackups, service.AdminActionExecute, http.MethodPost,
		admin+"/backups/s3-config/test", admin+"/backups", admin+"/backups/:id/restore",
		admin+"/backups/image-storage/test")
	humanOnly(http.MethodPut, admin+"/backups/image-storage")

	add(service.AdminResourceSystem, service.AdminActionView, http.MethodGet,
		admin+"/system/version", admin+"/system/check-updates", admin+"/system/rollback-versions")
	add(service.AdminResourceSystem, service.AdminActionExecute, http.MethodPost,
		admin+"/system/update", admin+"/system/rollback", admin+"/system/restart")

	add(service.AdminResourceSubscriptions, service.AdminActionView, http.MethodGet,
		admin+"/subscriptions", admin+"/subscriptions/:id", admin+"/subscriptions/:id/progress",
		admin+"/groups/:id/subscriptions", admin+"/users/:id/subscriptions")
	add(service.AdminResourceSubscriptions, service.AdminActionCreate, http.MethodPost, admin+"/subscriptions/assign", admin+"/subscriptions/bulk-assign")
	add(service.AdminResourceSubscriptions, service.AdminActionExecute, http.MethodPost,
		admin+"/subscriptions/:id/extend", admin+"/subscriptions/:id/reset-quota", admin+"/subscriptions/:id/revoke", admin+"/subscriptions/:id/restore")
	add(service.AdminResourceSubscriptions, service.AdminActionDelete, http.MethodDelete, admin+"/subscriptions/:id")
	add(service.AdminResourceUsage, service.AdminActionView, http.MethodGet,
		admin+"/usage", admin+"/usage/stats", admin+"/usage/search-users", admin+"/usage/search-api-keys", admin+"/usage/cleanup-tasks")
	add(service.AdminResourceUsage, service.AdminActionCreate, http.MethodPost, admin+"/usage/cleanup-tasks")
	add(service.AdminResourceUsage, service.AdminActionExecute, http.MethodPost, admin+"/usage/cleanup-tasks/:id/cancel")
	add(service.AdminResourceUsageInteractions, service.AdminActionView, http.MethodGet, admin+"/usage/:id/interaction")
	add(service.AdminResourceUsageInteractionRaw, service.AdminActionView, http.MethodGet, admin+"/usage/:id/interaction/raw")
	humanOnly(http.MethodGet, admin+"/usage/:id/interaction/raw")

	add(service.AdminResourceErrorPassthroughRules, service.AdminActionView, http.MethodGet, admin+"/error-passthrough-rules", admin+"/error-passthrough-rules/:id")
	add(service.AdminResourceErrorPassthroughRules, service.AdminActionCreate, http.MethodPost, admin+"/error-passthrough-rules")
	add(service.AdminResourceErrorPassthroughRules, service.AdminActionUpdate, http.MethodPut, admin+"/error-passthrough-rules/:id")
	add(service.AdminResourceErrorPassthroughRules, service.AdminActionDelete, http.MethodDelete, admin+"/error-passthrough-rules/:id")
	add(service.AdminResourceTLSFingerprintProfiles, service.AdminActionView, http.MethodGet, admin+"/tls-fingerprint-profiles", admin+"/tls-fingerprint-profiles/:id")
	add(service.AdminResourceTLSFingerprintProfiles, service.AdminActionCreate, http.MethodPost, admin+"/tls-fingerprint-profiles")
	add(service.AdminResourceTLSFingerprintProfiles, service.AdminActionUpdate, http.MethodPut, admin+"/tls-fingerprint-profiles/:id")
	add(service.AdminResourceTLSFingerprintProfiles, service.AdminActionDelete, http.MethodDelete, admin+"/tls-fingerprint-profiles/:id")

	add(service.AdminResourceScheduledTests, service.AdminActionCreate, http.MethodPost, admin+"/scheduled-test-plans")
	add(service.AdminResourceScheduledTests, service.AdminActionUpdate, http.MethodPut, admin+"/scheduled-test-plans/:id")
	add(service.AdminResourceScheduledTests, service.AdminActionDelete, http.MethodDelete, admin+"/scheduled-test-plans/:id")
	add(service.AdminResourceScheduledTests, service.AdminActionView, http.MethodGet,
		admin+"/scheduled-test-plans/:id/results", admin+"/accounts/:id/scheduled-test-plans")

	add(service.AdminResourceChannels, service.AdminActionView, http.MethodGet,
		admin+"/channels", admin+"/channels/model-pricing", admin+"/channels/:id")
	add(service.AdminResourceChannels, service.AdminActionExecute, http.MethodGet, admin+"/channels/pricing/sync-models")
	add(service.AdminResourceChannels, service.AdminActionCreate, http.MethodPost, admin+"/channels")
	add(service.AdminResourceChannels, service.AdminActionUpdate, http.MethodPut, admin+"/channels/:id")
	add(service.AdminResourceChannels, service.AdminActionDelete, http.MethodDelete, admin+"/channels/:id")
	add(service.AdminResourceChannelMonitor, service.AdminActionView, http.MethodGet,
		admin+"/channel-monitors", admin+"/channel-monitors/:id", admin+"/channel-monitors/:id/history",
		admin+"/channel-monitor-templates", admin+"/channel-monitor-templates/:id", admin+"/channel-monitor-templates/:id/monitors")
	add(service.AdminResourceChannelMonitor, service.AdminActionCreate, http.MethodPost,
		admin+"/channel-monitors", admin+"/channel-monitors/:id/duplicate", admin+"/channel-monitor-templates")
	add(service.AdminResourceChannelMonitor, service.AdminActionUpdate, http.MethodPut,
		admin+"/channel-monitors/:id", admin+"/channel-monitor-templates/:id")
	add(service.AdminResourceChannelMonitor, service.AdminActionDelete, http.MethodDelete,
		admin+"/channel-monitors/:id", admin+"/channel-monitor-templates/:id")
	add(service.AdminResourceChannelMonitor, service.AdminActionExecute, http.MethodPost,
		admin+"/channel-monitors/:id/run", admin+"/channel-monitor-templates/:id/apply")

	add(service.AdminResourceRiskControl, service.AdminActionView, http.MethodGet,
		admin+"/risk-control/config", admin+"/risk-control/status", admin+"/risk-control/logs")
	add(service.AdminResourceRiskControl, service.AdminActionUpdate, http.MethodPut, admin+"/risk-control/config")
	add(service.AdminResourceRiskControl, service.AdminActionExecute, http.MethodPost,
		admin+"/risk-control/api-keys/test", admin+"/risk-control/users/:user_id/unban")
	add(service.AdminResourceRiskControl, service.AdminActionDelete, http.MethodDelete, admin+"/risk-control/hashes", admin+"/risk-control/hashes/all")
	add(service.AdminResourcePromptAudit, service.AdminActionView, http.MethodGet,
		admin+"/prompt-audit/config", admin+"/prompt-audit/runtime", admin+"/prompt-audit/events", admin+"/prompt-audit/events/:id")
	add(service.AdminResourcePromptAudit, service.AdminActionUpdate, http.MethodPut, admin+"/prompt-audit/config")
	add(service.AdminResourcePromptAudit, service.AdminActionExecute, http.MethodPost,
		admin+"/prompt-audit/endpoints/probe", admin+"/prompt-audit/events/delete-preview", admin+"/prompt-audit/events/delete-by-filter")
	add(service.AdminResourcePromptAudit, service.AdminActionDelete, http.MethodDelete, admin+"/prompt-audit/events/:id")
	add(service.AdminResourcePromptAudit, service.AdminActionDelete, http.MethodPost, admin+"/prompt-audit/events/batch-delete")
	add(service.AdminResourceAuditLogs, service.AdminActionView, http.MethodGet, admin+"/audit-logs", admin+"/audit-logs/:id")
	add(service.AdminResourceAuditLogs, service.AdminActionExecute, http.MethodPost, admin+"/audit-logs/clear")

	add(service.AdminResourceAffiliates, service.AdminActionView, http.MethodGet,
		admin+"/affiliates/invites", admin+"/affiliates/rebates", admin+"/affiliates/transfers",
		admin+"/affiliates/users", admin+"/affiliates/users/lookup", admin+"/affiliates/users/:user_id/overview")
	add(service.AdminResourceAffiliates, service.AdminActionUpdate, http.MethodPost, admin+"/affiliates/users/batch-rate")
	add(service.AdminResourceAffiliates, service.AdminActionUpdate, http.MethodPut, admin+"/affiliates/users/:user_id")
	add(service.AdminResourceAffiliates, service.AdminActionDelete, http.MethodDelete, admin+"/affiliates/users/:user_id")

	add(service.AdminResourcePaymentDashboard, service.AdminActionView, http.MethodGet, admin+"/payment/dashboard")
	add(service.AdminResourcePaymentSettings, service.AdminActionView, http.MethodGet, admin+"/payment/config")
	add(service.AdminResourcePaymentSettings, service.AdminActionUpdate, http.MethodPut, admin+"/payment/config")
	add(service.AdminResourcePaymentOrders, service.AdminActionView, http.MethodGet, admin+"/payment/orders", admin+"/payment/orders/:id")
	add(service.AdminResourcePaymentOrders, service.AdminActionExecute, http.MethodPost,
		admin+"/payment/orders/:id/cancel", admin+"/payment/orders/:id/retry", admin+"/payment/orders/:id/refund", admin+"/payment/orders/:id/refund/query")
	add(service.AdminResourcePaymentPlans, service.AdminActionView, http.MethodGet, admin+"/payment/plans")
	add(service.AdminResourcePaymentPlans, service.AdminActionCreate, http.MethodPost, admin+"/payment/plans")
	add(service.AdminResourcePaymentPlans, service.AdminActionUpdate, http.MethodPut, admin+"/payment/plans/:id")
	add(service.AdminResourcePaymentPlans, service.AdminActionDelete, http.MethodDelete, admin+"/payment/plans/:id")
	add(service.AdminResourcePaymentProviders, service.AdminActionView, http.MethodGet, admin+"/payment/providers")
	add(service.AdminResourcePaymentProviders, service.AdminActionCreate, http.MethodPost, admin+"/payment/providers")
	add(service.AdminResourcePaymentProviders, service.AdminActionUpdate, http.MethodPut, admin+"/payment/providers/:id")
	add(service.AdminResourcePaymentProviders, service.AdminActionDelete, http.MethodDelete, admin+"/payment/providers/:id")

	add(service.AdminResourcePages, service.AdminActionView, http.MethodGet, "/api/v1/pages")
	add(service.AdminResourceAdminPermissions, service.AdminActionView, http.MethodGet, admin+"/admin-permissions")
	humanOnly(http.MethodGet, admin+"/admin-permissions")

	return manifest
}

func adminRoutePermissionKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

// AdminRoutePermissionFor returns a copy of an explicitly registered mapping.
func AdminRoutePermissionFor(method, path string) (AdminRoutePermission, bool) {
	permission, ok := adminRoutePermissionManifest[adminRoutePermissionKey(method, path)]
	return permission, ok
}

// RequireAdminRoutePermission gates every route in the admin group. An
// unmapped route is denied; there is no route, method, or dashboard fallback.
func RequireAdminRoutePermission(permissionMiddleware middleware.AdminPermissionMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		permission, ok := AdminRoutePermissionFor(c.Request.Method, c.FullPath())
		if !ok {
			middleware.AbortWithError(c, http.StatusForbidden, "ADMIN_PERMISSION_DENIED", "Admin permission denied")
			return
		}
		if permission.HumanOnly {
			subject, subjectOK := middleware.GetAuthSubjectFromContext(c)
			role, roleOK := middleware.GetUserRoleFromContext(c)
			if !subjectOK || !subject.IsHuman() {
				middleware.AbortWithError(c, http.StatusForbidden, "MACHINE_CREDENTIAL_FORBIDDEN", "This operation requires a human administrator")
				return
			}
			if !roleOK || role != service.RoleSuperAdmin {
				middleware.AbortWithError(c, http.StatusForbidden, "SUPER_ADMIN_REQUIRED", "Super administrator access required")
				return
			}
		}
		permissionMiddleware(permission.Resource, permission.Action)(c)
	}
}

// ValidateAdminRouteManifest compares the authoritative manifest against the
// router's actual protected paths. It is invoked on startup and exercised in
// route tests so newly added admin endpoints cannot silently bypass RBAC.
func ValidateAdminRouteManifest(routes []gin.RouteInfo) error {
	actual := make(map[string]struct{})
	for _, route := range routes {
		key := adminRoutePermissionKey(route.Method, route.Path)
		if strings.HasPrefix(route.Path, "/api/v1/admin/") || key == adminRoutePermissionKey(http.MethodGet, "/api/v1/pages") {
			actual[key] = struct{}{}
			if _, ok := adminRoutePermissionManifest[key]; !ok {
				return fmt.Errorf("protected admin route missing permission manifest entry: %s", key)
			}
		}
	}
	missing := make([]string, 0)
	for key := range adminRoutePermissionManifest {
		if _, ok := actual[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("admin permission manifest contains unregistered route(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
