package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/adminpermission"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adminPermissionRepository struct {
	client *dbent.Client
}

func NewAdminPermissionRepository(client *dbent.Client) service.AdminPermissionRepository {
	return &adminPermissionRepository{client: client}
}

func (r *adminPermissionRepository) ListByUserID(ctx context.Context, userID int64) ([]service.AdminPermission, error) {
	rows, err := clientFromContext(ctx, r.client).AdminPermission.Query().
		Where(adminpermission.UserIDEQ(userID)).
		Order(dbent.Asc(adminpermission.FieldResource)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	permissions := make([]service.AdminPermission, 0, len(rows))
	for _, row := range rows {
		actions := make([]service.AdminPermissionAction, 0, len(row.Actions))
		for _, action := range row.Actions {
			actions = append(actions, service.AdminPermissionAction(action))
		}
		permissions = append(permissions, service.AdminPermission{
			Resource: service.AdminPermissionResource(row.Resource),
			Actions:  actions,
		})
	}
	return permissions, nil
}

// ReplaceForUser is transaction-aware through clientFromContext. Callers that
// update a user role and its grants atomically pass an Ent transaction context.
func (r *adminPermissionRepository) ReplaceForUser(ctx context.Context, userID int64, permissions []service.AdminPermission) error {
	normalized, err := service.NormalizeAdminPermissions(permissions)
	if err != nil {
		return err
	}
	client := clientFromContext(ctx, r.client)
	if _, err := client.AdminPermission.Delete().Where(adminpermission.UserIDEQ(userID)).Exec(ctx); err != nil {
		return err
	}
	for _, permission := range normalized {
		actions := make([]string, 0, len(permission.Actions))
		for _, action := range permission.Actions {
			actions = append(actions, string(action))
		}
		if _, err := client.AdminPermission.Create().
			SetUserID(userID).
			SetResource(string(permission.Resource)).
			SetActions(actions).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *adminPermissionRepository) DeleteForUser(ctx context.Context, userID int64) error {
	_, err := clientFromContext(ctx, r.client).AdminPermission.Delete().
		Where(adminpermission.UserIDEQ(userID)).
		Exec(ctx)
	return err
}

func (r *adminPermissionRepository) HasPermission(ctx context.Context, userID int64, resource service.AdminPermissionResource, action service.AdminPermissionAction) (bool, error) {
	permissions, err := r.ListByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	normalized, err := service.NormalizeAdminPermissions(permissions)
	if err != nil {
		return false, fmt.Errorf("validate stored admin permissions: %w", err)
	}
	for _, permission := range normalized {
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
