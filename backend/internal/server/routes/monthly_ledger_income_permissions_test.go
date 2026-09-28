package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIncomePermissionManifest(t *testing.T) {
	for method, paths := range map[string][]string{
		"GET":    {"overview", "income-entries", "income-entries/:id/payments", "income-schedules"},
		"POST":   {"income-entries", "income-entries/:id/payments", "income-schedules", "income-schedules/preview"},
		"PUT":    {"income-entries/:id", "income-payments/:id", "income-schedules/:id", "income-schedules/:id/state"},
		"DELETE": {"income-entries/:id", "income-payments/:id"},
	} {
		for _, path := range paths {
			p, ok := AdminRoutePermissionFor(method, "/api/v1/admin/monthly-ledger/"+path)
			require.True(t, ok, path)
			require.Equal(t, service.AdminResourceMonthlyLedger, p.Resource)
			require.Equal(t, map[string]service.AdminPermissionAction{"GET": service.AdminActionView, "POST": service.AdminActionCreate, "PUT": service.AdminActionUpdate, "DELETE": service.AdminActionDelete}[method], p.Action)
		}
	}
}
