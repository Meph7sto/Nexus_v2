package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageInteractionCaptureMiddlewareCapturesActualDownstreamOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, false, "7")}
	interactions := NewUsageInteractionService(nil, settings)

	router := gin.New()
	router.Use(interactions.UsageInteractionCaptureMiddleware())
	var capture *UsageInteractionCapture
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(`{"id":"resp_1","output":"visible to the client"}`))
		capture = BuildUsageInteractionCaptureFromContext(c, []byte(`{"model":"test-model"}`), map[string]any{"stream": false})
	})

	recording := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model"}`))
	router.ServeHTTP(recording, request)

	require.Equal(t, http.StatusOK, recording.Code)
	require.JSONEq(t, `{"id":"resp_1","output":"visible to the client"}`, recording.Body.String())
	require.NotNil(t, capture)
	require.Equal(t, UsageInteractionCaptureComplete, capture.CaptureStatus)
	require.Equal(t, "visible to the client", capture.ResponseContent["output"])
}

func TestUsageInteractionCaptureEnforcesPayloadLimit(t *testing.T) {
	response := []byte(`{"output":"` + strings.Repeat("x", maxUsageInteractionPayloadBytes) + `"}`)
	capture := BuildUsageInteractionCapture([]byte(`{"input":"keep"}`), response, nil)

	require.Equal(t, UsageInteractionCapturePartial, capture.CaptureStatus)
	require.Equal(t, true, capture.ResponseContent["truncated"])
	require.NotNil(t, capture.CaptureError)
}

func TestUsageInteractionServiceDefaultsOffAndRedactsCapturedPayloads(t *testing.T) {
	usageLog := &UsageLog{ID: 17, RequestID: "req-17", UserID: 3, APIKeyID: 4, AccountID: 5, Model: "mapped-model", RequestedModel: "requested-model", BillingType: 2, TotalCost: 1.2, ActualCost: 0.6}

	disabledRepo := &usageInteractionTestRepository{}
	disabled := NewUsageInteractionService(disabledRepo, &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(false, false, "7")})
	require.NoError(t, disabled.RecordForUsageLog(context.Background(), usageLog, &UsageInteractionCapture{ResponseContent: map[string]any{"output": "must not store"}}, nil))
	require.False(t, disabledRepo.created)

	repo := &usageInteractionTestRepository{}
	enabled := NewUsageInteractionService(repo, &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, false, "7")})
	capture := &UsageInteractionCapture{
		RequestContent:    map[string]any{"Authorization": "Bearer request-secret", "prompt": "keep this prompt"},
		ResponseContent:   map[string]any{"output": "visible output", "token": "response-secret"},
		RequestParameters: map[string]any{"temperature": 0.2},
		RoutingContext:    map[string]any{"credential": "must redact"},
		RawRequestJSON:    map[string]any{"Authorization": "raw-request-secret"},
		RawResponseJSON:   map[string]any{"token": "raw-response-secret"},
	}

	require.NoError(t, enabled.RecordForUsageLog(context.Background(), usageLog, capture, nil))
	require.True(t, repo.created)
	require.Equal(t, UsageInteractionCaptureComplete, repo.input.CaptureStatus)
	require.Equal(t, "visible output", repo.input.ResponseContent["output"])
	require.Equal(t, "[REDACTED]", repo.input.RequestContent["Authorization"])
	require.Equal(t, "[REDACTED]", repo.input.ResponseContent["token"])
	require.Equal(t, "[REDACTED]", repo.input.RoutingContext["credential"])
	require.Nil(t, repo.input.RawRequestJSON)
	require.Nil(t, repo.input.RawResponseJSON)
	require.Equal(t, "requested-model", repo.input.RoutingContext["requested_model"])
	require.Equal(t, "mapped-model", repo.input.RoutingContext["mapped_model"])
	require.Equal(t, int64(5), repo.input.RoutingContext["account_id"])
	require.Equal(t, float64(1.2), repo.input.RoutingContext["total_cost"])
	require.True(t, repo.redactionApplied)
	require.ElementsMatch(t, []string{"Authorization", "token", "credential"}, repo.redactionKeys)
}

func TestUsageInteractionServiceRawStorageRedactsCredentials(t *testing.T) {
	repo := &usageInteractionTestRepository{}
	service := NewUsageInteractionService(repo, &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, true, "7")})

	require.NoError(t, service.RecordForUsageLog(context.Background(), &UsageLog{ID: 18, RequestID: "req-18"}, &UsageInteractionCapture{
		RawRequestJSON: map[string]any{
			"api_key":          "raw-request-secret",
			"provider_api_key": "provider-secret",
			"input_tokens":     123,
			"prompt":           "keep",
		},
		RawResponseJSON: map[string]any{"nested": map[string]any{
			"access_token": "raw-response-secret",
			"bearer_token": "bearer-secret",
			"text":         "done",
		}},
	}, nil))

	require.Equal(t, "[REDACTED]", repo.input.RawRequestJSON["api_key"])
	require.Equal(t, "[REDACTED]", repo.input.RawRequestJSON["provider_api_key"])
	require.Equal(t, 123, repo.input.RawRequestJSON["input_tokens"])
	nested := repo.input.RawResponseJSON["nested"].(map[string]any)
	require.Equal(t, "[REDACTED]", nested["access_token"])
	require.Equal(t, "[REDACTED]", nested["bearer_token"])
	require.Equal(t, "done", nested["text"])
}

func TestUsageInteractionSettingsCacheInvalidatesAndRejectsMalformedRetention(t *testing.T) {
	settings := &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, false, "7days")}
	service := NewUsageInteractionService(nil, settings)

	initial, err := service.GetSettings(context.Background())
	require.NoError(t, err)
	require.True(t, initial.RecordingEnabled)
	require.Equal(t, 7, initial.RetentionDays)

	settings.values[SettingKeyUsageInteractionRecordingEnabled] = "false"
	service.InvalidateSettingsCache()
	updated, err := service.GetSettings(context.Background())
	require.NoError(t, err)
	require.False(t, updated.RecordingEnabled)
}

func TestUsageInteractionCleanupUsesConfiguredRetention(t *testing.T) {
	repo := &usageInteractionTestRepository{deleteResult: 2}
	service := NewUsageInteractionService(repo, &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, false, "3")})
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	deleted, err := service.CleanupExpired(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)
	require.Equal(t, now.AddDate(0, 0, -3), repo.deleteCutoff)
}

func TestUsageInteractionPersistenceFailureDoesNotBlockUsageLog(t *testing.T) {
	usageRepo := &usageInteractionTestUsageLogRepository{nextID: 77}
	interactionRepo := &usageInteractionTestRepository{createErr: errors.New("interaction storage unavailable")}
	interactions := NewUsageInteractionService(interactionRepo, &usageInteractionTestSettingRepository{values: usageInteractionTestSettings(true, false, "7")})
	usageLog := &UsageLog{RequestID: "req-isolated", UserID: 2, APIKeyID: 3, AccountID: 4}

	writeUsageLogWithInteractionBestEffort(context.Background(), usageRepo, usageLog, interactions, &UsageInteractionCapture{
		RequestContent:  map[string]any{"prompt": "keep"},
		ResponseContent: map[string]any{"output": "still delivered"},
	}, "service.usage_interaction_test")

	require.Equal(t, 1, usageRepo.createCalls)
	require.Equal(t, int64(77), usageLog.ID)
	require.True(t, interactionRepo.created)
}

type usageInteractionTestRepository struct {
	created          bool
	input            UsageInteractionInput
	redactionApplied bool
	redactionKeys    []string
	createErr        error
	deleteCutoff     time.Time
	deleteResult     int64
}

func (r *usageInteractionTestRepository) Create(_ context.Context, input UsageInteractionInput, redactionApplied bool, redactionKeys []string) error {
	r.created = true
	r.input = input
	r.redactionApplied = redactionApplied
	r.redactionKeys = append([]string(nil), redactionKeys...)
	return r.createErr
}

func (r *usageInteractionTestRepository) GetByUsageLogID(context.Context, int64, bool) (*UsageInteraction, error) {
	return nil, ErrUsageInteractionNotFound
}

func (r *usageInteractionTestRepository) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	r.deleteCutoff = cutoff
	return r.deleteResult, nil
}

type usageInteractionTestSettingRepository struct {
	values map[string]string
	err    error
}

func (r *usageInteractionTestSettingRepository) Get(context.Context, string) (*Setting, error) {
	return nil, nil
}

func (r *usageInteractionTestSettingRepository) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], r.err
}

func (r *usageInteractionTestSettingRepository) Set(context.Context, string, string) error {
	return nil
}

func (r *usageInteractionTestSettingRepository) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = r.values[key]
	}
	return values, nil
}

func (r *usageInteractionTestSettingRepository) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *usageInteractionTestSettingRepository) GetAll(context.Context) (map[string]string, error) {
	return r.values, r.err
}

func (r *usageInteractionTestSettingRepository) Delete(context.Context, string) error {
	return nil
}

func usageInteractionTestSettings(recording, raw bool, retention string) map[string]string {
	return map[string]string{
		SettingKeyUsageInteractionRecordingEnabled: strconv.FormatBool(recording),
		SettingKeyUsageInteractionStoreRawEnabled:  strconv.FormatBool(raw),
		SettingKeyUsageInteractionRetentionDays:    retention,
	}
}

type usageInteractionTestUsageLogRepository struct {
	UsageLogRepository
	nextID      int64
	createCalls int
}

func (r *usageInteractionTestUsageLogRepository) Create(_ context.Context, usageLog *UsageLog) (bool, error) {
	r.createCalls++
	if usageLog.ID == 0 {
		usageLog.ID = r.nextID
	}
	return true, nil
}
