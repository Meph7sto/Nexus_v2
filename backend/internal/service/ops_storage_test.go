package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsServiceGetStorageUsageCollectsConfiguredPathsWithPrecedence(t *testing.T) {
	root := t.TempDir()
	appData := filepath.Join(root, "app-data")
	configPath := filepath.Join(root, "postgres-data")
	configDuplicate := filepath.Join(root, "config-duplicate")
	envDuplicate := filepath.Join(root, "env-duplicate")
	dockerData := filepath.Join(root, "docker")

	writeOpsStorageTestFile(t, filepath.Join(appData, "app.bin"), 10)
	writeOpsStorageTestFile(t, filepath.Join(configPath, "base", "db.bin"), 20)
	writeOpsStorageTestFile(t, filepath.Join(configDuplicate, "ignored.bin"), 30)
	writeOpsStorageTestFile(t, filepath.Join(envDuplicate, "active.bin"), 40)
	writeOpsStorageTestFile(t, filepath.Join(dockerData, "overlay2", "layer.bin"), 50)

	t.Setenv("DATA_DIR", appData)
	t.Setenv("OPS_STORAGE_PATHS", "duplicate="+envDuplicate+";docker="+dockerData)
	svc := newOpsStorageTestService(&opsRepoMock{
		GetCurrentDatabaseSizeBytesFn: func(context.Context) (int64, error) { return 60, nil },
	}, &config.Config{Ops: config.OpsConfig{
		Enabled: true,
		Storage: config.OpsStorageConfig{Paths: []config.OpsStoragePathConfig{
			{Key: "postgres_data", Label: "PostgreSQL data", Kind: "directory", Path: configPath},
			{Key: "duplicate", Path: configDuplicate},
		}},
	}})

	usage, err := svc.GetStorageUsage(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 180, usage.TotalUsedBytes)

	items := opsStorageItemsByKey(usage.Items)
	requireStorageItem(t, items["postgres_db"], OpsStorageStatusOK, 60)
	requireStorageItem(t, items["app_data"], OpsStorageStatusOK, 10)
	requireStorageItem(t, items["postgres_data"], OpsStorageStatusOK, 20)
	requireStorageItem(t, items["duplicate"], OpsStorageStatusOK, 40)
	requireStorageItem(t, items["docker"], OpsStorageStatusOK, 50)
	require.Equal(t, "env", items["duplicate"].Source)
	require.Equal(t, envDuplicate, items["duplicate"].Path)
	require.Equal(t, "PostgreSQL data", items["postgres_data"].Label)
}

func TestOpsServiceGetStorageUsageIsBestEffortAndDoesNotLeakRepositoryError(t *testing.T) {
	root := t.TempDir()
	appData := filepath.Join(root, "app-data")
	missing := filepath.Join(root, "missing")
	writeOpsStorageTestFile(t, filepath.Join(appData, "app.bin"), 12)

	t.Setenv("DATA_DIR", appData)
	t.Setenv("OPS_STORAGE_PATHS", "")
	svc := newOpsStorageTestService(&opsRepoMock{
		GetCurrentDatabaseSizeBytesFn: func(context.Context) (int64, error) {
			return 0, errors.New("postgres://user:secret@example.invalid/nexus " + strings.Repeat("x", 512))
		},
	}, &config.Config{Ops: config.OpsConfig{
		Enabled: true,
		Storage: config.OpsStorageConfig{Paths: []config.OpsStoragePathConfig{
			{Key: "missing", Path: missing},
			{Key: "empty", Path: ""},
		}},
	}})

	usage, err := svc.GetStorageUsage(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 12, usage.TotalUsedBytes)

	items := opsStorageItemsByKey(usage.Items)
	requireStorageItem(t, items["app_data"], OpsStorageStatusOK, 12)
	require.Equal(t, OpsStorageStatusUnavailable, items["postgres_db"].Status)
	require.NotEmpty(t, items["postgres_db"].Error)
	require.LessOrEqual(t, len(items["postgres_db"].Error), opsStorageMaxErrorLength)
	require.NotContains(t, items["postgres_db"].Error, "secret")
	require.Equal(t, OpsStorageStatusUnavailable, items["missing"].Status)
	require.NotEmpty(t, items["missing"].Error)
	require.Equal(t, OpsStorageStatusUnconfigured, items["empty"].Status)
	require.Nil(t, items["empty"].UsedBytes)
}

func TestOpsServiceGetStorageUsageNormalizesNegativeDatabaseSize(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DATA_DIR", root)
	t.Setenv("OPS_STORAGE_PATHS", "")
	svc := newOpsStorageTestService(&opsRepoMock{
		GetCurrentDatabaseSizeBytesFn: func(context.Context) (int64, error) { return -1, nil },
	}, &config.Config{Ops: config.OpsConfig{Enabled: true}})

	usage, err := svc.GetStorageUsage(context.Background())
	require.NoError(t, err)
	items := opsStorageItemsByKey(usage.Items)
	requireStorageItem(t, items["postgres_db"], OpsStorageStatusOK, 0)
}

func TestParseOpsStorageEnvPaths(t *testing.T) {
	specs := parseOpsStorageEnvPaths("postgres_data=/var/lib/postgresql/data;docker=/var/lib/docker\n/cache")
	require.Len(t, specs, 3)
	require.Equal(t, opsStoragePathSpec{Key: "postgres_data", Label: "postgres_data", Kind: "directory", Source: "env", Path: "/var/lib/postgresql/data"}, specs[0])
	require.Equal(t, "docker", specs[1].Key)
	require.Equal(t, "/var/lib/docker", specs[1].Path)
	require.Equal(t, "cache", specs[2].Key)
	require.Equal(t, "/cache", specs[2].Path)
}

func TestCollectOpsStoragePathKeepsPartialBytesAndClassifiesPermissionFailure(t *testing.T) {
	root := t.TempDir()
	partialPath := filepath.Join(root, "partial.bin")
	writeOpsStorageTestFile(t, partialPath, 9)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	originalWalkDir := opsStorageWalkDir
	t.Cleanup(func() { opsStorageWalkDir = originalWalkDir })
	opsStorageWalkDir = func(path string, walkFn fs.WalkDirFunc) error {
		if err := walkFn(partialPath, entries[0], nil); err != nil {
			return err
		}
		return walkFn(filepath.Join(path, "restricted"), nil, fs.ErrPermission)
	}

	partial := collectOpsStoragePath(context.Background(), opsStoragePathSpec{Key: "partial", Path: root})
	requireStorageItem(t, partial, OpsStorageStatusOK, 9)
	require.NotEmpty(t, partial.Error)
	require.LessOrEqual(t, len(partial.Error), opsStorageMaxErrorLength)

	opsStorageWalkDir = func(string, fs.WalkDirFunc) error { return fs.ErrPermission }
	denied := collectOpsStoragePath(context.Background(), opsStoragePathSpec{Key: "denied", Path: root})
	require.Equal(t, OpsStorageStatusUnavailable, denied.Status)
	require.Nil(t, denied.UsedBytes)
	require.NotEmpty(t, denied.Error)
}

func TestCollectOpsStoragePathHonorsCanceledAndExpiredContexts(t *testing.T) {
	root := t.TempDir()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled := collectOpsStoragePath(canceledCtx, opsStoragePathSpec{Key: "canceled", Path: root})
	require.Equal(t, OpsStorageStatusUnavailable, canceled.Status)
	require.Equal(t, opsStorageTimedOutError, canceled.Error)

	expiredCtx, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	expired := collectOpsStoragePath(expiredCtx, opsStoragePathSpec{Key: "expired", Path: root})
	require.Equal(t, OpsStorageStatusUnavailable, expired.Status)
	require.Equal(t, opsStorageTimedOutError, expired.Error)
}

func TestCalculateOpsPathSizePreservesLargeFileSizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.bin")
	const want = int64(1<<31) + 17
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, want); err != nil {
		t.Skipf("sparse large test file is not supported: %v", err)
	}

	got, err := calculateOpsPathSize(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestOpsServiceGetStorageUsageRejectsDisabledMonitoring(t *testing.T) {
	svc := newOpsStorageTestService(&opsRepoMock{}, &config.Config{Ops: config.OpsConfig{Enabled: false}})
	_, err := svc.GetStorageUsage(context.Background())
	require.ErrorIs(t, err, ErrOpsDisabled)
}

func newOpsStorageTestService(repo OpsRepository, cfg *config.Config) *OpsService {
	return NewOpsService(repo, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
}

func writeOpsStorageTestFile(t *testing.T, path string, size int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, make([]byte, size), 0o644))
}

func opsStorageItemsByKey(items []*OpsStorageUsageItem) map[string]*OpsStorageUsageItem {
	byKey := make(map[string]*OpsStorageUsageItem, len(items))
	for _, item := range items {
		if item != nil {
			byKey[item.Key] = item
		}
	}
	return byKey
}

func requireStorageItem(t *testing.T, item *OpsStorageUsageItem, status string, usedBytes int64) {
	t.Helper()
	require.NotNil(t, item)
	require.Equal(t, status, item.Status)
	require.NotNil(t, item.UsedBytes)
	require.Equal(t, usedBytes, *item.UsedBytes)
}
