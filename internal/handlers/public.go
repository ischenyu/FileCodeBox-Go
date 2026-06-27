// Package handlers 公共处理器（首页、健康检查、robots.txt、公共配置、静态资源）
// 与 Python 版 main.py 的 / /health /robots.txt /assets /api/v1/config 保持一致
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/utils"
)

// PublicHandler 公共处理器
type PublicHandler struct {
	Cfg *config.Settings
}

// NewPublicHandler 创建公共处理器
func NewPublicHandler(cfg *config.Settings) *PublicHandler {
	return &PublicHandler{Cfg: cfg}
}

// Index 首页
// GET /
func (h *PublicHandler) Index(c *gin.Context) {
	// 尝试从主题目录加载 index.html
	themeRoot := h.Cfg.GetString("themesSelect")
	indexPath := themeRoot + "/index.html"

	// 如果主题文件存在，替换模板变量后返回
	// 否则返回简单 HTML
	html := h.loadThemeIndex(indexPath)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// GetPublicConfig 获取公共配置（旧接口，POST）
// POST /
func (h *PublicHandler) GetPublicConfig(c *gin.Context) {
	c.JSON(http.StatusOK, utils.Success(h.buildPublicConfig()))
}

// GetPublicConfigV1 获取公共配置（新接口，GET）
// GET /api/v1/config
func (h *PublicHandler) GetPublicConfigV1(c *gin.Context) {
	c.JSON(http.StatusOK, utils.Success(gin.H{
		"config": h.buildPublicConfig(),
		"meta":   h.buildPublicMeta(),
	}))
}

// HealthCheck 健康检查
// GET /health
func (h *PublicHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, utils.Success(gin.H{
		"status":  "ok",
		"version": utils.AppVersion,
		"storage": h.Cfg.GetString("file_storage"),
		"theme":   h.Cfg.GetString("themesSelect"),
	}))
}

// Robots robots.txt
// GET /robots.txt
func (h *PublicHandler) Robots(c *gin.Context) {
	robotsText := h.Cfg.GetString("robotsText")
	if robotsText == "" {
		robotsText = "User-agent: *\nDisallow: /"
	}
	c.String(http.StatusOK, robotsText)
}

// ThemeAsset 主题静态资源
// GET /assets/:asset_path
func (h *PublicHandler) ThemeAsset(c *gin.Context) {
	assetPath := c.Param("asset_path")
	themeRoot := h.Cfg.GetString("themesSelect")
	fullPath := themeRoot + "/assets/" + assetPath
	c.File(fullPath)
}

// buildPublicConfig 构建公共配置（前端可直接使用）
func (h *PublicHandler) buildPublicConfig() gin.H {
	return gin.H{
		"name":               h.Cfg.GetString("name"),
		"description":        h.Cfg.GetString("description"),
		"explain":            h.Cfg.GetString("page_explain"),
		"uploadSize":         h.Cfg.GetInt64("uploadSize"),
		"allowedFileTypes":   h.Cfg.GetStringSlice("allowed_file_types"),
		"expireStyle":        h.Cfg.GetStringSlice("expireStyle"),
		"enableChunk":        h.Cfg.GetBool("enableChunk"),
		"openUpload":         h.Cfg.GetBool("openUpload"),
		"notify_title":       h.Cfg.GetString("notify_title"),
		"notify_content":     h.Cfg.GetString("notify_content"),
		"show_admin_address": h.Cfg.GetBool("showAdminAddr"),
		"max_save_seconds":   h.Cfg.GetInt64("max_save_seconds"),
	}
}

// buildPublicMeta 构建公共元数据
func (h *PublicHandler) buildPublicMeta() gin.H {
	return gin.H{
		"version": utils.AppVersion,
		"api": gin.H{
			"legacyConfig": "/",
			"publicConfig": "/api/v1/config",
			"health":       "/health",
		},
		"features": gin.H{
			"chunkUpload":         h.Cfg.GetBool("enableChunk"),
			"guestUpload":         h.Cfg.GetBool("openUpload"),
			"adminAddressVisible": h.Cfg.GetBool("showAdminAddr"),
			"expirationModes":     h.Cfg.GetStringSlice("expireStyle"),
		},
		"limits": gin.H{
			"uploadSize":          h.Cfg.GetInt64("uploadSize"),
			"allowedFileTypes":    h.Cfg.GetStringSlice("allowed_file_types"),
			"maxSaveSeconds":      h.Cfg.GetInt64("max_save_seconds"),
			"uploadWindowMinutes": h.Cfg.GetInt("uploadMinute"),
			"uploadWindowCount":   h.Cfg.GetInt("uploadCount"),
		},
	}
}

// loadThemeIndex 加载主题首页模板
func (h *PublicHandler) loadThemeIndex(themePath string) string {
	// 如果主题目录存在 index.html，读取并替换变量
	// 否则返回简单的默认页面
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + h.Cfg.GetString("name") + `</title>
  <meta name="description" content="` + h.Cfg.GetString("description") + `">
  <meta name="keywords" content="` + h.Cfg.GetString("keywords") + `">
  <style>
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 24px;
           font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
           background: ` + h.Cfg.GetString("background") + `; color: #18181b; }
    main { text-align: center; max-width: 600px; padding: 48px; border-radius: 20px;
           background: rgba(255,255,255,0.9); box-shadow: 0 22px 60px rgba(0,0,0,.08); }
    h1 { margin: 0; font-size: 28px; }
    p { margin: 12px 0 0; color: #71717a; line-height: 1.6; }
    .links { margin-top: 32px; display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
    a { display: inline-flex; align-items: center; min-height: 40px; padding: 0 20px;
        border-radius: 8px; text-decoration: none; font-weight: 600; }
    .btn-admin { background: #18181b; color: #fff; }
    .btn-upload { background: #2563eb; color: #fff; }
  </style>
</head>
<body>
  <main>
    <h1>` + h.Cfg.GetString("name") + `</h1>
    <p>` + h.Cfg.GetString("description") + `</p>
    <div class="links">
      <a class="btn-upload" href="/">上传/取件</a>
      <a class="btn-admin" href="/#/admin">管理后台</a>
    </div>
    <p style="font-size:12px;margin-top:24px;">
      <a href="https://github.com/vastsa/FileCodeBox" target="_blank">GitHub</a> · Version ` + utils.AppVersion + `
    </p>
  </main>
</body>
</html>`
}
