package migrations

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsErrorOwnerReconciliationMigrationTargetsOnlyLegacyBrandOwners(t *testing.T) {
	content, err := fs.ReadFile(FS, "185_reconcile_ops_error_owner_brand_values.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "SET error_owner = 'platform'")
	require.Contains(t, sql, "LOWER(COALESCE(TRIM(error_owner), ''))")
	require.Contains(t, sql, "IN ('nexus', 'sub2api')")
}
