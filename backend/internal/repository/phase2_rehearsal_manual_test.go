//go:build phase2rehearsal

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

const phase2RehearsalConfirmation = "ISOLATED_COPY"

var phase2RehearsalHistoricalMigrations = []string{
	"001_init.sql",
	"002_account_type_migration.sql",
	"003_subscription.sql",
	"038_ops_errors_resolution_retry_results_and_standardize_classification.sql",
	"052_migrate_upstream_to_apikey.sql",
}

var phase2RehearsalTableCounts = []string{
	"api_keys",
	"accounts",
	"groups",
	"usage_logs",
	"payment_orders",
	"user_subscriptions",
	"settings",
	"admin_permissions",
}

type phase2RehearsalSnapshot struct {
	AppliedMigrations  map[string]string                    `json:"applied_migrations"`
	RoleCounts         map[string]int64                     `json:"role_counts"`
	OwnerCounts        map[string]int64                     `json:"owner_counts"`
	BalanceTotal       string                               `json:"balance_total"`
	FrozenBalanceTotal string                               `json:"frozen_balance_total"`
	TableCounts        map[string]phase2RehearsalTableCount `json:"table_counts"`
	AdminPermissions   phase2AdminPermissionsInvariants     `json:"admin_permissions"`
}

type phase2RehearsalTableCount struct {
	Exists bool  `json:"exists"`
	Count  int64 `json:"count"`
}

type phase2AdminPermissionsInvariants struct {
	Exists             bool `json:"exists"`
	UserResourceUnique bool `json:"user_resource_unique"`
	ActionsArrayCheck  bool `json:"actions_array_check"`
	UserIDIndex        bool `json:"user_id_index"`
	ResourceIndex      bool `json:"resource_index"`
	UserDeleteCascade  bool `json:"user_delete_cascade"`
}

type phase2RehearsalReport struct {
	Database       string                  `json:"database"`
	StartedAt      time.Time               `json:"started_at"`
	CompletedAt    time.Time               `json:"completed_at"`
	MigrationTime  string                  `json:"migration_time"`
	Before         phase2RehearsalSnapshot `json:"before"`
	After          phase2RehearsalSnapshot `json:"after"`
	SeededUserID   int64                   `json:"seeded_user_id"`
	SeededUserRole string                  `json:"seeded_user_role"`
	LoginClaimRole string                  `json:"login_claim_role"`
}

// TestPhase2RehearsalOnIsolatedCopy is opt-in because it runs real migrations.
// It refuses a database unless the caller explicitly names it as a rehearsal copy.
func TestPhase2RehearsalOnIsolatedCopy(t *testing.T) {
	dsn := os.Getenv("PHASE2_REHEARSAL_DSN")
	if dsn == "" {
		t.Skip("PHASE2_REHEARSAL_DSN is not set")
	}
	require.Equal(t, phase2RehearsalConfirmation, os.Getenv("PHASE2_REHEARSAL_CONFIRM"), "refusing to modify a database without explicit isolated-copy confirmation")

	reportPath := os.Getenv("PHASE2_REHEARSAL_REPORT")
	require.NotEmpty(t, reportPath, "PHASE2_REHEARSAL_REPORT is required to preserve rehearsal evidence")

	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))

	var database string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.NotEmpty(t, os.Getenv("PHASE2_REHEARSAL_DATABASE"), "PHASE2_REHEARSAL_DATABASE is required")
	require.Equal(t, os.Getenv("PHASE2_REHEARSAL_DATABASE"), database, "database name must be explicitly supplied")
	require.Contains(t, strings.ToLower(database), "rehearsal", "refusing a database not named as an isolated rehearsal copy")

	assertPhase2RehearsalPreflight(t, ctx, db)
	before, err := capturePhase2RehearsalSnapshot(ctx, db)
	require.NoError(t, err)

	seededUserID, email, password := seedPhase2RehearsalAdmin(t, ctx, db)
	t.Cleanup(func() {
		_, cleanupErr := db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", seededUserID)
		require.NoError(t, cleanupErr)
	})

	startedAt := time.Now().UTC()
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db), "second run must be idempotent")
	completedAt := time.Now().UTC()

	var seededUserRole string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT role FROM users WHERE id = $1", seededUserID).Scan(&seededUserRole))
	require.Equal(t, service.RoleSuperAdmin, seededUserRole)

	claimsRole := assertPhase2RehearsalLogin(t, ctx, db, email, password)
	after, err := capturePhase2RehearsalSnapshot(ctx, db)
	require.NoError(t, err)

	assertPhase2RehearsalResult(t, before, after)
	require.NoError(t, writePhase2RehearsalReport(reportPath, phase2RehearsalReport{
		Database:       database,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		MigrationTime:  completedAt.Sub(startedAt).String(),
		Before:         before,
		After:          after,
		SeededUserID:   seededUserID,
		SeededUserRole: seededUserRole,
		LoginClaimRole: claimsRole,
	}))
}

func assertPhase2RehearsalPreflight(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	for _, name := range phase2RehearsalHistoricalMigrations {
		rule, ok := migrationChecksumCompatibilityRules[name]
		require.Truef(t, ok, "missing exact compatibility rule for %s", name)
		require.Truef(t, rule.requireExactFileChecksum, "rule for %s must require the current exact V2 file checksum", name)

		var checksum string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename = $1", name).Scan(&checksum))
		_, accepted := rule.acceptedDBChecksum[checksum]
		require.Truef(t, accepted, "checksum for %s is not an accepted exact historical value", name)
	}

	for _, name := range []string{
		"185_reconcile_ops_error_owner_brand_values.sql",
		"186_admin_permissions.sql",
	} {
		var applied bool
		require.NoError(t, db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)", name).Scan(&applied))
		require.Falsef(t, applied, "%s was already applied; restore a pre-Phase-2 copy before rehearsing", name)
	}
}

func capturePhase2RehearsalSnapshot(ctx context.Context, db *sql.DB) (phase2RehearsalSnapshot, error) {
	snapshot := phase2RehearsalSnapshot{
		AppliedMigrations: make(map[string]string, len(phase2RehearsalHistoricalMigrations)+2),
		RoleCounts:        make(map[string]int64),
		OwnerCounts:       make(map[string]int64),
		TableCounts:       make(map[string]phase2RehearsalTableCount, len(phase2RehearsalTableCounts)),
	}

	for _, name := range append(append([]string{}, phase2RehearsalHistoricalMigrations...), "185_reconcile_ops_error_owner_brand_values.sql", "186_admin_permissions.sql") {
		var checksum string
		err := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename = $1", name).Scan(&checksum)
		if err == nil {
			snapshot.AppliedMigrations[name] = checksum
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return phase2RehearsalSnapshot{}, fmt.Errorf("read migration %s: %w", name, err)
		}
	}

	roleRows, err := db.QueryContext(ctx, "SELECT role, COUNT(*) FROM users GROUP BY role ORDER BY role")
	if err != nil {
		return phase2RehearsalSnapshot{}, fmt.Errorf("count roles: %w", err)
	}
	defer func() { _ = roleRows.Close() }()
	for roleRows.Next() {
		var role string
		var count int64
		if err := roleRows.Scan(&role, &count); err != nil {
			return phase2RehearsalSnapshot{}, fmt.Errorf("scan role count: %w", err)
		}
		snapshot.RoleCounts[role] = count
	}
	if err := roleRows.Err(); err != nil {
		return phase2RehearsalSnapshot{}, fmt.Errorf("iterate role counts: %w", err)
	}

	ownerRows, err := db.QueryContext(ctx, `
SELECT LOWER(COALESCE(TRIM(error_owner), '<null>')), COUNT(*)
FROM ops_error_logs
GROUP BY 1
ORDER BY 1`)
	if err != nil {
		return phase2RehearsalSnapshot{}, fmt.Errorf("count error owners: %w", err)
	}
	defer func() { _ = ownerRows.Close() }()
	for ownerRows.Next() {
		var owner string
		var count int64
		if err := ownerRows.Scan(&owner, &count); err != nil {
			return phase2RehearsalSnapshot{}, fmt.Errorf("scan error owner count: %w", err)
		}
		snapshot.OwnerCounts[owner] = count
	}
	if err := ownerRows.Err(); err != nil {
		return phase2RehearsalSnapshot{}, fmt.Errorf("iterate error owner counts: %w", err)
	}

	if err := db.QueryRowContext(ctx, "SELECT COALESCE(SUM(balance), 0)::text, COALESCE(SUM(frozen_balance), 0)::text FROM users").Scan(&snapshot.BalanceTotal, &snapshot.FrozenBalanceTotal); err != nil {
		return phase2RehearsalSnapshot{}, fmt.Errorf("sum balances: %w", err)
	}

	for _, table := range phase2RehearsalTableCounts {
		var regclass sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1)", "public."+table).Scan(&regclass); err != nil {
			return phase2RehearsalSnapshot{}, fmt.Errorf("check %s: %w", table, err)
		}
		if !regclass.Valid {
			snapshot.TableCounts[table] = phase2RehearsalTableCount{}
			continue
		}

		var count int64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return phase2RehearsalSnapshot{}, fmt.Errorf("count %s: %w", table, err)
		}
		snapshot.TableCounts[table] = phase2RehearsalTableCount{Exists: true, Count: count}
	}

	adminPermissions, err := capturePhase2AdminPermissionsInvariants(ctx, db)
	if err != nil {
		return phase2RehearsalSnapshot{}, err
	}
	snapshot.AdminPermissions = adminPermissions

	return snapshot, nil
}

func capturePhase2AdminPermissionsInvariants(ctx context.Context, db *sql.DB) (phase2AdminPermissionsInvariants, error) {
	var regclass sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.admin_permissions')").Scan(&regclass); err != nil {
		return phase2AdminPermissionsInvariants{}, fmt.Errorf("check admin_permissions: %w", err)
	}
	if !regclass.Valid {
		return phase2AdminPermissionsInvariants{}, nil
	}

	invariants := phase2AdminPermissionsInvariants{Exists: true}
	checks := []struct {
		name   string
		target *bool
		query  string
	}{
		{
			name:   "unique constraint",
			target: &invariants.UserResourceUnique,
			query:  "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'public.admin_permissions'::regclass AND conname = 'uq_admin_permissions_user_resource' AND contype = 'u')",
		},
		{
			name:   "actions check",
			target: &invariants.ActionsArrayCheck,
			query:  "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'public.admin_permissions'::regclass AND conname = 'chk_admin_permissions_actions_array' AND contype = 'c')",
		},
		{
			name:   "user index",
			target: &invariants.UserIDIndex,
			query:  "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND tablename = 'admin_permissions' AND indexname = 'idx_admin_permissions_user_id')",
		},
		{
			name:   "resource index",
			target: &invariants.ResourceIndex,
			query:  "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND tablename = 'admin_permissions' AND indexname = 'idx_admin_permissions_resource')",
		},
		{
			name:   "cascade foreign key",
			target: &invariants.UserDeleteCascade,
			query:  "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'public.admin_permissions'::regclass AND contype = 'f' AND confdeltype = 'c')",
		},
	}
	for _, check := range checks {
		if err := db.QueryRowContext(ctx, check.query).Scan(check.target); err != nil {
			return phase2AdminPermissionsInvariants{}, fmt.Errorf("check admin_permissions %s: %w", check.name, err)
		}
	}
	return invariants, nil
}

func seedPhase2RehearsalAdmin(t *testing.T, ctx context.Context, db *sql.DB) (int64, string, string) {
	t.Helper()

	password := "phase2-rehearsal-admin-password"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	email := fmt.Sprintf("phase2-rehearsal-%d@example.invalid", time.Now().UnixNano())

	var id int64
	require.NoError(t, db.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, role, status)
VALUES ($1, $2, $3, $4)
RETURNING id`, email, string(passwordHash), service.RoleAdmin, service.StatusActive).Scan(&id))
	return id, email, password
}

func assertPhase2RehearsalLogin(t *testing.T, ctx context.Context, db *sql.DB, email, password string) string {
	t.Helper()

	driver := entsql.OpenDB(dialect.Postgres, db)
	entClient := dbent.NewClient(dbent.Driver(driver))

	authService := service.NewAuthService(
		entClient,
		NewUserRepository(entClient, db),
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

	token, user, err := authService.Login(ctx, email, password)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, service.RoleSuperAdmin, user.Role)

	claims, err := authService.ValidateToken(token)
	require.NoError(t, err)
	require.Equal(t, service.RoleSuperAdmin, claims.Role)
	return claims.Role
}

func assertPhase2RehearsalResult(t *testing.T, before, after phase2RehearsalSnapshot) {
	t.Helper()

	require.Equal(t, int64(0), after.RoleCounts[service.RoleAdmin])
	require.Equal(t, before.RoleCounts[service.RoleSuperAdmin]+before.RoleCounts[service.RoleAdmin]+1, after.RoleCounts[service.RoleSuperAdmin])
	require.Equal(t, before.RoleCounts[service.RoleUser], after.RoleCounts[service.RoleUser])

	expectedOwners := make(map[string]int64, len(before.OwnerCounts)+1)
	for owner, count := range before.OwnerCounts {
		if owner == "nexus" || owner == "sub2api" {
			expectedOwners["platform"] += count
			continue
		}
		expectedOwners[owner] += count
	}
	require.Equal(t, expectedOwners, after.OwnerCounts)

	for _, name := range phase2RehearsalHistoricalMigrations {
		require.Equalf(t, before.AppliedMigrations[name], after.AppliedMigrations[name], "historical checksum changed for %s", name)
	}
	for _, name := range []string{
		"185_reconcile_ops_error_owner_brand_values.sql",
		"186_admin_permissions.sql",
	} {
		require.NotEmptyf(t, after.AppliedMigrations[name], "%s was not recorded", name)
	}
	for _, table := range phase2RehearsalTableCounts {
		require.Truef(t, after.TableCounts[table].Exists, "expected table %s after rehearsal", table)
	}
	require.True(t, after.AdminPermissions.Exists)
	require.True(t, after.AdminPermissions.UserResourceUnique)
	require.True(t, after.AdminPermissions.ActionsArrayCheck)
	require.True(t, after.AdminPermissions.UserIDIndex)
	require.True(t, after.AdminPermissions.ResourceIndex)
	require.True(t, after.AdminPermissions.UserDeleteCascade)
}

func writePhase2RehearsalReport(path string, report phase2RehearsalReport) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rehearsal report: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write rehearsal report: %w", err)
	}
	return nil
}
