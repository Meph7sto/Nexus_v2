package middleware

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitFailureMode Redis 故障策略
type RateLimitFailureMode int

const (
	RateLimitFailOpen RateLimitFailureMode = iota
	RateLimitFailClose
)

// RateLimitOptions 限流可选配置
type RateLimitOptions struct {
	FailureMode RateLimitFailureMode
}

var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
local repaired = 0
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
elseif ttl == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  repaired = 1
end
return {current, repaired}
`)

// rateLimitRun 允许测试覆写脚本执行逻辑
var rateLimitRun = func(ctx context.Context, client *redis.Client, key string, windowMillis int64) (int64, bool, error) {
	values, err := rateLimitScript.Run(ctx, client, []string{key}, windowMillis).Slice()
	if err != nil {
		return 0, false, err
	}
	if len(values) < 2 {
		return 0, false, fmt.Errorf("rate limit script returned %d values", len(values))
	}
	count, err := parseInt64(values[0])
	if err != nil {
		return 0, false, err
	}
	repaired, err := parseInt64(values[1])
	if err != nil {
		return 0, false, err
	}
	return count, repaired == 1, nil
}

var tokenBucketRateLimitScript = redis.NewScript(`
local capacity = tonumber(ARGV[1])
local refill_per_ms = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
local state = redis.call('HMGET', KEYS[1], 'tokens', 'updated_at')
local tokens = tonumber(state[1])
local updated_at = tonumber(state[2])

if tokens == nil or updated_at == nil then
  tokens = capacity
  updated_at = now_ms
else
  tokens = math.min(capacity, tokens + math.max(0, now_ms - updated_at) * refill_per_ms)
end

local allowed = 0
local retry_ms = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  retry_ms = math.ceil((1 - tokens) / refill_per_ms)
end

redis.call('HMSET', KEYS[1], 'tokens', tokens, 'updated_at', now_ms)
redis.call('PEXPIRE', KEYS[1], ttl_ms)
return {allowed, retry_ms}
`)

// tokenBucketRateLimitRun is replaceable in tests and keeps the limiter backed by Redis.
var tokenBucketRateLimitRun = func(ctx context.Context, client *redis.Client, key string, rate int, window time.Duration, burst int) (bool, time.Duration, error) {
	windowMillis := windowTTLMillis(window)
	if rate <= 0 || burst <= 0 {
		return false, 0, fmt.Errorf("token bucket rate and burst must be positive")
	}
	refillPerMillis := float64(rate) / float64(windowMillis)
	values, err := tokenBucketRateLimitScript.Run(ctx, client, []string{key}, burst, strconv.FormatFloat(refillPerMillis, 'f', -1, 64), windowMillis).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(values) < 2 {
		return false, 0, fmt.Errorf("token bucket script returned %d values", len(values))
	}
	allowed, err := parseInt64(values[0])
	if err != nil {
		return false, 0, err
	}
	retryMillis, err := parseInt64(values[1])
	if err != nil {
		return false, 0, err
	}
	return allowed == 1, time.Duration(retryMillis) * time.Millisecond, nil
}

// RateLimiter Redis 速率限制器
type RateLimiter struct {
	redis  *redis.Client
	prefix string
}

// RateLimitKeyResolver derives a trusted, bounded identity from the request.
// Callers must use authenticated context rather than a client-supplied header or IP address.
type RateLimitKeyResolver func(*gin.Context) (string, bool)

// NewRateLimiter 创建速率限制器实例
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		prefix: "rate_limit:",
	}
}

// Limit 返回速率限制中间件
// key: 限制类型标识
// limit: 时间窗口内最大请求数
// window: 时间窗口
func (r *RateLimiter) Limit(key string, limit int, window time.Duration) gin.HandlerFunc {
	return r.LimitWithOptions(key, limit, window, RateLimitOptions{})
}

// LimitWithOptions 返回速率限制中间件（带可选配置）
func (r *RateLimiter) LimitWithOptions(key string, limit int, window time.Duration, opts RateLimitOptions) gin.HandlerFunc {
	failureMode := opts.FailureMode
	if failureMode != RateLimitFailClose {
		failureMode = RateLimitFailOpen
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		redisKey := r.prefix + key + ":" + ip

		ctx := c.Request.Context()

		windowMillis := windowTTLMillis(window)

		// 使用 Lua 脚本原子操作增加计数并设置过期
		count, repaired, err := rateLimitRun(ctx, r.redis, redisKey, windowMillis)
		if err != nil {
			log.Printf("[RateLimit] redis error: key=%s mode=%s err=%v", redisKey, failureModeLabel(failureMode), err)
			if failureMode == RateLimitFailClose {
				abortRateLimit(c)
				return
			}
			// Redis 错误时放行，避免影响正常服务
			c.Next()
			return
		}
		if repaired {
			log.Printf("[RateLimit] ttl repaired: key=%s window_ms=%d", redisKey, windowMillis)
		}

		// 超过限制
		if count > int64(limit) {
			abortRateLimit(c)
			return
		}

		c.Next()
	}
}

// LimitByKeyWithTokenBucket applies a Redis-backed token bucket to a trusted caller key.
// rate is the sustained allowance per window and burst is the immediate bucket capacity.
func (r *RateLimiter) LimitByKeyWithTokenBucket(key string, rate int, window time.Duration, burst int, resolve RateLimitKeyResolver, opts RateLimitOptions) gin.HandlerFunc {
	failureMode := opts.FailureMode
	if failureMode != RateLimitFailClose {
		failureMode = RateLimitFailOpen
	}

	return func(c *gin.Context) {
		identity, ok := resolve(c)
		if !ok || strings.TrimSpace(identity) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User not authenticated",
			})
			return
		}

		redisKey := r.prefix + key + ":" + identity
		allowed, retryAfter, err := tokenBucketRateLimitRun(c.Request.Context(), r.redis, redisKey, rate, window, burst)
		if err != nil {
			log.Printf("[RateLimit] redis error: key=%s mode=%s err=%v", redisKey, failureModeLabel(failureMode), err)
			if failureMode == RateLimitFailClose {
				abortTokenBucketRateLimit(c, time.Second)
				return
			}
			c.Next()
			return
		}
		if !allowed {
			abortTokenBucketRateLimit(c, retryAfter)
			return
		}

		c.Next()
	}
}

func windowTTLMillis(window time.Duration) int64 {
	ttl := window.Milliseconds()
	if ttl < 1 {
		return 1
	}
	return ttl
}

func abortRateLimit(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":   "rate limit exceeded",
		"message": "Too many requests, please try again later",
	})
}

func abortTokenBucketRateLimit(c *gin.Context, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	abortRateLimit(c)
}

func failureModeLabel(mode RateLimitFailureMode) string {
	if mode == RateLimitFailClose {
		return "fail-close"
	}
	return "fail-open"
}

func parseInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
