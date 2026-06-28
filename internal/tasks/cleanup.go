// Package tasks 提供后台任务（清理过期文件、清理未完成上传）
// 与 Python 版 core/tasks.py 保持一致
package tasks

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/middleware"
	"github.com/ischenyu/FileCodeBox-Go/internal/models"
	"github.com/ischenyu/FileCodeBox-Go/internal/storage"
)

// DeleteExpireFiles 定期清理过期文件的后台任务
// 每 600 秒执行一次
func DeleteExpireFiles(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("清理过期文件任务崩溃", "error", r)
		}
	}()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// 刷新配置
		cfg.RefreshFromDB(db)
		middleware.InitRateLimiters(cfg)

		// 清理过期的 IP 记录
		middleware.ErrorLimit.RemoveExpiredIP()
		middleware.UploadLimit.RemoveExpiredIP()

		// 清理本地空目录
		if cfg.GetString("file_storage") == "local" {
			cleanEmptyDirs(filepath.Join(cfg.GetDataRoot(), "share", "data"))
		}

		// 查询过期文件
		var expireData []models.FileCodes
		db.Where("expired_at < ? OR expired_count = ?", time.Now(), 0).Find(&expireData)

		for _, exp := range expireData {
			// 删除存储中的文件
			if err := store.DeleteFile(&exp); err != nil {
				slog.Error("删除过期文件失败", "code", exp.Code, "error", err)
			}
			// 删除数据库记录
			if err := db.Delete(&exp).Error; err != nil {
				slog.Error("删除过期记录失败", "code", exp.Code, "error", err)
			}
		}

		if len(expireData) > 0 {
			slog.Info("已清理过期文件", "count", len(expireData))
		}
	}
}

// CleanIncompleteUploads 定期清理未完成的上传分片
// 每 3600 秒（1小时）执行一次
func CleanIncompleteUploads(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("清理未完成上传任务崩溃", "error", r)
		}
	}()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cfg.RefreshFromDB(db)

		expireHours := 24 // 24小时后清理未完成上传
		expireTime := time.Now().Add(-time.Duration(expireHours) * time.Hour)

		var expiredSessions []models.UploadChunk
		db.Where("chunk_index = ? AND created_at < ?", -1, expireTime).Find(&expiredSessions)

		for _, session := range expiredSessions {
			savePath := ""
			if session.SavePath != nil {
				savePath = *session.SavePath
			}

			// 清理分片文件
			if savePath != "" {
				if err := store.CleanChunks(session.UploadID, savePath); err != nil {
					slog.Error("清理分片文件失败", "upload_id", session.UploadID, "error", err)
				}
			}

			// 删除数据库记录
			db.Where("upload_id = ?", session.UploadID).Delete(&models.UploadChunk{})
			slog.Info("已清理过期上传会话", "upload_id", session.UploadID)
		}
	}
}

// cleanEmptyDirs 递归删除空的本地目录
func cleanEmptyDirs(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}

		entries, err := os.ReadDir(path)
		if err != nil || len(entries) > 0 {
			return nil
		}

		os.Remove(path)
		return nil
	})
}
