package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIQuotaSummaryAggregatesStatusesAndSnapshots(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	group := &Group{ID: 10, Name: "Alpha"}

	accounts := []Account{
		openAIQuotaSummaryTestAccount(1, "active-a", StatusActive, map[string]any{"plan_type": "Pro"}, openAIQuotaSummaryCompleteExtra(projectionAt, 25, 40), group),
		openAIQuotaSummaryTestAccount(2, "active-b", StatusActive, map[string]any{"plan_type": "Pro"}, openAIQuotaSummaryCompleteExtra(projectionAt, 50, 60), group),
		openAIQuotaSummaryTestAccount(3, "error", StatusError, map[string]any{"plan_type": "Pro"}, nil, group),
		openAIQuotaSummaryTestAccount(4, "inactive", "inactive", map[string]any{"plan_type": "Pro"}, nil, group),
		openAIQuotaSummaryTestAccount(5, "disabled", StatusDisabled, map[string]any{"plan_type": "Pro"}, nil, group),
		openAIQuotaSummaryTestAccount(6, "other", "suspended", map[string]any{"plan_type": "Pro"}, nil, group),
	}

	summary := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	require.Len(t, summary.Groups, 1)
	require.Len(t, summary.Groups[0].Rows, 1)
	row := summary.Groups[0].Rows[0]
	require.Equal(t, "pro", row.AccountType)
	require.Equal(t, 2, row.IncludedCount)
	require.Equal(t, 1, row.ErrorCount)
	require.Equal(t, 2, row.InactiveCount)
	require.Equal(t, 1, row.OtherExcludedCount)
	require.Zero(t, row.Missing5HSnapshotCount)
	require.Zero(t, row.Missing7DSnapshotCount)
	require.InDelta(t, 62.5, row.Avg5HRemainingPercent, 0.001)
	require.InDelta(t, 50.0, row.Avg7DRemainingPercent, 0.001)
}

func TestBuildOpenAIQuotaSummaryTreatsPartialAndNonFiniteSnapshotsAsMissing(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	group := &Group{ID: 10, Name: "Alpha"}
	resetAt := projectionAt.Add(time.Hour).Format(time.RFC3339)

	accounts := []Account{
		openAIQuotaSummaryTestAccount(1, "valid", StatusActive, map[string]any{"plan_type": "pro"}, openAIQuotaSummaryCompleteExtra(projectionAt, 20, 30), group),
		openAIQuotaSummaryTestAccount(2, "partial", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": 40,
			"codex_7d_reset_at":     resetAt,
		}, group),
		openAIQuotaSummaryTestAccount(3, "string-nonfinite", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": "NaN",
			"codex_5h_reset_at":     resetAt,
			"codex_7d_used_percent": "Infinity",
			"codex_7d_reset_at":     resetAt,
		}, group),
		openAIQuotaSummaryTestAccount(4, "float-nonfinite", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": math.NaN(),
			"codex_5h_reset_at":     resetAt,
			"codex_7d_used_percent": math.Inf(1),
			"codex_7d_reset_at":     resetAt,
		}, group),
		openAIQuotaSummaryTestAccount(5, "invalid", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": "not-a-number",
			"codex_5h_reset_at":     resetAt,
			"codex_7d_used_percent": 50,
			"codex_7d_reset_at":     "not-a-timestamp",
		}, group),
	}

	summary := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	row := summary.Groups[0].Rows[0]
	require.Equal(t, 5, row.IncludedCount)
	require.Equal(t, 4, row.Missing5HSnapshotCount)
	require.Equal(t, 4, row.Missing7DSnapshotCount)
	require.InDelta(t, 96.0, row.Avg5HRemainingPercent, 0.001)
	require.InDelta(t, 94.0, row.Avg7DRemainingPercent, 0.001)

	payload, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "NaN")
	require.NotContains(t, string(payload), "Inf")
}

func TestBuildOpenAIQuotaSummaryProjectsExpiredWindowsAndAggregatesRecovery(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	group := &Group{ID: 10, Name: "Alpha"}
	earlyReset := projectionAt.Add(time.Hour)
	lateReset := projectionAt.Add(2 * time.Hour)
	expiredReset := projectionAt.Add(-time.Hour)

	accounts := []Account{
		openAIQuotaSummaryTestAccount(1, "first", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": 60,
			"codex_5h_reset_at":     earlyReset.Format(time.RFC3339),
			"codex_7d_used_percent": 40,
			"codex_7d_reset_at":     lateReset.Format(time.RFC3339),
		}, group),
		openAIQuotaSummaryTestAccount(2, "second", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": 20,
			"codex_5h_reset_at":     lateReset.Format(time.RFC3339),
			"codex_7d_used_percent": 50,
			"codex_7d_reset_at":     earlyReset.Format(time.RFC3339),
		}, group),
		openAIQuotaSummaryTestAccount(3, "expired", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": 90,
			"codex_5h_reset_at":     expiredReset.Format(time.RFC3339),
			"codex_7d_used_percent": 80,
			"codex_7d_reset_at":     lateReset.Format(time.RFC3339),
		}, group),
	}

	summary := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	row := summary.Groups[0].Rows[0]
	require.Zero(t, row.Missing5HSnapshotCount)
	require.Zero(t, row.Missing7DSnapshotCount)
	require.InDelta(t, 220.0/3.0, row.Avg5HRemainingPercent, 0.001)
	require.InDelta(t, 130.0/3.0, row.Avg7DRemainingPercent, 0.001)

	require.NotNil(t, row.Earliest5HRecovery)
	require.Equal(t, earlyReset, row.Earliest5HRecovery.ResetAt)
	require.InDelta(t, 220.0/3.0, row.Earliest5HRecovery.RemainingBeforePercent, 0.001)
	require.InDelta(t, 280.0/3.0, row.Earliest5HRecovery.RemainingAfterPercent, 0.001)

	require.NotNil(t, row.Earliest7DRecovery)
	require.Equal(t, earlyReset, row.Earliest7DRecovery.ResetAt)
	require.InDelta(t, 130.0/3.0, row.Earliest7DRecovery.RemainingBeforePercent, 0.001)
	require.InDelta(t, 180.0/3.0, row.Earliest7DRecovery.RemainingAfterPercent, 0.001)
}

func TestBuildOpenAIQuotaSummaryClampsPercentEdges(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	group := &Group{ID: 10, Name: "Alpha"}
	resetAt := projectionAt.Add(time.Hour).Format(time.RFC3339)

	accounts := []Account{
		openAIQuotaSummaryTestAccount(1, "low", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": -10,
			"codex_5h_reset_at":     resetAt,
			"codex_7d_used_percent": 0,
			"codex_7d_reset_at":     resetAt,
		}, group),
		openAIQuotaSummaryTestAccount(2, "high", StatusActive, map[string]any{"plan_type": "pro"}, map[string]any{
			"codex_5h_used_percent": 100,
			"codex_5h_reset_at":     resetAt,
			"codex_7d_used_percent": 150,
			"codex_7d_reset_at":     resetAt,
		}, group),
	}

	summary := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	row := summary.Groups[0].Rows[0]
	require.InDelta(t, 50.0, row.Avg5HRemainingPercent, 0.001)
	require.InDelta(t, 50.0, row.Avg7DRemainingPercent, 0.001)
	require.Zero(t, row.Missing5HSnapshotCount)
	require.Zero(t, row.Missing7DSnapshotCount)
}

func TestBuildOpenAIQuotaSummaryGroupsFiltersAndResolvesPlanType(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	alpha := &Group{ID: 20, Name: "Alpha"}
	beta := &Group{ID: 10, Name: "Beta"}

	accounts := []Account{
		openAIQuotaSummaryTestAccount(1, "credential-wins", StatusActive, map[string]any{"plan_type": "Team"}, map[string]any{
			"plan": "enterprise",
		}, alpha, beta),
		openAIQuotaSummaryTestAccount(2, "pat-fallback", StatusActive, map[string]any{"chatgpt_plan_type": " Plus "}, nil),
		openAIQuotaSummaryTestAccount(3, "extra-fallback", StatusActive, nil, map[string]any{
			"chatgpt_plan": "ChatGPT Pro",
		}, alpha),
		openAIQuotaSummaryTestAccount(4, "unknown", StatusActive, nil, nil, beta),
	}
	for index := range accounts {
		accounts[index].Extra = openAIQuotaSummaryCompleteExtra(projectionAt, 10, 20)
		if accounts[index].ID == 1 {
			accounts[index].Extra["plan"] = "enterprise"
		}
		if accounts[index].ID == 3 {
			accounts[index].Extra["chatgpt_plan"] = "ChatGPT Pro"
		}
	}

	summary := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	require.Len(t, summary.Groups, 3)
	require.Equal(t, int64(10), *summary.Groups[0].GroupID)
	require.Equal(t, int64(20), *summary.Groups[1].GroupID)
	require.True(t, summary.Groups[2].Ungrouped)
	require.Equal(t, []string{"team", "unknown"}, openAIQuotaSummaryRowTypes(summary.Groups[0].Rows))
	require.Equal(t, []string{"pro", "team"}, openAIQuotaSummaryRowTypes(summary.Groups[1].Rows))
	require.Equal(t, []string{"plus"}, openAIQuotaSummaryRowTypes(summary.Groups[2].Rows))

	groupID := int64(10)
	filtered := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
		GroupFilter:  &OpenAIQuotaSummaryGroupFilter{ID: &groupID},
		AccountType:  "team",
	})
	require.Len(t, filtered.Groups, 1)
	require.Equal(t, int64(10), *filtered.Groups[0].GroupID)
	require.Equal(t, []string{"team"}, openAIQuotaSummaryRowTypes(filtered.Groups[0].Rows))

	ungrouped := BuildOpenAIQuotaSummary(accounts, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
		GroupFilter:  &OpenAIQuotaSummaryGroupFilter{Ungrouped: true},
		AccountType:  "plus",
	})
	require.Len(t, ungrouped.Groups, 1)
	require.True(t, ungrouped.Groups[0].Ungrouped)
	require.Equal(t, []string{"plus"}, openAIQuotaSummaryRowTypes(ungrouped.Groups[0].Rows))
}

func TestBuildOpenAIQuotaSummaryHandlesZeroAccounts(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	summary := BuildOpenAIQuotaSummary(nil, OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	require.Empty(t, summary.Groups)
	require.Equal(t, projectionAt, summary.ProjectionAt)
	require.Equal(t, projectionAt, summary.GeneratedAt)
}

func openAIQuotaSummaryTestAccount(id int64, name, status string, credentials, extra map[string]any, groups ...*Group) Account {
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	return Account{
		ID:          id,
		Name:        name,
		Platform:    PlatformOpenAI,
		Status:      status,
		Credentials: credentials,
		Extra:       extra,
		Groups:      groups,
		GroupIDs:    groupIDs,
	}
}

func openAIQuotaSummaryCompleteExtra(projectionAt time.Time, used5H, used7D float64) map[string]any {
	return map[string]any{
		"codex_5h_used_percent": used5H,
		"codex_5h_reset_at":     projectionAt.Add(time.Hour).Format(time.RFC3339),
		"codex_7d_used_percent": used7D,
		"codex_7d_reset_at":     projectionAt.Add(2 * time.Hour).Format(time.RFC3339),
	}
}

func openAIQuotaSummaryRowTypes(rows []OpenAIQuotaSummaryRow) []string {
	types := make([]string, 0, len(rows))
	for _, row := range rows {
		types = append(types, row.AccountType)
	}
	return types
}
