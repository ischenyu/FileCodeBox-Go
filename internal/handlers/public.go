// Package handlers 公共处理器（首页、健康检查、robots.txt、公共配置、静态资源）
// 与 Python 版 main.py 的 / /health /robots.txt /assets /api/v1/config 保持一致
package handlers

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// PublicHandler 公共处理器
type PublicHandler struct {
	Cfg            *config.Settings
	embeddedAssets *embed.FS // 嵌入的 Vue 静态资源（可选）
}

// NewPublicHandler 创建公共处理器
// embeddedAssets 可选：嵌入的 Vue 编译后静态资源目录
func NewPublicHandler(cfg *config.Settings, embeddedAssets *embed.FS) *PublicHandler {
	return &PublicHandler{Cfg: cfg, embeddedAssets: embeddedAssets}
}

// Index 首页
// GET /
func (h *PublicHandler) Index(c *gin.Context) {
	// 优先使用嵌入的 Vue 编译后 index.html
	if h.embeddedAssets != nil {
		data, err := fs.ReadFile(h.embeddedAssets, "assets/index.html")
		if err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			return
		}
	}

	// 回退到主题目录的 index.html
	themeRoot := h.Cfg.GetString("themesSelect")
	indexPath := themeRoot + "/index.html"
	c.File(indexPath)
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
// 优先返回嵌入的 Vue 编译静态文件，找不到再回退到主题目录
func (h *PublicHandler) ThemeAsset(c *gin.Context) {
	assetPath := c.Param("asset_path")
	// Gin *param 捕获包含前导 /，需去除
	assetPath = strings.TrimPrefix(assetPath, "/")

	// 1. 尝试从嵌入的 assets 目录加载（先查 assets/assets/ 即 Vue 编译输出，再查 assets/）
	if h.embeddedAssets != nil {
		for _, prefix := range []string{"assets/assets/", "assets/"} {
			data, err := fs.ReadFile(h.embeddedAssets, prefix+assetPath)
			if err == nil {
				contentType := mime.TypeByExtension(filepath.Ext(assetPath))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				c.Data(http.StatusOK, contentType, data)
				return
			}
		}
	}

	// 2. 回退到主题目录
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


