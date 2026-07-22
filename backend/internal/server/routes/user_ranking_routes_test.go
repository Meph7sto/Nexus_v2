package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRegisterUserRoutesProtectsUsageRankingWithJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	handlers := &handler.Handlers{
		User:             &handler.UserHandler{},
		APIKey:           &handler.APIKeyHandler{},
		Usage:            &handler.UsageHandler{},
		Redeem:           &handler.RedeemHandler{},
		Subscription:     &handler.SubscriptionHandler{},
		Announcement:     &handler.AnnouncementHandler{},
		ChannelMonitor:   &handler.ChannelMonitorUserHandler{},
		Totp:             &handler.TotpHandler{},
		AvailableChannel: &handler.AvailableChannelHandler{},
	}
	jwtAuth := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = redisClient.Close() })

	RegisterUserRoutes(v1, handlers, jwtAuth, auditLog, nil, redisClient)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/ranking", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
