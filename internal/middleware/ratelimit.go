// Package middleware 提供 IP 频率限制中间件
// 与 Python 版 apps/base/dependencies.py 的 IPRateLimit 保持一致
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/utils"
)

// IPRateLimit IP 频率限制器
type IPRateLimit struct {
	mu      sync.Mutex
	ips     map[string]*ipRecord
	count   int
	minutes int
}

type ipRecord struct {
	count int
	time  time.Time
}

// NewIPRateLimit 创建 IP 频率限制器
func NewIPRateLimit(count, minutes int) *IPRateLimit {
	return &IPRateLimit{
		ips:     make(map[string]*ipRecord),
		count:   count,
		minutes: minutes,
	}
}

// CheckIP 检查 IP 是否超过限制
func (r *IPRateLimit) CheckIP(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.ips[ip]
	if exists {
		if record.count >= r.count {
			if record.time.Add(time.Duration(r.minutes) * time.Minute).After(time.Now()) {
				return false // 超限
			}
			delete(r.ips, ip) // 过期，移除记录
		}
	}
	return true
}

// AddIP 记录一次 IP 访问并返回当前计数
func (r *IPRateLimit) AddIP(ip string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	record := r.ips[ip]
	if record == nil {
		record = &ipRecord{count: 0, time: time.Now()}
		r.ips[ip] = record
	}
	record.count++
	record.time = time.Now()
	return record.count
}

// RemoveExpiredIP 清理过期的 IP 记录
func (r *IPRateLimit) RemoveExpiredIP() {
	r.mu.Lock()
	defer r.mu.Unlock()

	expireTime := time.Now().Add(-time.Duration(r.minutes) * time.Minute)
	for ip, record := range r.ips {
		if record.time.Before(expireTime) {
			delete(r.ips, ip)
		}
	}
}

// 全局限流器实例
var (
	ErrorLimit  *IPRateLimit // 取件错误限流
	UploadLimit *IPRateLimit // 上传限流
)

// InitRateLimiters 初始化限流器
func InitRateLimiters(cfg *config.Settings) {
	ErrorLimit = NewIPRateLimit(cfg.GetInt("errorCount"), cfg.GetInt("errorMinute"))
	UploadLimit = NewIPRateLimit(cfg.GetInt("uploadCount"), cfg.GetInt("uploadMinute"))
}

// RateLimitError 取件错误频率限制中间件
func RateLimitError() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getClientIP(c)
		if ErrorLimit != nil && !ErrorLimit.CheckIP(ip) {
			c.AbortWithStatusJSON(http.StatusLocked, utils.Error(423, "请求次数过多，请稍后再试"))
			return
		}
		c.Set("client_ip", ip)
		c.Next()
	}
}

// RateLimitUpload 上传频率限制中间件
func RateLimitUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getClientIP(c)
		if UploadLimit != nil && !UploadLimit.CheckIP(ip) {
			c.AbortWithStatusJSON(http.StatusLocked, utils.Error(423, "请求次数过多，请稍后再试"))
			return
		}
		c.Set("client_ip", ip)
		c.Next()
	}
}

// getClientIP 获取客户端真实 IP
// 优先从 X-Forwarded-For / X-Real-IP 获取（反向代理支持）
func getClientIP(c *gin.Context) string {
	// 优先 X-Forwarded-For
	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		// 取最左边的非信任代理 IP
		parts := splitAndTrim(forwardedFor, ",")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	// 其次 X-Real-IP
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 最后用直连 IP
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, p := range stringsSplit(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func stringsSplit(s, sep string) []string {
	var result []string
	for {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
