// Package middleware provides HTTP middleware functions for the Veer backend.
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// limiterEntry 存储每个 IP 的限流器
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter 基于 IP 的限流器管理器
type IPRateLimiter struct {
	limiters  sync.Map // map[string]*limiterEntry
	rps       rate.Limit
	burst     int
	whitelist map[string]bool
	mu        sync.RWMutex
}

// 全局限流器实例（支持多配置）
var (
	limiters   = make(map[string]*IPRateLimiter)
	limitersMu sync.RWMutex
)

// getOrCreateLimiter 获取或创建指定 IP 的限流器
//
// 参数:
//   - ip: 客户端 IP 地址
//   - limiter: IP 限流器管理器
//
// 返回:
//   - *rate.Limiter: 该 IP 对应的令牌桶限流器
func getOrCreateLimiter(ip string, limiter *IPRateLimiter) *rate.Limiter {
	entry, exists := limiter.limiters.Load(ip)
	if exists {
		le := entry.(*limiterEntry)
		le.lastSeen = time.Now()
		return le.limiter
	}

	// 创建新的限流器
	newLimiter := rate.NewLimiter(limiter.rps, limiter.burst)
	limiter.limiters.Store(ip, &limiterEntry{
		limiter:  newLimiter,
		lastSeen: time.Now(),
	})

	return newLimiter
}

// cleanupOldLimiters 定期清理过期的限流器（释放内存）
//
// 参数:
//   - limiter: IP 限流器管理器
//   - maxAge: 限流器最大存活时间
func cleanupOldLimiters(limiter *IPRateLimiter, maxAge time.Duration) {
	ticker := time.NewTicker(maxAge)
	go func() {
		for range ticker.C {
			limiter.limiters.Range(func(key, value interface{}) bool {
				entry := value.(*limiterEntry)
				if time.Since(entry.lastSeen) > maxAge {
					limiter.limiters.Delete(key)
				}
				return true
			})
		}
	}()
}

// NewIPRateLimiter 创建新的 IP 限流器
//
// 参数:
//   - key: 限流器标识符
//   - rps: 每秒请求数（Requests Per Second）
//   - burst: 突发容量（允许的临时最大请求数）
//   - whitelist: IP 白名单列表
func NewIPRateLimiter(key string, rps float64, burst int, whitelist []string) *IPRateLimiter {
	limiter := &IPRateLimiter{
		rps:       rate.Limit(rps),
		burst:     burst,
		whitelist: make(map[string]bool),
	}

	// 初始化白名单
	for _, ip := range whitelist {
		limiter.whitelist[ip] = true
	}

	// 启动清理协程，清理超过 10 分钟未使用的限流器
	cleanupOldLimiters(limiter, 10*time.Minute)

	// 注册到全局 map
	limitersMu.Lock()
	limiters[key] = limiter
	limitersMu.Unlock()

	return limiter
}

// GetRateLimiter 获取指定标识符的限流器
func GetRateLimiter(key string) *IPRateLimiter {
	limitersMu.RLock()
	defer limitersMu.RUnlock()
	return limiters[key]
}

// isWhitelisted 检查 IP 是否在白名单中
func (l *IPRateLimiter) isWhitelisted(ip string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.whitelist[ip]
}

// Allow 检查是否允许该 IP 的请求
func (l *IPRateLimiter) Allow(ip string) bool {
	limiter := getOrCreateLimiter(ip, l)
	return limiter.Allow()
}

// getClientIP 获取客户端真实 IP
//
// 优先从 X-Forwarded-For 或 X-Real-IP 获取，否则使用 RemoteAddr
func getClientIP(c *gin.Context) string {
	// 优先从代理头获取
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个（最原始的客户端 IP）
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	xrip := c.GetHeader("X-Real-IP")
	if xrip != "" {
		return xrip
	}

	// 使用 RemoteAddr
	return c.ClientIP()
}

// RateLimitMiddleware 限流中间件
//
// 使用令牌桶算法实现基于 IP 的请求限流。
// 白名单中的 IP 将直接放行，不受限制。
// 超出限流阈值时返回 429 Too Many Requests 并附带 Retry-After 头。
//
// 参数:
//   - rps: 每秒请求数
//   - burst: 突发容量
//   - whitelist: IP 白名单列表
//
// 返回:
//   - gin.HandlerFunc: Gin 中间件处理函数
func RateLimitMiddleware(rps float64, burst int, whitelist []string) gin.HandlerFunc {
	limiter := NewIPRateLimiter("default", rps, burst, whitelist)

	return func(c *gin.Context) {
		ip := getClientIP(c)

		// 白名单直接放行
		if limiter.isWhitelisted(ip) {
			c.Next()
			return
		}

		// 检查限流
		if !limiter.Allow(ip) {
			retryAfter := strconv.Itoa(int(time.Minute / time.Second))
			c.Header("Retry-After", retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests, please try again later",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// RateLimitMiddlewareWithConfig 使用配置创建限流中间件
//
// 参数:
//   - requestsPerMinute: 每分钟请求数
//   - whitelist: IP 白名单列表
//
// 返回:
//   - gin.HandlerFunc: Gin 中间件处理函数
func RateLimitMiddlewareWithConfig(requestsPerMinute int, whitelist []string) gin.HandlerFunc {
	rps := float64(requestsPerMinute) / 60.0
	burst := requestsPerMinute / 10
	if burst < 1 {
		burst = 1
	}
	return RateLimitMiddleware(rps, burst, whitelist)
}

// FormatRateLimitHeaders 设置标准限流响应头
//
// 在响应中添加 X-RateLimit-* 头，供客户端了解限流信息
func FormatRateLimitHeaders(rps float64, remaining int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%.0f", rps))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Next()
	}
}
