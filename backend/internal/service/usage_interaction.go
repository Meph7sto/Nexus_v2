package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	UsageInteractionCaptureComplete = "complete"
	UsageInteractionCapturePartial  = "partial"
	UsageInteractionCaptureFailed   = "failed"

	maxUsageInteractionPayloadBytes  = 1024 * 1024
	maxUsageInteractionRetentionDays = 3650
	usageInteractionSettingsCacheTTL = 15 * time.Second
)

var ErrUsageInteractionNotFound = errors.New("usage interaction not found")

type UsageInteractionSettings struct {
	RecordingEnabled bool `json:"usage_interaction_recording_enabled"`
	StoreRawEnabled  bool `json:"usage_interaction_store_raw_enabled"`
	RetentionDays    int  `json:"usage_interaction_retention_days"`
}

type UsageInteraction struct {
	ID                int64          `json:"id"`
	UsageLogID        int64          `json:"usage_log_id"`
	RequestID         string         `json:"request_id"`
	UserID            int64          `json:"user_id"`
	APIKeyID          int64          `json:"api_key_id"`
	AccountID         int64          `json:"account_id"`
	GroupID           *int64         `json:"group_id,omitempty"`
	CaptureStatus     string         `json:"capture_status"`
	CaptureError      *string        `json:"capture_error,omitempty"`
	RequestContent    map[string]any `json:"request_content"`
	ResponseContent   map[string]any `json:"response_content"`
	RequestParameters map[string]any `json:"request_parameters"`
	RoutingContext    map[string]any `json:"routing_context"`
	RawRequestJSON    map[string]any `json:"raw_request_json,omitempty"`
	RawResponseJSON   map[string]any `json:"raw_response_json,omitempty"`
	RawAvailable      bool           `json:"raw_available"`
	RedactionApplied  bool           `json:"redaction_applied"`
	RedactionKeys     []string       `json:"redaction_keys"`
	CreatedAt         time.Time      `json:"created_at"`
}

type UsageInteractionInput struct {
	UsageLogID        int64
	RequestID         string
	UserID            int64
	APIKeyID          int64
	AccountID         int64
	GroupID           *int64
	CaptureStatus     string
	CaptureError      *string
	RequestContent    map[string]any
	ResponseContent   map[string]any
	RequestParameters map[string]any
	RoutingContext    map[string]any
	RawRequestJSON    map[string]any
	RawResponseJSON   map[string]any
	CreatedAt         time.Time
}

type UsageInteractionCapture struct {
	CaptureStatus     string
	CaptureError      *string
	RequestContent    map[string]any
	ResponseContent   map[string]any
	RequestParameters map[string]any
	RoutingContext    map[string]any
	RawRequestJSON    map[string]any
	RawResponseJSON   map[string]any
}

type UsageInteractionRepository interface {
	Create(ctx context.Context, input UsageInteractionInput, redactionApplied bool, redactionKeys []string) error
	GetByUsageLogID(ctx context.Context, usageLogID int64, includeRaw bool) (*UsageInteraction, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// usageInteractionAvailabilityRepository is deliberately separate from the
// persistence contract above so existing writers and test doubles do not need
// a list-only query. The admin usage list treats an unavailable status query
// as an optional navigation enhancement.
type usageInteractionAvailabilityRepository interface {
	ExistingUsageLogIDs(ctx context.Context, usageLogIDs []int64) (map[int64]struct{}, error)
}

type cachedUsageInteractionSettings struct {
	settings  UsageInteractionSettings
	expiresAt time.Time
}

type UsageInteractionService struct {
	repo        UsageInteractionRepository
	settingRepo SettingRepository

	settingsCache atomic.Value // *cachedUsageInteractionSettings
	settingsMu    sync.Mutex
}

func NewUsageInteractionService(repo UsageInteractionRepository, settingRepo SettingRepository) *UsageInteractionService {
	return &UsageInteractionService{repo: repo, settingRepo: settingRepo}
}

func (s *UsageInteractionService) RecordingEnabled(ctx context.Context) (bool, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return false, err
	}
	return settings.RecordingEnabled, nil
}

func (s *UsageInteractionService) CaptureEnabled(ctx context.Context) bool {
	enabled, err := s.RecordingEnabled(ctx)
	return err == nil && enabled
}

// InvalidateSettingsCache applies a successful settings update immediately.
// In particular, turning raw storage off must not leave a short cache window
// in which another request still considers it enabled.
func (s *UsageInteractionService) InvalidateSettingsCache() {
	if s != nil {
		s.settingsCache.Store(&cachedUsageInteractionSettings{})
	}
}

func (s *UsageInteractionService) GetSettings(ctx context.Context) (UsageInteractionSettings, error) {
	if s == nil || s.settingRepo == nil {
		return UsageInteractionSettings{RetentionDays: 7}, nil
	}
	if cached, ok := s.settingsCache.Load().(*cachedUsageInteractionSettings); ok && cached != nil && time.Now().Before(cached.expiresAt) {
		return cached.settings, nil
	}

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if cached, ok := s.settingsCache.Load().(*cachedUsageInteractionSettings); ok && cached != nil && time.Now().Before(cached.expiresAt) {
		return cached.settings, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyUsageInteractionRecordingEnabled,
		SettingKeyUsageInteractionStoreRawEnabled,
		SettingKeyUsageInteractionRetentionDays,
	})
	if err != nil {
		return UsageInteractionSettings{}, fmt.Errorf("get usage interaction settings: %w", err)
	}
	settings := UsageInteractionSettings{
		RecordingEnabled: strings.EqualFold(strings.TrimSpace(values[SettingKeyUsageInteractionRecordingEnabled]), "true"),
		StoreRawEnabled:  strings.EqualFold(strings.TrimSpace(values[SettingKeyUsageInteractionStoreRawEnabled]), "true"),
		RetentionDays:    7,
	}
	if raw := strings.TrimSpace(values[SettingKeyUsageInteractionRetentionDays]); raw != "" {
		if days, err := strconv.Atoi(raw); err == nil && days >= 0 && days <= maxUsageInteractionRetentionDays {
			settings.RetentionDays = days
		}
	}
	s.settingsCache.Store(&cachedUsageInteractionSettings{settings: settings, expiresAt: time.Now().Add(usageInteractionSettingsCacheTTL)})
	return settings, nil
}

func (s *UsageInteractionService) GetByUsageLogID(ctx context.Context, usageLogID int64, includeRaw bool) (*UsageInteraction, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("usage interaction service is unavailable")
	}
	interaction, err := s.repo.GetByUsageLogID(ctx, usageLogID, includeRaw)
	if err != nil {
		return nil, err
	}
	if interaction == nil {
		return nil, ErrUsageInteractionNotFound
	}
	return interaction, nil
}

// ExistingUsageLogIDs returns the usage-log IDs with a retained interaction.
// It is used only to decide whether the ordinary usage list can expose a
// navigation control; it does not load interaction content.
func (s *UsageInteractionService) ExistingUsageLogIDs(ctx context.Context, usageLogIDs []int64) (map[int64]struct{}, error) {
	available := make(map[int64]struct{})
	if s == nil || s.repo == nil || len(usageLogIDs) == 0 {
		return available, nil
	}
	repo, ok := s.repo.(usageInteractionAvailabilityRepository)
	if !ok {
		return available, nil
	}
	return repo.ExistingUsageLogIDs(ctx, usageLogIDs)
}

func (s *UsageInteractionService) RecordForUsageLog(ctx context.Context, usageLog *UsageLog, capture *UsageInteractionCapture, captureErr error) error {
	if s == nil || s.repo == nil || usageLog == nil || usageLog.ID <= 0 {
		return nil
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.RecordingEnabled {
		return nil
	}
	if capture == nil {
		message := "interaction capture was unavailable"
		capture = &UsageInteractionCapture{CaptureStatus: UsageInteractionCaptureFailed, CaptureError: &message}
	}
	if captureErr != nil {
		message := captureErr.Error()
		capture.CaptureStatus = UsageInteractionCaptureFailed
		capture.CaptureError = &message
	}
	if capture.CaptureStatus == "" {
		capture.CaptureStatus = UsageInteractionCaptureComplete
	}

	input := usageInteractionInputFromUsageLog(usageLog, capture)
	requestContent, responseContent, requestParameters, routingContext, rawRequestJSON, rawResponseJSON, captureStatus, captureError, redactionApplied, redactionKeys := redactUsageInteractionCapture(capture, input.RoutingContext, settings.StoreRawEnabled)
	input.RequestContent = requestContent
	input.ResponseContent = responseContent
	input.RequestParameters = requestParameters
	input.RoutingContext = routingContext
	input.RawRequestJSON = rawRequestJSON
	input.RawResponseJSON = rawResponseJSON
	input.CaptureStatus = captureStatus
	input.CaptureError = captureError
	return s.repo.Create(ctx, input, redactionApplied, redactionKeys)
}

func (s *UsageInteractionService) CleanupExpired(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return 0, err
	}
	if settings.RetentionDays <= 0 {
		return 0, nil
	}
	return s.repo.DeleteOlderThan(ctx, now.AddDate(0, 0, -settings.RetentionDays))
}

func BuildUsageInteractionCapture(requestBody, responseBody []byte, requestParameters map[string]any) *UsageInteractionCapture {
	return buildUsageInteractionCapture(requestBody, responseBody, false, false, requestParameters)
}

// BuildUsageInteractionCaptureWithResponseTruncation is used by WebSocket
// forwarding, where the outgoing messages are retained separately from Gin's
// HTTP response writer.
func BuildUsageInteractionCaptureWithResponseTruncation(requestBody, responseBody []byte, responseTruncated bool, requestParameters map[string]any) *UsageInteractionCapture {
	return buildUsageInteractionCapture(requestBody, responseBody, false, responseTruncated, requestParameters)
}

func appendUsageInteractionResponseBytes(dst, data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return dst, false
	}
	remaining := maxUsageInteractionPayloadBytes - len(dst)
	if remaining <= 0 {
		return dst, true
	}
	if len(data) > remaining {
		return append(dst, data[:remaining]...), true
	}
	return append(dst, data...), false
}

func buildUsageInteractionCapture(requestBody, responseBody []byte, requestKnownTruncated, responseKnownTruncated bool, requestParameters map[string]any) *UsageInteractionCapture {
	requestContent, requestTruncated := usageInteractionJSONMap(requestBody)
	responseContent, responseTruncated := usageInteractionJSONMap(responseBody)
	requestTruncated = requestTruncated || requestKnownTruncated
	responseTruncated = responseTruncated || responseKnownTruncated
	if requestTruncated {
		requestContent["truncated"] = true
	}
	if responseTruncated {
		responseContent["truncated"] = true
	}
	capture := &UsageInteractionCapture{
		CaptureStatus:     UsageInteractionCaptureComplete,
		RequestContent:    requestContent,
		ResponseContent:   responseContent,
		RequestParameters: cloneUsageInteractionMap(requestParameters),
		RoutingContext:    map[string]any{},
		RawRequestJSON:    cloneUsageInteractionMap(requestContent),
		RawResponseJSON:   cloneUsageInteractionMap(responseContent),
	}
	if len(responseBody) == 0 {
		message := "response body was not captured"
		capture.CaptureStatus = UsageInteractionCapturePartial
		capture.CaptureError = &message
	} else if requestTruncated || responseTruncated {
		message := "interaction payload exceeded the capture limit"
		capture.CaptureStatus = UsageInteractionCapturePartial
		capture.CaptureError = &message
	}
	return capture
}

func BuildUsageInteractionContentFromRequestBody(body []byte) map[string]any {
	content, _ := usageInteractionJSONMap(body)
	return content
}

func BuildUsageInteractionContentFromResponseBody(body []byte) map[string]any {
	content, _ := usageInteractionJSONMap(body)
	return content
}

func JSONMapFromRawForUsageInteraction(body []byte) map[string]any {
	content, _ := usageInteractionJSONMap(body)
	return content
}

func usageInteractionJSONMap(raw []byte) (map[string]any, bool) {
	if len(raw) == 0 {
		return map[string]any{}, false
	}
	truncated := false
	if len(raw) > maxUsageInteractionPayloadBytes {
		raw = raw[:maxUsageInteractionPayloadBytes]
		truncated = true
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{"raw_text": string(raw), "truncated": truncated}, truncated
	}
	if object, ok := decoded.(map[string]any); ok && object != nil {
		if truncated {
			object["truncated"] = true
		}
		return object, truncated
	}
	return map[string]any{"raw_json": decoded, "truncated": truncated}, truncated
}

func usageInteractionInputFromUsageLog(usageLog *UsageLog, capture *UsageInteractionCapture) UsageInteractionInput {
	input := UsageInteractionInput{}
	if usageLog != nil {
		input.UsageLogID = usageLog.ID
		input.RequestID = usageLog.RequestID
		input.UserID = usageLog.UserID
		input.APIKeyID = usageLog.APIKeyID
		input.AccountID = usageLog.AccountID
		input.GroupID = usageLog.GroupID
		input.CreatedAt = usageLog.CreatedAt
		input.RoutingContext = usageInteractionRoutingContextFromUsageLog(usageLog)
	}
	if capture == nil {
		return input
	}
	input.CaptureStatus = capture.CaptureStatus
	input.CaptureError = capture.CaptureError
	input.RequestContent = cloneUsageInteractionMap(capture.RequestContent)
	input.ResponseContent = cloneUsageInteractionMap(capture.ResponseContent)
	input.RequestParameters = cloneUsageInteractionMap(capture.RequestParameters)
	if input.RoutingContext == nil {
		input.RoutingContext = map[string]any{}
	}
	for key, value := range capture.RoutingContext {
		input.RoutingContext[key] = value
	}
	input.RawRequestJSON = cloneUsageInteractionMap(capture.RawRequestJSON)
	input.RawResponseJSON = cloneUsageInteractionMap(capture.RawResponseJSON)
	return input
}

func usageInteractionRoutingContextFromUsageLog(usageLog *UsageLog) map[string]any {
	routing := map[string]any{}
	if usageLog == nil {
		return routing
	}
	addString := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			routing[key] = value
		}
	}
	addStringPointer := func(key string, value *string) {
		if value != nil {
			addString(key, *value)
		}
	}
	if usageLog.UserID > 0 {
		routing["user_id"] = usageLog.UserID
	}
	if usageLog.APIKeyID > 0 {
		routing["api_key_id"] = usageLog.APIKeyID
	}
	if usageLog.AccountID > 0 {
		routing["account_id"] = usageLog.AccountID
	}
	if usageLog.GroupID != nil {
		routing["group_id"] = *usageLog.GroupID
	}
	if usageLog.ChannelID != nil {
		routing["channel_id"] = *usageLog.ChannelID
	}
	addStringPointer("inbound_endpoint", usageLog.InboundEndpoint)
	addStringPointer("upstream_endpoint", usageLog.UpstreamEndpoint)

	requestedModel := usageLog.RequestedModel
	if strings.TrimSpace(requestedModel) == "" {
		requestedModel = usageLog.Model
	}
	addString("requested_model", requestedModel)
	addString("mapped_model", usageLog.Model)
	if usageLog.UpstreamModel != nil {
		addString("upstream_model", *usageLog.UpstreamModel)
	} else {
		addString("upstream_model", usageLog.Model)
	}
	addStringPointer("model_mapping_chain", usageLog.ModelMappingChain)
	if requestType := usageLog.EffectiveRequestType().String(); requestType != RequestTypeUnknown.String() {
		routing["request_type"] = requestType
	}
	if usageLog.Stream {
		routing["stream"] = true
	}
	if usageLog.OpenAIWSMode {
		routing["openai_ws_mode"] = true
	}

	// Keep the billing snapshot beside the route selection so an operator can
	// explain a captured request without reconstructing historical pricing.
	routing["billing_type"] = usageLog.BillingType
	addStringPointer("billing_mode", usageLog.BillingMode)
	addStringPointer("billing_tier", usageLog.BillingTier)
	routing["rate_multiplier"] = usageLog.RateMultiplier
	routing["total_cost"] = usageLog.TotalCost
	routing["actual_cost"] = usageLog.ActualCost
	if usageLog.AccountRateMultiplier != nil {
		routing["account_rate_multiplier"] = *usageLog.AccountRateMultiplier
	}
	if usageLog.AccountStatsCost != nil {
		routing["account_stats_cost"] = *usageLog.AccountStatsCost
	}
	if usageLog.LongContextBillingApplied {
		routing["long_context_billing_applied"] = true
	}
	addStringPointer("service_tier", usageLog.ServiceTier)
	addStringPointer("reasoning_effort", usageLog.ReasoningEffort)
	return routing
}

func redactUsageInteractionCapture(capture *UsageInteractionCapture, routingInput map[string]any, includeRaw bool) (map[string]any, map[string]any, map[string]any, map[string]any, map[string]any, map[string]any, string, *string, bool, []string) {
	requestContent, requestKeys, requestChanged := RedactUsageInteractionPayload(capture.RequestContent)
	responseContent, responseKeys, responseChanged := RedactUsageInteractionPayload(capture.ResponseContent)
	requestParameters, parameterKeys, parameterChanged := RedactUsageInteractionPayload(capture.RequestParameters)
	routingContext, routingKeys, routingChanged := RedactUsageInteractionPayload(routingInput)
	keys := append(append(append(requestKeys, responseKeys...), parameterKeys...), routingKeys...)
	changed := requestChanged || responseChanged || parameterChanged || routingChanged
	var rawRequest, rawResponse map[string]any
	if includeRaw {
		var rawRequestKeys, rawResponseKeys []string
		var rawRequestChanged, rawResponseChanged bool
		rawRequest, rawRequestKeys, rawRequestChanged = RedactUsageInteractionPayload(capture.RawRequestJSON)
		rawResponse, rawResponseKeys, rawResponseChanged = RedactUsageInteractionPayload(capture.RawResponseJSON)
		keys = append(keys, rawRequestKeys...)
		keys = append(keys, rawResponseKeys...)
		changed = changed || rawRequestChanged || rawResponseChanged
	}
	keys = uniqueUsageInteractionKeys(keys)
	return requestContent, responseContent, requestParameters, routingContext, rawRequest, rawResponse, capture.CaptureStatus, capture.CaptureError, changed, keys
}

func RedactUsageInteractionPayload(input map[string]any) (map[string]any, []string, bool) {
	value, keys, changed := redactUsageInteractionValue(input)
	result, ok := value.(map[string]any)
	if !ok || result == nil {
		result = map[string]any{}
	}
	return result, uniqueUsageInteractionKeys(keys), changed
}

func redactUsageInteractionValue(value any) (any, []string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		var keys []string
		changed := false
		for key, child := range typed {
			if isUsageInteractionSecretKey(key) {
				out[key] = "[REDACTED]"
				keys = append(keys, key)
				changed = true
				continue
			}
			next, childKeys, childChanged := redactUsageInteractionValue(child)
			out[key] = next
			keys = append(keys, childKeys...)
			changed = changed || childChanged
		}
		return out, keys, changed
	case []any:
		out := make([]any, len(typed))
		var keys []string
		changed := false
		for index, child := range typed {
			next, childKeys, childChanged := redactUsageInteractionValue(child)
			out[index] = next
			keys = append(keys, childKeys...)
			changed = changed || childChanged
		}
		return out, keys, changed
	default:
		return value, nil, false
	}
}

func isUsageInteractionSecretKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "authorization", "proxyauthorization", "cookie", "setcookie", "apikey", "key", "token", "accesstoken", "refreshtoken", "idtoken", "sessiontoken", "secret", "clientsecret", "password", "credential", "credentials":
		return true
	}
	return strings.Contains(normalized, "authorization") ||
		(strings.Contains(normalized, "apikey") && !strings.HasSuffix(normalized, "id")) ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "cookie") ||
		strings.HasSuffix(normalized, "token") ||
		strings.HasSuffix(normalized, "key")
}

func uniqueUsageInteractionKeys(keys []string) []string {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			set[key] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func cloneUsageInteractionMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}
