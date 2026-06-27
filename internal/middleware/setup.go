// Package middleware 提供系统初始化状态拦截中间件
// 与 Python 版 main.py 的 refresh_settings_middleware 保持一致
// 当系统未初始化时，拦截所有请求并引导到 /setup
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/utils"
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

		// 允许初始化相关和健康检查请求通过
		path := c.Request.URL.Path
		if isSetupPath(path) || path == "/health" {
			c.Next()
			return
		}

		// 如果是浏览器请求 HTML，返回初始化页面
		if wantsHTMLResponse(c) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildSetupPageHTML(cfg)))
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

// buildSetupPageHTML 构建系统初始化引导页面（简化版）
// 完整版在 handlers/setup.go 中
func buildSetupPageHTML(cfg *config.Settings) string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>系统未初始化 - FileCodeBox</title>
  <style>
    body { margin:0; min-height:100vh; display:grid; place-items:center; padding:24px;
           font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
           background:#f5f5f7; color:#18181b; }
    main { width:min(100%,480px); padding:32px; border-radius:16px;
           background:#fff; text-align:center; box-shadow:0 16px 48px rgba(0,0,0,.08); }
    h1 { margin:0 0 12px; font-size:24px; }
    p { margin:0 0 24px; color:#71717a; line-height:1.65; }
    a { display:inline-flex; align-items:center; justify-content:center;
        min-height:44px; padding:0 24px; border-radius:10px;
        background:#18181b; color:#fff; text-decoration:none; font-weight:650; }
  </style>
</head>
<body>
  <main>
    <h1>系统尚未初始化</h1>
    <p>首次使用需要配置管理员密码，请点击下方按钮进入初始化向导。</p>
    <a href="/setup">进入初始化向导</a>
  </main>
</body>
</html>`
}
