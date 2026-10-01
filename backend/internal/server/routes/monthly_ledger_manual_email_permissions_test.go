package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMonthlyLedgerManualEmailPermissions(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		action       service.AdminPermissionAction
	}{
		{"GET", "email-quota", service.AdminActionView},
		{"GET", ":month/users/:user_id/emails", service.AdminActionView},
		{"PUT", "email-quota", service.AdminActionUpdate},
		{"PUT", "email-notifications", service.AdminActionUpdate},
		{"POST", "emails", service.AdminActionCreate},
	} {
		permission, ok := AdminRoutePermissionFor(tc.method, "/api/v1/admin/monthly-ledger/"+tc.path)
		require.True(t, ok)
		require.Equal(t, service.AdminResourceMonthlyLedger, permission.Resource)
		require.Equal(t, tc.action, permission.Action)
	}
}
