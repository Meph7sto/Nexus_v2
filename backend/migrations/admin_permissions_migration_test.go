package migrations

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminPermissionsMigrationUpgradesLegacyNexusConstraintShape(t *testing.T) {
	content, err := fs.ReadFile(FS, "186_admin_permissions.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "RENAME CONSTRAINT")
	require.Contains(t, sql, "contype = 'u'")
	require.Contains(t, sql, "conkey = ARRAY")
	require.Contains(t, sql, "uq_admin_permissions_user_resource")
	require.Contains(t, sql, "chk_admin_permissions_actions_array")
}
