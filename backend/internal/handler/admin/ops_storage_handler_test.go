package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsStorageHandlerRepository struct {
	service.OpsRepository
}

func (opsStorageHandlerRepository) GetCurrentDatabaseSizeBytes(context.Context) (int64, error) {
	return 42, nil
}

func TestOpsStorageHandlerServiceUnavailable(t *testing.T) {
	router := newOpsStorageHandlerTestRouter(NewOpsHandler(nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/storage", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestOpsStorageHandlerMonitoringDisabled(t *testing.T) {
	svc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil)
	router := newOpsStorageHandlerTestRouter(NewOpsHandler(svc))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/storage", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestOpsStorageHandlerSuccess(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("OPS_STORAGE_PATHS", "")
	svc := service.NewOpsService(opsStorageHandlerRepository{}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	router := newOpsStorageHandlerTestRouter(NewOpsHandler(svc))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/storage", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int                             `json:"code"`
		Data service.OpsStorageUsageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code)
	require.NotZero(t, response.Data.GeneratedAt)
	require.NotEmpty(t, response.Data.Items)
}

func newOpsStorageHandlerTestRouter(handler *OpsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/storage", handler.GetStorageUsage)
	return router
}
