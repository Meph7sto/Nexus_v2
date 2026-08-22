package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type loginAdminPermissionRepoStub struct {
	permissions []service.AdminPermission
}

func (s *loginAdminPermissionRepoStub) ListByUserID(context.Context, int64) ([]service.AdminPermission, error) {
	return s.permissions, nil
}

func (*loginAdminPermissionRepoStub) ReplaceForUser(context.Context, int64, []service.AdminPermission) error {
	return nil
}

func (*loginAdminPermissionRepoStub) DeleteForUser(context.Context, int64) error {
	return nil
}

func (*loginAdminPermissionRepoStub) HasPermission(context.Context, int64, service.AdminPermissionResource, service.AdminPermissionAction) (bool, error) {
	return false, nil
}

func TestHydrateLoginAdminPermissionsLoadsLimitedAdminGrants(t *testing.T) {
	repo := &loginAdminPermissionRepoStub{permissions: []service.AdminPermission{{
		Resource: service.AdminResourceAccounts,
		Actions:  []service.AdminPermissionAction{service.AdminActionView},
	}}}
	userService := service.NewUserService(nil, nil, nil, nil, repo)
	user := &service.User{ID: 42, Role: service.RoleAdmin}

	require.NoError(t, hydrateLoginAdminPermissions(context.Background(), userService, user))
	require.Equal(t, repo.permissions, user.AdminPermissions)
}

func TestHydrateLoginAdminPermissionsSkipsSuperAdmin(t *testing.T) {
	repo := &loginAdminPermissionRepoStub{permissions: []service.AdminPermission{{
		Resource: service.AdminResourceAccounts,
		Actions:  []service.AdminPermissionAction{service.AdminActionView},
	}}}
	userService := service.NewUserService(nil, nil, nil, nil, repo)
	user := &service.User{ID: 1, Role: service.RoleSuperAdmin}

	require.NoError(t, hydrateLoginAdminPermissions(context.Background(), userService, user))
	require.Empty(t, user.AdminPermissions)
}
