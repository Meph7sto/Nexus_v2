package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIQuotaSummaryHandlerServiceStub struct {
	service.AdminService
	response *service.OpenAIQuotaSummaryResponse
	err      error
	calls    int
	input    service.OpenAIQuotaSummaryInput
}

func (s *openAIQuotaSummaryHandlerServiceStub) GetOpenAIQuotaSummary(_ context.Context, input service.OpenAIQuotaSummaryInput) (*service.OpenAIQuotaSummaryResponse, error) {
	s.calls++
	s.input = input
	return s.response, s.err
}

func TestOpenAIQuotaSummaryHandlerValidatesParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, rawURL := range []string{
		"/api/v1/admin/openai/quota-summary?projection_at=not-a-timestamp",
		"/api/v1/admin/openai/quota-summary?group=0",
		"/api/v1/admin/openai/quota-summary?group=-1",
		"/api/v1/admin/openai/quota-summary?group=not-a-group",
	} {
		t.Run(rawURL, func(t *testing.T) {
			stub := &openAIQuotaSummaryHandlerServiceStub{}
			handler := NewOpenAIOAuthHandler(nil, stub, nil)
			router := gin.New()
			router.GET("/api/v1/admin/openai/quota-summary", handler.QuotaSummary)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, rawURL, nil))

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Zero(t, stub.calls)
		})
	}
}

func TestOpenAIQuotaSummaryHandlerReturnsEmptyResultAndForwardsFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	stub := &openAIQuotaSummaryHandlerServiceStub{
		response: &service.OpenAIQuotaSummaryResponse{
			ProjectionAt: projectionAt,
			GeneratedAt:  projectionAt,
			Groups:       []service.OpenAIQuotaSummaryGroup{},
		},
	}
	handler := NewOpenAIOAuthHandler(nil, stub, nil)
	router := gin.New()
	router.GET("/api/v1/admin/openai/quota-summary", handler.QuotaSummary)

	requestURL := "/api/v1/admin/openai/quota-summary?projection_at=" + url.QueryEscape(projectionAt.Format(time.RFC3339)) + "&group=ungrouped&type=ChatGPT+Pro"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestURL, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, stub.calls)
	require.Equal(t, projectionAt, stub.input.ProjectionAt)
	require.Equal(t, "ChatGPT Pro", stub.input.AccountType)
	require.NotNil(t, stub.input.GroupFilter)
	require.True(t, stub.input.GroupFilter.Ungrouped)
	require.Nil(t, stub.input.GroupFilter.ID)
	require.False(t, stub.input.GeneratedAt.IsZero())

	var payload struct {
		Data struct {
			Groups []service.OpenAIQuotaSummaryGroup `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Empty(t, payload.Data.Groups)
}

func TestOpenAIQuotaSummaryHandlerReturnsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &openAIQuotaSummaryHandlerServiceStub{err: errors.New("database unavailable")}
	handler := NewOpenAIOAuthHandler(nil, stub, nil)
	router := gin.New()
	router.GET("/api/v1/admin/openai/quota-summary", handler.QuotaSummary)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai/quota-summary", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, 1, stub.calls)
}
