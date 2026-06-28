// Package middleware 提供系统初始化状态拦截中间件
// 与 Python 版 main.py 的 refresh_settings_middleware 保持一致
// 当系统未初始化时，拦截所有请求并引导到 /setup
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// SetupGuard 系统初始化拦截中间件
// 如果系统尚未初始化且请求不是 /setup 或 /health，返回 428 状态码引导初始化
func SetupGuard(cfg *config.Settings) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 刷新配置
		// cfg.RefreshFromDB() 已在上层调用

		// 检查系统是否已初始化
		if IsConfigInitialized(cfg) {
			c.Next()
			return
		}

		// 允许首页、静态资源、初始化相关和健康检查请求通过
		path := c.Request.URL.Path
		if path == "/" || isAssetPath(path) || isSetupPath(path) || path == "/health" {
			c.Next()
			return
		}

		// 如果是浏览器请求 HTML，重定向到首页（Vue SPA 处理初始化引导）
		if wantsHTMLResponse(c) {
			c.Redirect(http.StatusSeeOther, "/")
			c.Abort()
			return
		}

		// API 请求返回 428
		c.AbortWithStatusJSON(http.StatusPreconditionRequired, utils.ErrorDetail(428, "系统未初始化，请先完成初始化", gin.H{
			"setup": "/setup",
		}))
	}
}

// IsConfigInitialized 检查系统配置是否已完成初始化
// 与 Python 版 core/config.py 的 is_runtime_initialized 保持一致
func IsConfigInitialized(cfg *config.Settings) bool {
	adminToken := cfg.GetString("admin_token")
	if adminToken == "" {
		return false
	}
	// 检查是否还是旧版默认密码
	legacyDefault := "FileCodeBox2023"
	return !utils.VerifyPassword(legacyDefault, adminToken)
}

// isAssetPath 判断是否为静态资源路径
func isAssetPath(path string) bool {
	return strings.HasPrefix(path, "/assets/")
}

// isSetupPath 判断是否为 /setup 路径
func isSetupPath(path string) bool {
	return strings.TrimRight(path, "/") == "/setup"
}

// wantsHTMLResponse 判断请求是否需要 HTML 响应（浏览器直接访问）
func wantsHTMLResponse(c *gin.Context) bool {
	if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
		return false
	}
	accept := c.GetHeader("Accept")
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}


