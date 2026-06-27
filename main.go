// FileCodeBox - 文件快递柜
// 匿名口令分享文本和文件，无需注册，输入口令即可获取
//
// 基于 Python FastAPI 版重写为 Go + Gin + GORM
// 原项目: https://github.com/vastsa/FileCodeBox
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/database"
	"github.com/ischenyu/internal/middleware"
	"github.com/ischenyu/internal/server"
	"github.com/ischenyu/internal/storage"
	"github.com/ischenyu/internal/tasks"
	"github.com/ischenyu/internal/utils"
)

func main() {
	// 日志配置
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("正在初始化 FileCodeBox...", "version", utils.AppVersion)

	// 配置初始化
	config.Initialize()

	// 从环境变量读取基础配置
	host := getEnv("HOST", "0.0.0.0")
	port := getEnv("PORT", "12345")
	logLevel := getEnv("LOG_LEVEL", "info")

	// 设置日志级别
	switch logLevel {
	case "debug":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	case "warn", "warning":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})))
	case "error":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	}

	cfg := config.GlobSettings

	// 数据库初始化
	dataRoot := cfg.GetDataRoot()
	if err := os.MkdirAll(dataRoot, 0755); err != nil {
		slog.Error("创建数据目录失败", "path", dataRoot, "error", err)
		os.Exit(1)
	}

	// 初始化全局锁
	database.StartupLock = database.NewGlobalLock(dataRoot)

	// 获取或生成 JWT secret（启动时初始化）

	// 初始化数据库
	if err := database.InitDB(cfg); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	db := database.DB

	// 加载配置
	database.StartupLock.Lock()
	defer database.StartupLock.Unlock()

	// 确保 settings 行存在
	if err := database.EnsureSettingsRow(db, config.DefaultConfig); err != nil {
		slog.Error("初始化配置失败", "error", err)
		os.Exit(1)
	}

	// 记录系统启动时间
	db.Exec("UPDATE key_value SET value = json(?) WHERE key = ?",
		fmt.Sprintf("%d", time.Now().UnixMilli()), "sys_start")
	var count int64
	db.Table("key_value").Where("key = ?", "sys_start").Count(&count)
	if count == 0 {
		db.Exec("INSERT OR IGNORE INTO key_value (key, value) VALUES (?, json(?))",
			"sys_start", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}

	// 加载配置到运行时
	if err := config.LoadFromDB(db); err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	// 初始化限流器
	middleware.InitRateLimiters(cfg)

	// 初始化存储后端
	store, err := storage.NewStorage(cfg)
	if err != nil {
		slog.Error("初始化存储后端失败", "error", err)
		os.Exit(1)
	}

	// 配置路由
	router := server.Setup(db, cfg, store)

	// 启动后台任务
	go tasks.DeleteExpireFiles(db, cfg, store)
	go tasks.CleanIncompleteUploads(db, cfg, store)

	// 启动 HTTP 服务
	addr := fmt.Sprintf("%s:%s", host, port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("FileCodeBox 启动成功", "addr", addr, "version", utils.AppVersion, "storage", cfg.GetString("file_storage"))
	slog.Info(fmt.Sprintf("请访问 http://localhost:%s 使用", port))

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}

// getEnv 获取环境变量值，不存在时返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
