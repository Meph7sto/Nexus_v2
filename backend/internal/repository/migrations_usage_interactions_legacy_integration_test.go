//go:build integration

package repository

import (
	"context"
	"io/fs"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration187SupportsLegacySettingsWithoutCreatedAt(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	_, err := tx.ExecContext(ctx, `
CREATE SCHEMA phase6_legacy_settings;
SET LOCAL search_path TO phase6_legacy_settings, public;
CREATE TABLE settings (
  id BIGSERIAL PRIMARY KEY,
  key VARCHAR(100) NOT NULL UNIQUE,
  value TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE usage_logs (
  id BIGSERIAL PRIMARY KEY
);`)
	require.NoError(t, err)

	content, err := fs.ReadFile(migrations.FS, "187_add_usage_interactions.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(content))
	require.NoError(t, err)

	rows, err := tx.QueryContext(ctx, "SELECT key, value FROM settings ORDER BY key")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	actual := make(map[string]string)
	for rows.Next() {
		var key, value string
		require.NoError(t, rows.Scan(&key, &value))
		actual[key] = value
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]string{
		"usage_interaction_recording_enabled": "false",
		"usage_interaction_retention_days":    "7",
		"usage_interaction_store_raw_enabled": "false",
	}, actual)
}
