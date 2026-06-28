// Package server 配置 Gin 路由引擎，注册所有路由和中间件
// 与 Python 版 main.py 的 FastAPI 路由注册保持一致
package server

import (
	"embed"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/handlers"
	"github.com/ischenyu/FileCodeBox-Go/internal/middleware"
	"github.com/ischenyu/FileCodeBox-Go/internal/storage"
)

// Setup 配置 Gin 路由引擎
// embeddedAssets 可选：嵌入的 Vue 编译后静态资源
func Setup(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface, embeddedAssets *embed.FS) *gin.Engine {
	// 生产模式
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// 全局中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(middleware.SetupGuard(cfg))

	// 创建处理器
	setupH := handlers.NewSetupHandler(db, cfg, embeddedAssets)
	shareH := handlers.NewShareHandler(db, cfg, store)
	chunkH := handlers.NewChunkHandler(db, cfg, store)
	presignH := handlers.NewPresignHandler(db, cfg, store)
	adminH := handlers.NewAdminHandler(db, cfg)
	publicH := handlers.NewPublicHandler(cfg, embeddedAssets)

	// 公开路由

	// 首页
	r.GET("/", publicH.Index)
	r.POST("/", publicH.GetPublicConfig)

	// 配置 API
	r.GET("/api/v1/config", publicH.GetPublicConfigV1)
	r.GET("/health", publicH.HealthCheck)
	r.GET("/robots.txt", publicH.Robots)

	// 主题静态资源
	r.GET("/assets/*asset_path", publicH.ThemeAsset)

	// 初始化向导
	r.GET("/setup", setupH.GetSetupPage)
	r.GET("/setup/", setupH.GetSetupPage)
	r.POST("/setup", setupH.PostSetup)
	r.POST("/setup/", setupH.PostSetup)

	// 分享路由（需要上传权限检查）
	shareGroup := r.Group("/share")
	shareGroup.Use(middleware.ShareRequiredLogin(cfg))
	{
		shareGroup.POST("/text/", limitUpload(shareH.ShareText))
		shareGroup.POST("/file/", limitUpload(shareH.ShareFile))
		shareGroup.GET("/metadata/", limitError(shareH.GetFileMetadata))
		shareGroup.POST("/metadata/", limitError(shareH.PostFileMetadata))
		shareGroup.GET("/select/", limitError(shareH.GetCodeFile))
		shareGroup.POST("/select/", limitError(shareH.SelectFile))
		shareGroup.GET("/download", limitError(shareH.DownloadFile))
	}

	// 切片上传路由
	chunkGroup := r.Group("/chunk")
	chunkGroup.Use(middleware.ShareRequiredLogin(cfg))
	{
		chunkGroup.POST("/upload/init/", chunkH.InitUpload)
		chunkGroup.PUT("/upload/:upload_id/:chunk_index", chunkH.UploadChunk)
		chunkGroup.DELETE("/upload/:upload_id", chunkH.CancelUpload)
		chunkGroup.GET("/upload/status/:upload_id", chunkH.GetUploadStatus)
		chunkGroup.POST("/upload/complete/:upload_id", limitUpload(chunkH.CompleteUpload))
	}

	// 预签名上传路由
	presignGroup := r.Group("/presign")
	presignGroup.Use(middleware.ShareRequiredLogin(cfg))
	{
		presignGroup.POST("/upload/init", limitUpload(presignH.InitUpload))
		presignGroup.PUT("/upload/proxy/:upload_id", limitUpload(presignH.ProxyUpload))
		presignGroup.POST("/upload/confirm/:upload_id", limitUpload(presignH.ConfirmUpload))
		presignGroup.GET("/upload/status/:upload_id", presignH.GetUploadStatus)
		presignGroup.DELETE("/upload/:upload_id", presignH.CancelUpload)
	}

	// /api 前缀的预签名路由（向后兼容）
	apiPresign := r.Group("/api/presign")
	apiPresign.Use(middleware.ShareRequiredLogin(cfg))
	{
		apiPresign.POST("/upload/init", limitUpload(presignH.InitUpload))
		apiPresign.PUT("/upload/proxy/:upload_id", limitUpload(presignH.ProxyUpload))
		apiPresign.POST("/upload/confirm/:upload_id", limitUpload(presignH.ConfirmUpload))
	}

	// 管理后台路由（需要管理员认证）
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminRequired(cfg))
	{
		adminGroup.POST("/login", adminH.Login)
		adminGroup.GET("/verify", adminH.Verify)
		adminGroup.POST("/logout", adminH.Logout)
		adminGroup.GET("/dashboard", adminH.Dashboard)
		adminGroup.GET("/file/list", adminH.FileList)
		adminGroup.DELETE("/file/delete", adminH.FileDelete)
		adminGroup.DELETE("/file/batch-delete", adminH.FileDelete)
		adminGroup.POST("/file/batch-delete", adminH.FileDelete)
		adminGroup.GET("/config/get", adminH.ConfigGet)
		adminGroup.PATCH("/config/update", adminH.ConfigUpdate)
	}

	return r
}

// limitUpload 上传频率限制包装
func limitUpload(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.RateLimitUpload()(c)
		if c.IsAborted() {
			return
		}
		h(c)
	}
}

// limitError 取件错误频率限制包装
func limitError(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.RateLimitError()(c)
		if c.IsAborted() {
			return
		}
		h(c)
	}
}

// corsMiddleware CORS 中间件（允许所有来源）
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
