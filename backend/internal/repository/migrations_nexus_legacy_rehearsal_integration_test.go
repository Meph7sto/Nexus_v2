//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

var nexusFrozenChecksumFixture = []struct {
	filename string
	checksum string
}{
	{"001_init.sql", "5c0a4d96dcf6171a76c0634615e276821e7702787c25ad3bc6b6569595001815"},
	{"002_account_type_migration.sql", "3c14e65bdb91ab87e2b69437b11f2083c0a67df3d10b6f75ddaffff58d4fc7f0"},
	{"003_subscription.sql", "69df1b8ace3691e40e6a6aa099719a936629c1e066fa0a8796328690c87f4770"},
	{"038_ops_errors_resolution_retry_results_and_standardize_classification.sql", "debf408999da634be25cb20aec5011ca65ca8c27ef5aa45290eec32fb6e05df9"},
	{"052_migrate_upstream_to_apikey.sql", "1070dc0ed8174979aa71e02ec95a987a931ad8816024909c75ace7ed771b7f45"},
}

func TestMigrationsRunner_NexusLegacyFixtureRehearsal(t *testing.T) {
	ctx := context.Background()
	conn, schema := prepareNexusLegacyFixture(t, ctx)

	require.NoError(t, applyMigrationsOnConnection(ctx, conn, nexusLegacyRunnerFS(t)))

	assertNexusLegacyFixtureMigrated(t, ctx, conn, schema)

	// A second run proves that the pre-existing Nexus records remain accepted
	// and that 159/185/186 are recorded rather than replayed.
	require.NoError(t, applyMigrationsOnConnection(ctx, conn, nexusLegacyRunnerFS(t)))
	assertNexusLegacyFixtureMigrated(t, ctx, conn, schema)

	var migrationRows int
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationRows))
	require.Equal(t, 9, migrationRows)
}

func TestMigrationsRunner_NexusLegacyFixtureRejectsUnknownChecksum(t *testing.T) {
	ctx := context.Background()
	conn, _ := prepareNexusLegacyFixture(t, ctx)

	_, err := conn.ExecContext(ctx, `
UPDATE schema_migrations
SET checksum = 'ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff'
WHERE filename = '001_init.sql'`)
	require.NoError(t, err)

	err = applyMigrationsOnConnection(ctx, conn, nexusLegacyRunnerFS(t))
	require.ErrorContains(t, err, "migration 001_init.sql checksum mismatch")

	var role string
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT role FROM users WHERE email = 'legacy-admin@example.test'").Scan(&role))
	require.Equal(t, "admin", role)

	var legacyOwnerCount int
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM ops_error_logs
WHERE LOWER(TRIM(error_owner)) IN ('nexus', 'sub2api')`).Scan(&legacyOwnerCount))
	require.Equal(t, 2, legacyOwnerCount)

	for _, name := range []string{
		"159_batch_image_foundation.sql",
		"185_reconcile_ops_error_owner_brand_values.sql",
		"186_admin_permissions.sql",
	} {
		var applied bool
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)", name).Scan(&applied))
		require.Falsef(t, applied, "migration %s must not run after an unknown checksum", name)
	}
}

func prepareNexusLegacyFixture(t *testing.T, ctx context.Context) (*sql.Conn, string) {
	t.Helper()

	conn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)

	schema := fmt.Sprintf("phase2_nexus_legacy_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := conn.ExecContext(context.Background(), "SET search_path TO public"); err != nil {
			t.Errorf("reset rehearsal search path: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close rehearsal connection: %v", err)
		}
		if _, err := integrationDB.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schema)); err != nil {
			t.Errorf("drop rehearsal schema: %v", err)
		}
	})

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
CREATE SCHEMA %s;
SET search_path TO %s, public;

CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role VARCHAR(20) NOT NULL DEFAULT 'user',
  status VARCHAR(20) NOT NULL DEFAULT 'active'
);

CREATE TABLE ops_error_logs (
  id BIGSERIAL PRIMARY KEY,
  error_owner TEXT
);

CREATE TABLE admin_permissions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  resource VARCHAR(64) NOT NULL,
  actions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT admin_permissions_user_resource_unique UNIQUE (user_id, resource)
);

CREATE TABLE schema_migrations (
  filename TEXT PRIMARY KEY,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`, schema, schema))
	require.NoError(t, err)

	_, err = conn.ExecContext(ctx, `
INSERT INTO users (email, password_hash, role, status) VALUES
  ('legacy-admin@example.test', 'legacy-password-hash', 'admin', 'active'),
  ('legacy-user@example.test', 'legacy-password-hash', 'user', 'active');
INSERT INTO ops_error_logs (error_owner) VALUES
  ('Nexus'),
  (' sub2api '),
  ('client'),
  ('platform'),
  (NULL);
INSERT INTO schema_migrations (filename, checksum) VALUES
  ('159_admin_permissions.sql', 'legacy-nexus-admin-permissions');`)
	require.NoError(t, err)

	for _, entry := range nexusFrozenChecksumFixture {
		_, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", entry.filename, entry.checksum)
		require.NoError(t, err)
	}

	return conn, schema
}

func nexusLegacyRunnerFS(t *testing.T) fstest.MapFS {
	t.Helper()

	fsys := make(fstest.MapFS)
	for _, name := range []string{
		"001_init.sql",
		"002_account_type_migration.sql",
		"003_subscription.sql",
		"038_ops_errors_resolution_retry_results_and_standardize_classification.sql",
		"052_migrate_upstream_to_apikey.sql",
		"159_batch_image_foundation.sql",
		"185_reconcile_ops_error_owner_brand_values.sql",
		"186_admin_permissions.sql",
	} {
		content, err := fs.ReadFile(migrations.FS, name)
		require.NoError(t, err)
		fsys[name] = &fstest.MapFile{Data: content}
	}
	return fsys
}

func assertNexusLegacyFixtureMigrated(t *testing.T, ctx context.Context, conn *sql.Conn, schema string) {
	t.Helper()

	rows, err := conn.QueryContext(ctx, "SELECT role FROM users ORDER BY email")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	roles := make([]string, 0, 2)
	for rows.Next() {
		var role string
		require.NoError(t, rows.Scan(&role))
		roles = append(roles, role)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"super_admin", "user"}, roles)

	owners, err := conn.QueryContext(ctx, "SELECT COALESCE(error_owner, '<null>') FROM ops_error_logs ORDER BY id")
	require.NoError(t, err)
	defer func() { _ = owners.Close() }()

	actualOwners := make([]string, 0, 5)
	for owners.Next() {
		var owner string
		require.NoError(t, owners.Scan(&owner))
		actualOwners = append(actualOwners, owner)
	}
	require.NoError(t, owners.Err())
	require.Equal(t, []string{"platform", "platform", "client", "platform", "<null>"}, actualOwners)

	var batchImageJobs sql.NullString
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT to_regclass($1)", schema+".batch_image_jobs").Scan(&batchImageJobs))
	require.True(t, batchImageJobs.Valid)

	for _, name := range []string{
		"159_batch_image_foundation.sql",
		"185_reconcile_ops_error_owner_brand_values.sql",
		"186_admin_permissions.sql",
	} {
		var applied bool
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)", name).Scan(&applied))
		require.Truef(t, applied, "expected migration %s to be recorded", name)
	}
}
