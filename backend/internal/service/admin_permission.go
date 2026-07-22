package service

import (
	"context"
	"fmt"
	"sort"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AdminPermissionAction is an explicit capability on an admin resource.
// HTTP methods are intentionally not used to infer these capabilities.
type AdminPermissionAction string

// AdminPermissionResource identifies one independently grantable admin area.
type AdminPermissionResource string

const (
	AdminActionView    AdminPermissionAction = "view"
	AdminActionCreate  AdminPermissionAction = "create"
	AdminActionUpdate  AdminPermissionAction = "update"
	AdminActionDelete  AdminPermissionAction = "delete"
	AdminActionExport  AdminPermissionAction = "export"
	AdminActionExecute AdminPermissionAction = "execute"
)

const (
	AdminResourceDashboard              AdminPermissionResource = "dashboard"
	AdminResourceOps                    AdminPermissionResource = "ops"
	AdminResourceAlertRules             AdminPermissionResource = "alert_rules"
	AdminResourceAlertEvents            AdminPermissionResource = "alert_events"
	AdminResourceAlertSilences          AdminPermissionResource = "alert_silences"
	AdminResourceAuthCacheInvalidation  AdminPermissionResource = "auth_cache_invalidation"
	AdminResourceUsers                  AdminPermissionResource = "users"
	AdminResourceUserAttributes         AdminPermissionResource = "user_attributes"
	AdminResourceAPIKeys                AdminPermissionResource = "api_keys"
	AdminResourceGroups                 AdminPermissionResource = "groups"
	AdminResourceAccounts               AdminPermissionResource = "accounts"
	AdminResourceAnnouncements          AdminPermissionResource = "announcements"
	AdminResourceProxies                AdminPermissionResource = "proxies"
	AdminResourceRedeemCodes            AdminPermissionResource = "redeem_codes"
	AdminResourcePromoCodes             AdminPermissionResource = "promo_codes"
	AdminResourceChannels               AdminPermissionResource = "channels"
	AdminResourceChannelMonitor         AdminPermissionResource = "channel_monitor"
	AdminResourceSubscriptions          AdminPermissionResource = "subscriptions"
	AdminResourceUsage                  AdminPermissionResource = "usage"
	AdminResourceRiskControl            AdminPermissionResource = "risk_control"
	AdminResourcePromptAudit            AdminPermissionResource = "prompt_audit"
	AdminResourceAuditLogs              AdminPermissionResource = "audit_logs"
	AdminResourceAffiliates             AdminPermissionResource = "affiliates"
	AdminResourcePaymentDashboard       AdminPermissionResource = "payment_dashboard"
	AdminResourcePaymentOrders          AdminPermissionResource = "payment_orders"
	AdminResourcePaymentPlans           AdminPermissionResource = "payment_plans"
	AdminResourcePaymentProviders       AdminPermissionResource = "payment_providers"
	AdminResourcePaymentSettings        AdminPermissionResource = "payment_settings"
	AdminResourceDataManagement         AdminPermissionResource = "data_management"
	AdminResourceBackups                AdminPermissionResource = "backups"
	AdminResourceSettings               AdminPermissionResource = "settings"
	AdminResourceSystem                 AdminPermissionResource = "system"
	AdminResourceErrorPassthroughRules  AdminPermissionResource = "error_passthrough_rules"
	AdminResourceTLSFingerprintProfiles AdminPermissionResource = "tls_fingerprint_profiles"
	AdminResourceScheduledTests         AdminPermissionResource = "scheduled_tests"
	AdminResourcePages                  AdminPermissionResource = "pages"
	AdminResourceAdminPermissions       AdminPermissionResource = "admin_permissions"
)

type AdminPermission struct {
	Resource AdminPermissionResource `json:"resource"`
	Actions  []AdminPermissionAction `json:"actions"`
}

type AdminPermissionDefinition struct {
	Resource       AdminPermissionResource `json:"resource"`
	Label          string                  `json:"label"`
	Actions        []AdminPermissionAction `json:"actions"`
	SuperAdminOnly bool                    `json:"super_admin_only"`
}

var allAdminPermissionActions = []AdminPermissionAction{
	AdminActionView,
	AdminActionCreate,
	AdminActionUpdate,
	AdminActionDelete,
	AdminActionExport,
	AdminActionExecute,
}

var adminPermissionDefinitions = []AdminPermissionDefinition{
	{AdminResourceDashboard, "Dashboard", []AdminPermissionAction{AdminActionView, AdminActionExecute}, false},
	{AdminResourceOps, "Operations", allAdminPermissionActions, false},
	{AdminResourceAlertRules, "Alert Rules", allAdminPermissionActions, false},
	{AdminResourceAlertEvents, "Alert Events", []AdminPermissionAction{AdminActionView, AdminActionUpdate}, false},
	{AdminResourceAlertSilences, "Alert Silences", []AdminPermissionAction{AdminActionView, AdminActionCreate, AdminActionDelete}, false},
	{AdminResourceAuthCacheInvalidation, "Auth Cache Invalidation", []AdminPermissionAction{AdminActionView, AdminActionExecute}, false},
	{AdminResourceUsers, "Users", allAdminPermissionActions, false},
	{AdminResourceUserAttributes, "User Attributes", allAdminPermissionActions, false},
	{AdminResourceAPIKeys, "API Keys", []AdminPermissionAction{AdminActionView, AdminActionUpdate, AdminActionExecute}, false},
	{AdminResourceGroups, "Groups", allAdminPermissionActions, false},
	{AdminResourceAccounts, "Accounts", allAdminPermissionActions, false},
	{AdminResourceAnnouncements, "Announcements", allAdminPermissionActions, false},
	{AdminResourceProxies, "Proxies", allAdminPermissionActions, false},
	{AdminResourceRedeemCodes, "Redeem Codes", allAdminPermissionActions, false},
	{AdminResourcePromoCodes, "Promo Codes", allAdminPermissionActions, false},
	{AdminResourceChannels, "Channels", allAdminPermissionActions, false},
	{AdminResourceChannelMonitor, "Channel Monitor", allAdminPermissionActions, false},
	{AdminResourceSubscriptions, "Subscriptions", allAdminPermissionActions, false},
	{AdminResourceUsage, "Usage", []AdminPermissionAction{AdminActionView, AdminActionCreate, AdminActionExecute}, false},
	{AdminResourceRiskControl, "Risk Control", allAdminPermissionActions, false},
	{AdminResourcePromptAudit, "Prompt Audit", allAdminPermissionActions, false},
	{AdminResourceAuditLogs, "Audit Logs", []AdminPermissionAction{AdminActionView, AdminActionExecute}, false},
	{AdminResourceAffiliates, "Affiliates", allAdminPermissionActions, false},
	{AdminResourcePaymentDashboard, "Payment Dashboard", []AdminPermissionAction{AdminActionView}, false},
	{AdminResourcePaymentOrders, "Payment Orders", []AdminPermissionAction{AdminActionView, AdminActionExecute}, false},
	{AdminResourcePaymentPlans, "Payment Plans", allAdminPermissionActions, false},
	{AdminResourcePaymentProviders, "Payment Providers", allAdminPermissionActions, false},
	{AdminResourcePaymentSettings, "Payment Settings", []AdminPermissionAction{AdminActionView, AdminActionUpdate}, false},
	{AdminResourceDataManagement, "Data Management", allAdminPermissionActions, false},
	{AdminResourceBackups, "Backups", allAdminPermissionActions, false},
	{AdminResourceSettings, "Settings", allAdminPermissionActions, true},
	{AdminResourceSystem, "System", allAdminPermissionActions, true},
	{AdminResourceErrorPassthroughRules, "Error Passthrough Rules", allAdminPermissionActions, false},
	{AdminResourceTLSFingerprintProfiles, "TLS Fingerprint Profiles", allAdminPermissionActions, false},
	{AdminResourceScheduledTests, "Scheduled Tests", allAdminPermissionActions, false},
	{AdminResourcePages, "Pages", []AdminPermissionAction{AdminActionView, AdminActionUpdate}, false},
	{AdminResourceAdminPermissions, "Admin Permissions", allAdminPermissionActions, true},
}

// AdminPermissionRegistry returns a deep copy so callers cannot mutate the
// authorization source of truth.
func AdminPermissionRegistry() []AdminPermissionDefinition {
	definitions := make([]AdminPermissionDefinition, len(adminPermissionDefinitions))
	for i, definition := range adminPermissionDefinitions {
		definitions[i] = definition
		definitions[i].Actions = append([]AdminPermissionAction(nil), definition.Actions...)
	}
	return definitions
}

func AdminPermissionDefinitionFor(resource AdminPermissionResource) (AdminPermissionDefinition, bool) {
	for _, definition := range adminPermissionDefinitions {
		if definition.Resource == resource {
			definition.Actions = append([]AdminPermissionAction(nil), definition.Actions...)
			return definition, true
		}
	}
	return AdminPermissionDefinition{}, false
}

// NormalizeAdminPermissions validates a limited-admin permission set and
// returns a stable deep copy suitable for persistence and API responses.
func NormalizeAdminPermissions(permissions []AdminPermission) ([]AdminPermission, error) {
	normalized := make([]AdminPermission, 0, len(permissions))
	seenResources := make(map[AdminPermissionResource]struct{}, len(permissions))

	for _, permission := range permissions {
		definition, ok := AdminPermissionDefinitionFor(permission.Resource)
		if !ok {
			return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("invalid admin permission resource: %s", permission.Resource))
		}
		if definition.SuperAdminOnly {
			return nil, infraerrors.Forbidden("SUPER_ADMIN_REQUIRED", fmt.Sprintf("resource requires super administrator: %s", permission.Resource))
		}
		if _, duplicate := seenResources[permission.Resource]; duplicate {
			return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("duplicate admin permission resource: %s", permission.Resource))
		}
		seenResources[permission.Resource] = struct{}{}

		if len(permission.Actions) == 0 {
			return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("admin permission must contain actions: %s", permission.Resource))
		}
		allowedActions := make(map[AdminPermissionAction]struct{}, len(definition.Actions))
		for _, action := range definition.Actions {
			allowedActions[action] = struct{}{}
		}

		actions := append([]AdminPermissionAction(nil), permission.Actions...)
		hasView := false
		seenActions := make(map[AdminPermissionAction]struct{}, len(actions))
		for _, action := range actions {
			if _, ok := allowedActions[action]; !ok {
				return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("invalid admin permission action: %s:%s", permission.Resource, action))
			}
			if _, duplicate := seenActions[action]; duplicate {
				return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("duplicate admin permission action: %s:%s", permission.Resource, action))
			}
			seenActions[action] = struct{}{}
			hasView = hasView || action == AdminActionView
		}
		if !hasView {
			return nil, infraerrors.BadRequest("INVALID_ADMIN_PERMISSION", fmt.Sprintf("admin permission requires view action: %s", permission.Resource))
		}
		sort.Slice(actions, func(i, j int) bool {
			return adminPermissionActionOrder(actions[i]) < adminPermissionActionOrder(actions[j])
		})
		normalized = append(normalized, AdminPermission{Resource: permission.Resource, Actions: actions})
	}

	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Resource < normalized[j].Resource })
	return normalized, nil
}

func adminPermissionActionOrder(action AdminPermissionAction) int {
	switch action {
	case AdminActionView:
		return 0
	case AdminActionCreate:
		return 1
	case AdminActionUpdate:
		return 2
	case AdminActionDelete:
		return 3
	case AdminActionExport:
		return 4
	case AdminActionExecute:
		return 5
	default:
		return len(allAdminPermissionActions)
	}
}

func ValidateAdminPermissions(permissions []AdminPermission) error {
	_, err := NormalizeAdminPermissions(permissions)
	return err
}

// IsAdminPermissionRouteValid is used by route registration and middleware to
// reject manifest entries that do not exist in the resource directory.
func IsAdminPermissionRouteValid(resource AdminPermissionResource, action AdminPermissionAction) bool {
	definition, ok := AdminPermissionDefinitionFor(resource)
	if !ok {
		return false
	}
	for _, allowed := range definition.Actions {
		if allowed == action {
			return true
		}
	}
	return false
}

type AdminPermissionRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]AdminPermission, error)
	ReplaceForUser(ctx context.Context, userID int64, permissions []AdminPermission) error
	DeleteForUser(ctx context.Context, userID int64) error
	HasPermission(ctx context.Context, userID int64, resource AdminPermissionResource, action AdminPermissionAction) (bool, error)
}
