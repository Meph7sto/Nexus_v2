package migrations

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageInteractionsMigrationDefinesIsolatedStorageAndSafeDefaults(t *testing.T) {
	content, err := fs.ReadFile(FS, "187_add_usage_interactions.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_interactions")
	require.Contains(t, sql, "usage_log_id BIGINT NOT NULL UNIQUE REFERENCES usage_logs(id) ON DELETE CASCADE")
	require.Contains(t, sql, "raw_request_json JSONB")
	require.Contains(t, sql, "raw_response_json JSONB")
	require.Contains(t, sql, "INSERT INTO settings (key, value)\nVALUES")
	require.NotContains(t, sql, "INSERT INTO settings (key, value, created_at, updated_at)")
	require.Contains(t, sql, "('usage_interaction_recording_enabled', 'false'")
	require.Contains(t, sql, "('usage_interaction_store_raw_enabled', 'false'")
	require.Contains(t, sql, "('usage_interaction_retention_days', '7'")
	require.True(t, strings.Contains(sql, "idx_usage_interactions_created_at"))
}
