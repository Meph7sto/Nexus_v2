package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	OpsStorageStatusOK           = "ok"
	OpsStorageStatusUnavailable  = "unavailable"
	OpsStorageStatusUnconfigured = "unconfigured"

	opsStorageCollectionTimeout = 5 * time.Second
	opsStorageEnvPaths          = "OPS_STORAGE_PATHS"
	opsStorageMaxErrorLength    = 256
	opsStorageTimedOutError     = "storage scan timed out"
)

var (
	opsStorageKeySanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	opsStorageWalkDir      = filepath.WalkDir
)

type OpsStorageUsageResponse struct {
	GeneratedAt    time.Time              `json:"generated_at"`
	TotalUsedBytes int64                  `json:"total_used_bytes"`
	Items          []*OpsStorageUsageItem `json:"items"`
}

type OpsStorageUsageItem struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Path      string `json:"path,omitempty"`
	UsedBytes *int64 `json:"used_bytes"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type opsStoragePathSpec struct {
	Key    string
	Label  string
	Kind   string
	Source string
	Path   string
}

// GetStorageUsage collects bounded, best-effort database and configured path sizes.
func (s *OpsService) GetStorageUsage(ctx context.Context) (*OpsStorageUsageResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("ops service unavailable")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, opsStorageCollectionTimeout)
	defer cancel()

	response := &OpsStorageUsageResponse{
		GeneratedAt: time.Now().UTC(),
		Items:       make([]*OpsStorageUsageItem, 0, 4),
	}
	response.Items = append(response.Items, s.collectPostgresDatabaseSize(ctx))
	for _, spec := range s.storagePathSpecs() {
		response.Items = append(response.Items, collectOpsStoragePath(ctx, spec))
	}
	for _, item := range response.Items {
		if item != nil && item.Status == OpsStorageStatusOK && item.UsedBytes != nil {
			response.TotalUsedBytes += *item.UsedBytes
		}
	}
	return response, nil
}

func (s *OpsService) collectPostgresDatabaseSize(ctx context.Context) *OpsStorageUsageItem {
	item := &OpsStorageUsageItem{
		Key:    "postgres_db",
		Label:  "PostgreSQL DB",
		Kind:   "database",
		Source: "postgresql",
		Status: OpsStorageStatusUnavailable,
	}
	if s == nil || s.opsRepo == nil {
		item.Error = "ops repository unavailable"
		return item
	}

	size, err := s.opsRepo.GetCurrentDatabaseSizeBytes(ctx)
	if err != nil {
		item.Error = safeOpsStorageError(err, "database size query failed")
		return item
	}
	if size < 0 {
		size = 0
	}
	item.UsedBytes = &size
	item.Status = OpsStorageStatusOK
	return item
}

func (s *OpsService) storagePathSpecs() []opsStoragePathSpec {
	specs := []opsStoragePathSpec{{
		Key:    "app_data",
		Label:  "app_data",
		Kind:   "directory",
		Source: "filesystem",
		Path:   defaultOpsAppDataPath(),
	}}

	if s != nil && s.cfg != nil {
		for _, path := range s.cfg.Ops.Storage.Paths {
			specs = upsertOpsStoragePathSpec(specs, normalizeOpsStoragePathConfig(path, "config"))
		}
	}
	for _, path := range parseOpsStorageEnvPaths(os.Getenv(opsStorageEnvPaths)) {
		specs = upsertOpsStoragePathSpec(specs, path)
	}
	return specs
}

func normalizeOpsStoragePathConfig(path config.OpsStoragePathConfig, source string) opsStoragePathSpec {
	key := normalizeOpsStorageKey(path.Key)
	trimmedPath := strings.TrimSpace(path.Path)
	if key == "" {
		key = normalizeOpsStorageKey(filepath.Base(filepath.Clean(trimmedPath)))
	}
	label := strings.TrimSpace(path.Label)
	if label == "" {
		label = key
	}
	kind := strings.TrimSpace(path.Kind)
	if kind == "" {
		kind = "directory"
	}
	return opsStoragePathSpec{
		Key:    key,
		Label:  label,
		Kind:   kind,
		Source: source,
		Path:   trimmedPath,
	}
}

func parseOpsStorageEnvPaths(raw string) []opsStoragePathSpec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	specs := make([]opsStoragePathSpec, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, path, hasKey := strings.Cut(part, "=")
		if !hasKey {
			path = part
			key = filepath.Base(filepath.Clean(path))
		}
		specs = append(specs, normalizeOpsStoragePathConfig(config.OpsStoragePathConfig{
			Key:  key,
			Path: path,
		}, "env"))
	}
	return specs
}

func upsertOpsStoragePathSpec(specs []opsStoragePathSpec, next opsStoragePathSpec) []opsStoragePathSpec {
	if strings.TrimSpace(next.Key) == "" {
		return specs
	}
	for i := range specs {
		if specs[i].Key == next.Key {
			specs[i] = next
			return specs
		}
	}
	return append(specs, next)
}

func defaultOpsAppDataPath() string {
	if dataDir := strings.TrimSpace(os.Getenv("DATA_DIR")); dataDir != "" {
		return dataDir
	}
	if _, err := os.Stat("/app/data"); err == nil {
		return "/app/data"
	}
	return "./data"
}

func normalizeOpsStorageKey(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	key = strings.ReplaceAll(key, " ", "_")
	key = opsStorageKeySanitizer.ReplaceAllString(key, "_")
	return strings.Trim(key, "_-")
}

func collectOpsStoragePath(ctx context.Context, spec opsStoragePathSpec) *OpsStorageUsageItem {
	item := &OpsStorageUsageItem{
		Key:    spec.Key,
		Label:  firstNonEmptyOpsStorageString(spec.Label, spec.Key),
		Kind:   firstNonEmptyOpsStorageString(spec.Kind, "directory"),
		Source: firstNonEmptyOpsStorageString(spec.Source, "filesystem"),
		Path:   strings.TrimSpace(spec.Path),
		Status: OpsStorageStatusUnavailable,
	}
	if item.Key == "" {
		item.Key = normalizeOpsStorageKey(filepath.Base(filepath.Clean(item.Path)))
	}
	if item.Path == "" {
		item.Status = OpsStorageStatusUnconfigured
		item.Error = "path is not configured"
		return item
	}

	size, err := calculateOpsPathSize(ctx, item.Path)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			item.Error = opsStorageTimedOutError
		} else if size > 0 {
			item.UsedBytes = &size
			item.Status = OpsStorageStatusOK
			item.Error = safeOpsStorageError(err, "configured path is partially available")
		} else {
			item.Error = safeOpsStorageError(err, "configured path is unavailable")
		}
		return item
	}

	item.UsedBytes = &size
	item.Status = OpsStorageStatusOK
	return item
}

func calculateOpsPathSize(ctx context.Context, path string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	var firstErr error
	err = opsStorageWalkDir(path, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", entryPath, walkErr)
			}
			return nil
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", entryPath, infoErr)
			}
			return nil
		}
		total += entryInfo.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, firstErr
}

func safeOpsStorageError(err error, fallback string) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return opsStorageTimedOutError
	case errors.Is(err, fs.ErrNotExist):
		return "configured path does not exist"
	case errors.Is(err, fs.ErrPermission):
		return "configured path is not accessible"
	default:
		return truncateString(fallback, opsStorageMaxErrorLength)
	}
}

func firstNonEmptyOpsStorageString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
