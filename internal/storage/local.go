// Package storage 本地文件系统存储实现
// 与 Python 版 core/storage.py 的 SystemFileStorage 保持一致
package storage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/ischenyu/internal/models"
	"github.com/ischenyu/internal/utils"
)

// LocalStorage 本地文件系统存储
type LocalStorage struct {
	rootPath  string
	chunkSize int // 读写缓冲区大小
}

// NewLocalStorage 创建本地存储实例
func NewLocalStorage(rootPath string) *LocalStorage {
	return &LocalStorage{
		rootPath:  rootPath,
		chunkSize: 256 * 1024, // 256KB
	}
}

// SaveFile 保存上传文件到本地文件系统
func (s *LocalStorage) SaveFile(file *multipart.FileHeader, savePath string) error {
	// 打开上传文件
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	// 解析路径
	dir := filepath.Dir(savePath)
	filename := utils.SanitizeFilename(filepath.Base(savePath))
	fullPath := filepath.Join(s.rootPath, dir, filename)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建目标文件
	dst, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	// 分块写入
	buf := make([]byte, s.chunkSize)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取上传文件失败: %w", err)
		}
	}
	return nil
}

// DeleteFile 从本地文件系统删除文件
func (s *LocalStorage) DeleteFile(fileCode *models.FileCodes) error {
	fullPath := filepath.Join(s.rootPath, fileCode.GetFilePath())
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil // 文件已不存在，视为成功
	}
	return os.Remove(fullPath)
}

// GetFileURL 获取服务器中转下载URL
func (s *LocalStorage) GetFileURL(fileCode *models.FileCodes) string {
	return utils.GetFileURL(fileCode.Code, "") // 后续需要传入 jwt_secret
}

// GetFileResponse 获取本地文件路径
func (s *LocalStorage) GetFileResponse(fileCode *models.FileCodes) (string, error) {
	fullPath := filepath.Join(s.rootPath, fileCode.GetFilePath())
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("文件已过期删除")
	}
	return fullPath, nil
}

// SaveChunk 保存分片到本地文件系统
func (s *LocalStorage) SaveChunk(uploadID string, chunkIndex int, chunkData []byte, chunkHash string, savePath string) error {
	chunkDir := filepath.Join(s.rootPath, filepath.Dir(savePath), "chunks", uploadID)
	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d.part", chunkIndex))

	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("创建分片目录失败: %w", err)
	}

	// 先写入临时文件，再原子重命名
	tmpPath := chunkPath + ".tmp"
	if err := os.WriteFile(tmpPath, chunkData, 0644); err != nil {
		return fmt.Errorf("写入分片失败: %w", err)
	}
	if err := os.Rename(tmpPath, chunkPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名分片失败: %w", err)
	}
	return nil
}

// MergeChunks 合并本地分片文件
func (s *LocalStorage) MergeChunks(uploadID string, chunkInfo *models.UploadChunk, savePath string) (string, string, error) {
	fullPath := filepath.Join(s.rootPath, savePath)
	chunkBaseDir := filepath.Join(filepath.Dir(fullPath), "chunks", uploadID)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	fileHasher := sha256.New()
	tmpPath := fullPath + ".merging"

	outFile, err := os.Create(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("创建合并文件失败: %w", err)
	}
	defer outFile.Close()

	for i := 0; i < chunkInfo.TotalChunks; i++ {
		chunkPath := filepath.Join(chunkBaseDir, fmt.Sprintf("%d.part", i))
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			outFile.Close()
			os.Remove(tmpPath)
			return "", "", fmt.Errorf("读取分片%d失败: %w", i, err)
		}

		// 验证哈希
		chunkHasher := sha256.New()
		chunkHasher.Write(chunkData)
		_ = fmt.Sprintf("%x", chunkHasher.Sum(nil))
		// 实际的哈希验证需要在调用方传入每片哈希来对比

		fileHasher.Write(chunkData)
		if _, err := outFile.Write(chunkData); err != nil {
			outFile.Close()
			os.Remove(tmpPath)
			return "", "", fmt.Errorf("写入分片%d失败: %w", i, err)
		}
	}
	outFile.Close()

	// 原子重命名
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("重命名合并文件失败: %w", err)
	}

	fileHash := fmt.Sprintf("%x", fileHasher.Sum(nil))
	return fullPath, fileHash, nil
}

// GeneratePresignedUploadURL 本地存储不支持直传
func (s *LocalStorage) GeneratePresignedUploadURL(savePath string, expiresIn int) (string, error) {
	return "", nil // 不支持
}

// FileExists 检查文件是否存在
func (s *LocalStorage) FileExists(savePath string) (bool, error) {
	fullPath := filepath.Join(s.rootPath, savePath)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CleanChunks 清理本地临时分片
func (s *LocalStorage) CleanChunks(uploadID string, savePath string) error {
	chunkDir := filepath.Join(s.rootPath, filepath.Dir(savePath), "chunks", uploadID)
	if err := os.RemoveAll(chunkDir); err != nil {
		slog.Info("清理本地分片目录失败", "dir", chunkDir, "error", err)
		return nil // 非致命错误
	}

	// 清理空的父级 chunks 目录
	chunksParent := filepath.Dir(chunkDir)
	if entries, err := os.ReadDir(chunksParent); err == nil && len(entries) == 0 {
		os.Remove(chunksParent)
	}
	return nil
}
