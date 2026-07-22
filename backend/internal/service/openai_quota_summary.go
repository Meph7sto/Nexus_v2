package service

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const openAIQuotaSummaryUnknownPlanType = "unknown"

var openAIQuotaSummaryCredentialPlanKeys = []string{
	"plan_type",
	"chatgpt_plan_type",
	"chatgpt_plan",
	"subscription_plan",
	"subscription_tier",
	"tier",
}

var openAIQuotaSummaryExtraPlanKeys = []string{
	"chatgpt_plan_type",
	"chatgpt_plan",
	"plan",
	"account_plan",
	"subscription_plan",
	"subscription_tier",
	"tier",
	"quota_tier",
	"workspace_plan",
	"codex_plan",
}

// OpenAIQuotaSummaryInput controls the point-in-time projection and optional
// group and plan-type filters used to build an OpenAI quota summary.
type OpenAIQuotaSummaryInput struct {
	ProjectionAt time.Time
	GeneratedAt  time.Time
	GroupFilter  *OpenAIQuotaSummaryGroupFilter
	AccountType  string
}

type OpenAIQuotaSummaryGroupFilter struct {
	ID        *int64
	Ungrouped bool
}

type OpenAIQuotaSummaryResponse struct {
	ProjectionAt time.Time                 `json:"projection_at"`
	GeneratedAt  time.Time                 `json:"generated_at"`
	Groups       []OpenAIQuotaSummaryGroup `json:"groups"`
}

type OpenAIQuotaSummaryGroup struct {
	GroupID   *int64                  `json:"group_id"`
	GroupName string                  `json:"group_name"`
	Ungrouped bool                    `json:"ungrouped"`
	Rows      []OpenAIQuotaSummaryRow `json:"rows"`
}

type OpenAIQuotaSummaryRow struct {
	AccountType            string               `json:"account_type"`
	IncludedCount          int                  `json:"included_count"`
	ErrorCount             int                  `json:"error_count"`
	InactiveCount          int                  `json:"inactive_count"`
	OtherExcludedCount     int                  `json:"other_excluded_count"`
	Missing5HSnapshotCount int                  `json:"missing_5h_snapshot_count"`
	Missing7DSnapshotCount int                  `json:"missing_7d_snapshot_count"`
	Avg5HRemainingPercent  float64              `json:"avg_5h_remaining_percent"`
	Avg7DRemainingPercent  float64              `json:"avg_7d_remaining_percent"`
	Earliest5HRecovery     *OpenAIQuotaRecovery `json:"earliest_5h_recovery"`
	Earliest7DRecovery     *OpenAIQuotaRecovery `json:"earliest_7d_recovery"`
}

type OpenAIQuotaRecovery struct {
	AccountID              int64     `json:"account_id"`
	AccountName            string    `json:"account_name"`
	AccountType            string    `json:"account_type"`
	ResetAt                time.Time `json:"reset_at"`
	RemainingBeforePercent float64   `json:"remaining_before_percent"`
	RemainingAfterPercent  float64   `json:"remaining_after_percent"`
}

type openAIQuotaSummaryGroupAccumulator struct {
	groupID   int64
	groupName string
	ungrouped bool
	rows      map[string]*openAIQuotaSummaryRowAccumulator
}

type openAIQuotaSummaryRowAccumulator struct {
	accountType        string
	includedCount      int
	errorCount         int
	inactiveCount      int
	otherExcludedCount int
	missing5HCount     int
	missing7DCount     int
	fiveHWindows       []openAIQuotaSummaryWindow
	sevenDWindows      []openAIQuotaSummaryWindow
}

type openAIQuotaSummaryWindow struct {
	remainingPercent float64
	resetAt          time.Time
	valid            bool
}

type openAIQuotaSummaryMembership struct {
	groupID   int64
	groupName string
	ungrouped bool
}

// BuildOpenAIQuotaSummary turns persisted account quota snapshots into a
// deterministic projection. It deliberately only reads the canonical Codex
// five-hour and seven-day snapshot fields; it does not affect quota resets,
// account health, gateway traffic, or scheduler state.
func BuildOpenAIQuotaSummary(accounts []Account, input OpenAIQuotaSummaryInput) OpenAIQuotaSummaryResponse {
	planTypeFilter := strings.TrimSpace(input.AccountType)

	groups := make(map[string]*openAIQuotaSummaryGroupAccumulator)
	for _, account := range accounts {
		if account.Platform != PlatformOpenAI {
			continue
		}

		planType := openAIQuotaSummaryPlanType(account)
		if planTypeFilter != "" && !strings.EqualFold(planType, planTypeFilter) {
			continue
		}

		for _, membership := range openAIQuotaSummaryMemberships(account) {
			if !matchesOpenAIQuotaSummaryGroupFilter(membership, input.GroupFilter) {
				continue
			}

			group := getOpenAIQuotaSummaryGroupAccumulator(groups, membership)
			row := group.rows[planType]
			if row == nil {
				row = &openAIQuotaSummaryRowAccumulator{accountType: planType}
				group.rows[planType] = row
			}

			switch openAIQuotaSummaryAccountStatus(account.Status) {
			case openAIQuotaSummaryStatusActive:
				row.includedCount++
				fiveHWindow := openAIQuotaSummaryWindowFor(account.Extra, "codex_5h_used_percent", "codex_5h_reset_at", input.ProjectionAt)
				sevenDWindow := openAIQuotaSummaryWindowFor(account.Extra, "codex_7d_used_percent", "codex_7d_reset_at", input.ProjectionAt)
				row.fiveHWindows = append(row.fiveHWindows, fiveHWindow)
				row.sevenDWindows = append(row.sevenDWindows, sevenDWindow)
				if !fiveHWindow.valid {
					row.missing5HCount++
				}
				if !sevenDWindow.valid {
					row.missing7DCount++
				}
			case openAIQuotaSummaryStatusError:
				row.errorCount++
			case openAIQuotaSummaryStatusInactive:
				row.inactiveCount++
			default:
				row.otherExcludedCount++
			}
		}
	}

	result := OpenAIQuotaSummaryResponse{
		ProjectionAt: input.ProjectionAt,
		GeneratedAt:  input.GeneratedAt,
		Groups:       make([]OpenAIQuotaSummaryGroup, 0, len(groups)),
	}
	for _, group := range groups {
		rows := make([]OpenAIQuotaSummaryRow, 0, len(group.rows))
		for _, row := range group.rows {
			rows = append(rows, row.response(input.ProjectionAt))
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].AccountType < rows[j].AccountType
		})

		responseGroup := OpenAIQuotaSummaryGroup{
			GroupName: group.groupName,
			Ungrouped: group.ungrouped,
			Rows:      rows,
		}
		if !group.ungrouped {
			groupID := group.groupID
			responseGroup.GroupID = &groupID
		}
		result.Groups = append(result.Groups, responseGroup)
	}

	sort.Slice(result.Groups, func(i, j int) bool {
		if result.Groups[i].Ungrouped != result.Groups[j].Ungrouped {
			return !result.Groups[i].Ungrouped
		}
		if result.Groups[i].Ungrouped {
			return false
		}
		return *result.Groups[i].GroupID < *result.Groups[j].GroupID
	})
	return result
}

func getOpenAIQuotaSummaryGroupAccumulator(groups map[string]*openAIQuotaSummaryGroupAccumulator, membership openAIQuotaSummaryMembership) *openAIQuotaSummaryGroupAccumulator {
	key := "ungrouped"
	if !membership.ungrouped {
		key = "group:" + strconv.FormatInt(membership.groupID, 10)
	}
	if group := groups[key]; group != nil {
		if group.groupName == "" && membership.groupName != "" {
			group.groupName = membership.groupName
		}
		return group
	}

	group := &openAIQuotaSummaryGroupAccumulator{
		groupID:   membership.groupID,
		groupName: membership.groupName,
		ungrouped: membership.ungrouped,
		rows:      make(map[string]*openAIQuotaSummaryRowAccumulator),
	}
	groups[key] = group
	return group
}

func openAIQuotaSummaryMemberships(account Account) []openAIQuotaSummaryMembership {
	seenGroupIDs := make(map[int64]struct{}, len(account.Groups)+len(account.GroupIDs))
	memberships := make([]openAIQuotaSummaryMembership, 0, len(account.Groups)+len(account.GroupIDs))
	for _, group := range account.Groups {
		if group == nil {
			continue
		}
		if _, exists := seenGroupIDs[group.ID]; exists {
			continue
		}
		seenGroupIDs[group.ID] = struct{}{}
		memberships = append(memberships, openAIQuotaSummaryMembership{
			groupID:   group.ID,
			groupName: group.Name,
		})
	}
	for _, groupID := range account.GroupIDs {
		if _, exists := seenGroupIDs[groupID]; exists {
			continue
		}
		seenGroupIDs[groupID] = struct{}{}
		memberships = append(memberships, openAIQuotaSummaryMembership{groupID: groupID})
	}
	if len(memberships) == 0 {
		return []openAIQuotaSummaryMembership{{ungrouped: true, groupName: "Ungrouped"}}
	}
	return memberships
}

func matchesOpenAIQuotaSummaryGroupFilter(membership openAIQuotaSummaryMembership, filter *OpenAIQuotaSummaryGroupFilter) bool {
	if filter == nil {
		return true
	}
	if filter.Ungrouped {
		return membership.ungrouped
	}
	if filter.ID != nil {
		return !membership.ungrouped && membership.groupID == *filter.ID
	}
	return true
}

func openAIQuotaSummaryPlanType(account Account) string {
	for _, key := range openAIQuotaSummaryCredentialPlanKeys {
		if value, ok := openAIQuotaSummaryStringValue(account.Credentials[key]); ok {
			return normalizeOpenAIQuotaSummaryPlanType(value)
		}
	}
	for _, key := range openAIQuotaSummaryExtraPlanKeys {
		if value, ok := openAIQuotaSummaryStringValue(account.Extra[key]); ok {
			return normalizeOpenAIQuotaSummaryPlanType(value)
		}
	}
	return openAIQuotaSummaryUnknownPlanType
}

func openAIQuotaSummaryStringValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		value := strings.TrimSpace(typed)
		return value, value != ""
	case json.Number:
		value := strings.TrimSpace(typed.String())
		return value, value != ""
	default:
		return "", false
	}
}

func normalizeOpenAIQuotaSummaryPlanType(value string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "-")
	for _, prefix := range []string{"chatgpt-", "chatgpt_"} {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			break
		}
	}
	if normalized == "" {
		return openAIQuotaSummaryUnknownPlanType
	}
	return normalized
}

type openAIQuotaSummaryStatus int

const (
	openAIQuotaSummaryStatusOther openAIQuotaSummaryStatus = iota
	openAIQuotaSummaryStatusActive
	openAIQuotaSummaryStatusError
	openAIQuotaSummaryStatusInactive
)

func openAIQuotaSummaryAccountStatus(status string) openAIQuotaSummaryStatus {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case strings.ToLower(string(StatusActive)):
		return openAIQuotaSummaryStatusActive
	case strings.ToLower(string(StatusError)):
		return openAIQuotaSummaryStatusError
	case "inactive", strings.ToLower(string(StatusDisabled)):
		return openAIQuotaSummaryStatusInactive
	default:
		return openAIQuotaSummaryStatusOther
	}
}

func openAIQuotaSummaryWindowFor(extra map[string]any, usedKey, resetKey string, projectionAt time.Time) openAIQuotaSummaryWindow {
	usedPercent, usedOK := openAIQuotaSummaryFloat(extra[usedKey])
	resetAt, resetOK := openAIQuotaSummaryTime(extra[resetKey])
	if !usedOK || !resetOK {
		return openAIQuotaSummaryWindow{remainingPercent: 100}
	}

	remainingPercent := 100.0
	if projectionAt.Before(resetAt) {
		remainingPercent = 100 - clampOpenAIQuotaSummaryPercent(usedPercent)
	}
	return openAIQuotaSummaryWindow{
		remainingPercent: remainingPercent,
		resetAt:          resetAt,
		valid:            true,
	}
}

func openAIQuotaSummaryFloat(value any) (float64, bool) {
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int8:
		parsed = float64(typed)
	case int16:
		parsed = float64(typed)
	case int32:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case uint:
		parsed = float64(typed)
	case uint8:
		parsed = float64(typed)
	case uint16:
		parsed = float64(typed)
	case uint32:
		parsed = float64(typed)
	case uint64:
		parsed = float64(typed)
	case json.Number:
		value, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		parsed = value
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		parsed = value
	default:
		return 0, false
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func openAIQuotaSummaryTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		if typed.IsZero() {
			return time.Time{}, false
		}
		return typed, true
	case *time.Time:
		if typed == nil || typed.IsZero() {
			return time.Time{}, false
		}
		return *typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		if err != nil || parsed.IsZero() {
			return time.Time{}, false
		}
		return parsed, true
	default:
		return time.Time{}, false
	}
}

func clampOpenAIQuotaSummaryPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func (row *openAIQuotaSummaryRowAccumulator) response(projectionAt time.Time) OpenAIQuotaSummaryRow {
	response := OpenAIQuotaSummaryRow{
		AccountType:            row.accountType,
		IncludedCount:          row.includedCount,
		ErrorCount:             row.errorCount,
		InactiveCount:          row.inactiveCount,
		OtherExcludedCount:     row.otherExcludedCount,
		Missing5HSnapshotCount: row.missing5HCount,
		Missing7DSnapshotCount: row.missing7DCount,
	}
	response.Avg5HRemainingPercent = averageOpenAIQuotaSummaryWindows(row.fiveHWindows)
	response.Avg7DRemainingPercent = averageOpenAIQuotaSummaryWindows(row.sevenDWindows)
	response.Earliest5HRecovery = openAIQuotaSummaryRecovery(row.fiveHWindows, projectionAt, response.Avg5HRemainingPercent)
	response.Earliest7DRecovery = openAIQuotaSummaryRecovery(row.sevenDWindows, projectionAt, response.Avg7DRemainingPercent)
	return response
}

func averageOpenAIQuotaSummaryWindows(windows []openAIQuotaSummaryWindow) float64 {
	if len(windows) == 0 {
		return 0
	}
	total := 0.0
	for _, window := range windows {
		total += window.remainingPercent
	}
	return total / float64(len(windows))
}

func openAIQuotaSummaryRecovery(windows []openAIQuotaSummaryWindow, projectionAt time.Time, beforePercent float64) *OpenAIQuotaRecovery {
	var earliestReset time.Time
	for _, window := range windows {
		if !window.valid || !window.resetAt.After(projectionAt) || window.remainingPercent >= 100 {
			continue
		}
		if earliestReset.IsZero() || window.resetAt.Before(earliestReset) {
			earliestReset = window.resetAt
		}
	}
	if earliestReset.IsZero() {
		return nil
	}

	afterTotal := 0.0
	for _, window := range windows {
		remainingPercent := window.remainingPercent
		if window.valid && !window.resetAt.After(earliestReset) {
			remainingPercent = 100
		}
		afterTotal += remainingPercent
	}
	return &OpenAIQuotaRecovery{
		ResetAt:                earliestReset,
		RemainingBeforePercent: beforePercent,
		RemainingAfterPercent:  afterTotal / float64(len(windows)),
	}
}
