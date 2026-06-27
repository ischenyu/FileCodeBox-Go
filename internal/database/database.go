// Package database 提供 SQLite 数据库初始化与迁移
// 与 Python 版 core/database.py 保持一致
package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/models"
)

var (
	// DB 全局数据库实例
	DB         *gorm.DB
	dbInitOnce sync.Once
)

// GlobalLock 全局启/初始化排他锁（用于多进程串行化启动写操作）
type GlobalLock struct {
	mu       sync.Mutex
	lockFile *os.File
	dataRoot string
}

var (
	// StartupLock 全局启动锁实例
	StartupLock *GlobalLock
)

// NewGlobalLock 创建基于文件锁的全局锁
func NewGlobalLock(dataRoot string) *GlobalLock {
	return &GlobalLock{dataRoot: dataRoot}
}

// Lock 获取文件锁
func (l *GlobalLock) Lock() error {
	l.mu.Lock()
	os.MkdirAll(l.dataRoot, 0755)
	lockPath := filepath.Join(l.dataRoot, "filecodebox.startup.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		l.mu.Unlock()
		return fmt.Errorf("打开锁文件失败: %w", err)
	}
	// Windows 上简单的文件存在即锁；Unix 上使用 syscall.Flock
	l.lockFile = f
	return nil
}

// Unlock 释放文件锁
func (l *GlobalLock) Unlock() {
	if l.lockFile != nil {
		l.lockFile.Close()
		l.lockFile = nil
	}
	l.mu.Unlock()
}

// InitDB 初始化数据库连接并执行迁移
func InitDB(cfg *config.Settings) error {
	var initErr error
	dbInitOnce.Do(func() {
		dataRoot := cfg.GetDataRoot()
		if err := os.MkdirAll(dataRoot, 0755); err != nil {
			initErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join(dataRoot, "filecodebox.db")
		slog.Info("初始化数据库", "path", dbPath)

		// 打开 SQLite 连接，启用 WAL 模式和外键
		db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			initErr = fmt.Errorf("连接数据库失败: %w", err)
			return
		}

		// 设置连接池
		sqlDB, _ := db.DB()
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(time.Hour)

		DB = db

		// 自动迁移
		if err := Migrate(db); err != nil {
			initErr = fmt.Errorf("迁移失败: %w", err)
			return
		}

		slog.Info("数据库初始化完成")
	})
	return initErr
}

// Migrate 执行数据库迁移
// 先创建 migrates 表跟踪迁移记录，再自动迁移模型
func Migrate(db *gorm.DB) error {
	// 创建迁移记录表
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			migration_file VARCHAR(255) NOT NULL UNIQUE,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return fmt.Errorf("创建 migrates 表失败: %w", err)
	}

	// GORM AutoMigrate 自动创建/更新表结构
	if err := db.AutoMigrate(
		&models.FileCodes{},
		&models.UploadChunk{},
		&models.KeyValue{},
		&models.PresignUploadSession{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	// 记录迁移
	var count int64
	db.Raw("SELECT COUNT(*) FROM migrates WHERE migration_file = ?", "gorm_auto_migrate").Scan(&count)
	if count == 0 {
		db.Exec("INSERT INTO migrates (migration_file) VALUES (?)", "gorm_auto_migrate")
	}

	return nil
}

// EnsureSettingsRow 确保 settings 配置行存在
// 与 Python 版 core/config.py 的 ensure_settings_row 类似
func EnsureSettingsRow(db *gorm.DB, defaultConfig map[string]interface{}) error {
	securityConfig := prepareSecurityConfig(defaultConfig)

	configJSON, err := json.Marshal(securityConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	var existing models.KeyValue
	result := db.Where("key = ?", "settings").First(&existing)
	if result.Error == gorm.ErrRecordNotFound {
		kv := models.KeyValue{
			Key: "settings",
			Value: sql.NullString{
				String: string(configJSON),
				Valid:  true,
			},
		}
		if err := db.Create(&kv).Error; err != nil {
			return fmt.Errorf("创建 settings 行失败: %w", err)
		}
		slog.Warn("系统尚未初始化，请在浏览器中打开站点并完成管理员密码设置")
	}
	return nil
}

func prepareSecurityConfig(cfg map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range cfg {
		result[k] = v
	}
	// 确保 jwt_secret 存在
	if _, ok := result["jwt_secret"]; !ok {
		result["jwt_secret"] = ""
	}
	return result
}
