//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type adminPermissionRepoStub struct {
	byUser map[int64][]AdminPermission
}

func (s *adminPermissionRepoStub) ListByUserID(_ context.Context, userID int64) ([]AdminPermission, error) {
	return append([]AdminPermission(nil), s.byUser[userID]...), nil
}

func (s *adminPermissionRepoStub) ReplaceForUser(_ context.Context, userID int64, permissions []AdminPermission) error {
	normalized, err := NormalizeAdminPermissions(permissions)
	if err != nil {
		return err
	}
	s.byUser[userID] = normalized
	return nil
}

func (s *adminPermissionRepoStub) DeleteForUser(_ context.Context, userID int64) error {
	delete(s.byUser, userID)
	return nil
}

func (s *adminPermissionRepoStub) HasPermission(_ context.Context, userID int64, resource AdminPermissionResource, action AdminPermissionAction) (bool, error) {
	for _, permission := range s.byUser[userID] {
		if permission.Resource != resource {
			continue
		}
		for _, granted := range permission.Actions {
			if granted == action {
				return true, nil
			}
		}
	}
	return false, nil
}

func TestAdminServiceUpdateUserReplacesLimitedAdminPermissions(t *testing.T) {
	permissions := []AdminPermission{{
		Resource: AdminResourceUsers,
		Actions:  []AdminPermissionAction{AdminActionView, AdminActionUpdate},
	}}
	permissionRepo := &adminPermissionRepoStub{byUser: map[int64][]AdminPermission{}}
	base := &userRepoStub{user: &User{ID: 42, Email: "admin@example.com", Role: RoleAdmin, Status: StatusActive}}
	svc := &adminServiceImpl{
		userRepo:             &rpmUserRepoStub{userRepoStub: base},
		redeemCodeRepo:       &redeemRepoStub{},
		adminPermissionRepo:  permissionRepo,
		authCacheInvalidator: &authCacheInvalidatorStub{},
	}

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AdminPermissions: &permissions})
	require.NoError(t, err)
	require.Equal(t, permissions, updated.AdminPermissions)
	require.Equal(t, permissions, permissionRepo.byUser[42])
}

func TestAdminServiceUpdateUserClearsPermissionsWhenLeavingLimitedAdminRole(t *testing.T) {
	permissionRepo := &adminPermissionRepoStub{byUser: map[int64][]AdminPermission{
		42: {{Resource: AdminResourceUsers, Actions: []AdminPermissionAction{AdminActionView}}},
	}}
	base := &userRepoStub{user: &User{ID: 42, Email: "admin@example.com", Role: RoleAdmin, Status: StatusActive}}
	svc := &adminServiceImpl{
		userRepo:            &rpmUserRepoStub{userRepoStub: base},
		redeemCodeRepo:      &redeemRepoStub{},
		adminPermissionRepo: permissionRepo,
	}

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleSuperAdmin})
	require.NoError(t, err)
	_, exists := permissionRepo.byUser[42]
	require.False(t, exists)
}

func TestAdminServiceUpdateUserReturnsExistingLimitedAdminPermissions(t *testing.T) {
	permissions := []AdminPermission{{
		Resource: AdminResourceUsers,
		Actions:  []AdminPermissionAction{AdminActionView, AdminActionUpdate},
	}}
	permissionRepo := &adminPermissionRepoStub{byUser: map[int64][]AdminPermission{42: permissions}}
	base := &userRepoStub{user: &User{ID: 42, Email: "admin@example.com", Role: RoleAdmin, Status: StatusActive}}
	svc := &adminServiceImpl{
		userRepo:            &rpmUserRepoStub{userRepoStub: base},
		redeemCodeRepo:      &redeemRepoStub{},
		adminPermissionRepo: permissionRepo,
	}

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Email: "renamed@example.com"})
	require.NoError(t, err)
	require.Equal(t, permissions, updated.AdminPermissions)
}
