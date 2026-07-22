//go:build integration

package repository

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestMigration186UpgradesLegacyNexusAdminPermissions(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	_, err := tx.ExecContext(ctx, `
CREATE SCHEMA phase2_legacy_admin_permissions;
SET LOCAL search_path TO phase2_legacy_admin_permissions, public;
CREATE TABLE admin_permissions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  resource VARCHAR(64) NOT NULL,
  actions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT admin_permissions_user_resource_unique UNIQUE (user_id, resource)
);`)
	require.NoError(t, err)

	content, err := fs.ReadFile(migrations.FS, "186_admin_permissions.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(content))
	require.NoError(t, err)

	var uniqueConstraintExists bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_constraint
  WHERE conrelid = 'phase2_legacy_admin_permissions.admin_permissions'::regclass
    AND conname = 'uq_admin_permissions_user_resource'
    AND contype = 'u'
)`).Scan(&uniqueConstraintExists))
	require.True(t, uniqueConstraintExists)

	var actionsCheckExists bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_constraint
  WHERE conrelid = 'phase2_legacy_admin_permissions.admin_permissions'::regclass
    AND conname = 'chk_admin_permissions_actions_array'
    AND contype = 'c'
)`).Scan(&actionsCheckExists))
	require.True(t, actionsCheckExists)
}

func TestMigration185ReconcilesOnlyLegacyBrandOwners(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	_, err := tx.ExecContext(ctx, "CREATE SCHEMA phase2_owner_reconcile")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO phase2_owner_reconcile, public")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
CREATE TABLE ops_error_logs (
  id BIGSERIAL PRIMARY KEY,
  error_owner TEXT
);
INSERT INTO ops_error_logs (error_owner) VALUES
  ('Nexus'),
  (' sub2api '),
  ('client'),
  ('platform'),
  ('SUB2API'),
  (NULL);`)
	require.NoError(t, err)

	content, err := fs.ReadFile(migrations.FS, "185_reconcile_ops_error_owner_brand_values.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(content))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(content))
	require.NoError(t, err)

	rows, err := tx.QueryContext(ctx, "SELECT COALESCE(error_owner, '<null>') FROM ops_error_logs ORDER BY id")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	owners := make([]string, 0, 6)
	for rows.Next() {
		var owner string
		require.NoError(t, rows.Scan(&owner))
		owners = append(owners, owner)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"platform", "platform", "client", "platform", "platform", "<null>"}, owners)
}

func TestMigration186BackfillsLegacyAdminAndPreservesLogin(t *testing.T) {
	ctx := context.Background()
	password := "phase2-legacy-admin-password"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	var existingAdminIDs []int64
	rows, err := integrationDB.QueryContext(ctx, "SELECT id FROM users WHERE role = $1", service.RoleAdmin)
	require.NoError(t, err)
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		existingAdminIDs = append(existingAdminIDs, id)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	legacyAdmin := mustCreateUser(t, integrationEntClient, &service.User{
		Email:        fmt.Sprintf("phase2-legacy-admin-%d@example.test", time.Now().UnixNano()),
		PasswordHash: string(passwordHash),
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
	})
	t.Cleanup(func() {
		for _, id := range existingAdminIDs {
			_, cleanupErr := integrationDB.ExecContext(ctx, "UPDATE users SET role = $1 WHERE id = $2", service.RoleAdmin, id)
			require.NoError(t, cleanupErr)
		}
		require.NoError(t, integrationEntClient.User.DeleteOneID(legacyAdmin.ID).Exec(ctx))
	})

	content, err := fs.ReadFile(migrations.FS, "186_admin_permissions.sql")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, string(content))
	require.NoError(t, err)

	migrated, err := integrationEntClient.User.Get(ctx, legacyAdmin.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleSuperAdmin, migrated.Role)

	authService := service.NewAuthService(
		integrationEntClient,
		NewUserRepository(integrationEntClient, integrationDB),
		nil,
		nil,
		&config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("p", 64), ExpireHour: 1}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	token, authenticated, err := authService.Login(ctx, legacyAdmin.Email, password)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, service.RoleSuperAdmin, authenticated.Role)

	claims, err := authService.ValidateToken(token)
	require.NoError(t, err)
	require.Equal(t, service.RoleSuperAdmin, claims.Role)

}
