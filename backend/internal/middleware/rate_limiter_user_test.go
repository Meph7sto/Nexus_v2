package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTokenBucketRateLimitRunUsesBurstAndRecovers(t *testing.T) {
	server := miniredis.RunT(t)
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	for range 3 {
		allowed, retryAfter, err := tokenBucketRateLimitRun(context.Background(), client, "usage-ranking:user:42", 6, time.Minute, 3)

		require.NoError(t, err)
		require.True(t, allowed)
		require.Zero(t, retryAfter)
	}

	allowed, retryAfter, err := tokenBucketRateLimitRun(context.Background(), client, "usage-ranking:user:42", 6, time.Minute, 3)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 10*time.Second, retryAfter)

	server.SetTime(now.Add(10 * time.Second))
	allowed, retryAfter, err = tokenBucketRateLimitRun(context.Background(), client, "usage-ranking:user:42", 6, time.Minute, 3)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
}

func TestTokenBucketRateLimiterUsesTrustedUserKeyAndRecovers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type call struct {
		key    string
		rate   int
		window time.Duration
		burst  int
	}
	var calls []call
	responses := []struct {
		allowed bool
		retry   time.Duration
	}{
		{allowed: true},
		{retry: 1200 * time.Millisecond},
		{allowed: true},
		{allowed: true},
	}
	originalRun := tokenBucketRateLimitRun
	tokenBucketRateLimitRun = func(_ context.Context, _ *redis.Client, key string, rate int, window time.Duration, burst int) (bool, time.Duration, error) {
		calls = append(calls, call{key: key, rate: rate, window: window, burst: burst})
		response := responses[len(calls)-1]
		return response.allowed, response.retry, nil
	}
	t.Cleanup(func() { tokenBucketRateLimitRun = originalRun })

	limiter := NewRateLimiter(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}))
	router := gin.New()
	router.Use(limiter.LimitByKeyWithTokenBucket("usage-ranking:user", 6, time.Minute, 3, func(c *gin.Context) (string, bool) {
		return c.Query("user_id"), c.Query("user_id") != ""
	}, RateLimitOptions{FailureMode: RateLimitFailClose}))
	router.GET("/ranking", func(c *gin.Context) { c.Status(http.StatusOK) })

	serve := func(userID string, remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/ranking?user_id="+userID, nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusOK, serve("42", "10.0.0.1:1234").Code)
	limited := serve("42", "10.0.0.2:5678")
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Equal(t, "2", limited.Header().Get("Retry-After"))
	require.Equal(t, http.StatusOK, serve("42", "10.0.0.3:9012").Code)
	require.Equal(t, http.StatusOK, serve("99", "10.0.0.3:9012").Code)

	require.Len(t, calls, 4)
	require.Equal(t, "rate_limit:usage-ranking:user:42", calls[0].key)
	require.Equal(t, calls[0].key, calls[1].key)
	require.Equal(t, "rate_limit:usage-ranking:user:99", calls[3].key)
	require.Equal(t, 6, calls[0].rate)
	require.Equal(t, time.Minute, calls[0].window)
	require.Equal(t, 3, calls[0].burst)
}

func TestTokenBucketRateLimiterFailsClosedAndRejectsMissingIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRun := tokenBucketRateLimitRun
	tokenBucketRateLimitRun = func(context.Context, *redis.Client, string, int, time.Duration, int) (bool, time.Duration, error) {
		return false, 0, strconv.ErrSyntax
	}
	t.Cleanup(func() { tokenBucketRateLimitRun = originalRun })

	limiter := NewRateLimiter(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}))
	router := gin.New()
	router.Use(limiter.LimitByKeyWithTokenBucket("usage-ranking:user", 6, time.Minute, 3, func(c *gin.Context) (string, bool) {
		return c.Query("user_id"), c.Query("user_id") != ""
	}, RateLimitOptions{FailureMode: RateLimitFailClose}))
	router.GET("/ranking", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ranking?user_id=42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "1", rec.Header().Get("Retry-After"))

	req = httptest.NewRequest(http.MethodGet, "/ranking", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
