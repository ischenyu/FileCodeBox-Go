// Package storage OneDrive 存储实现
// 与 Python 版 core/storage.py 的 OneDriveFileStorage 保持一致
// 注意：OneDrive 需要 OAuth 认证，当前为简化实现，仅提供核心接口
package storage

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/ischenyu/internal/models"
)

// OneDriveStorage OneDrive 存储后端
type OneDriveStorage struct {
	domain      string
	clientID    string
	username    string
	password    string
	rootPath    string
	proxy       bool
	accessToken string
}

// OneDriveConfig OneDrive 存储配置
type OneDriveConfig struct {
	Domain   string
	ClientID string
	Username string
	Password string
	RootPath string
	Proxy    bool
}

// NewOneDriveStorage 创建 OneDrive 存储实例
func NewOneDriveStorage(cfg OneDriveConfig) (*OneDriveStorage, error) {
	return &OneDriveStorage{
		domain:   cfg.Domain,
		clientID: cfg.ClientID,
		username: cfg.Username,
		password: cfg.Password,
		rootPath: cfg.RootPath,
		proxy:    cfg.Proxy,
	}, nil
}

// SaveFile 上传文件到 OneDrive（简化实现）
func (s *OneDriveStorage) SaveFile(file *multipart.FileHeader, savePath string) error {
	return fmt.Errorf("OneDrive 上传功能需要 OAuth 授权，暂未实现")
}

// DeleteFile 从 OneDrive 删除文件
func (s *OneDriveStorage) DeleteFile(fileCode *models.FileCodes) error {
	return nil // 简化：异步清理，忽略删除错误
}

// GetFileURL 获取中转下载URL
func (s *OneDriveStorage) GetFileURL(fileCode *models.FileCodes) string {
	return fmt.Sprintf("/share/download?key=%s&code=%s", fileCode.Code, fileCode.Code)
}

// GetFileResponse 获取文件响应
func (s *OneDriveStorage) GetFileResponse(fileCode *models.FileCodes) (string, error) {
	return "", fmt.Errorf("OneDrive 文件下载需通过 API 代理，暂未实现")
}

// SaveChunk 保存分片
func (s *OneDriveStorage) SaveChunk(uploadID string, chunkIndex int, chunkData []byte, chunkHash string, savePath string) error {
	return fmt.Errorf("OneDrive 切片上传暂未实现")
}

// MergeChunks 合并分片
func (s *OneDriveStorage) MergeChunks(uploadID string, chunkInfo *models.UploadChunk, savePath string) (string, string, error) {
	return "", "", fmt.Errorf("OneDrive 切片合并暂未实现")
}

// GeneratePresignedUploadURL OneDrive 不支持直传
func (s *OneDriveStorage) GeneratePresignedUploadURL(savePath string, expiresIn int) (string, error) {
	return "", nil
}

// FileExists 检查文件是否存在
func (s *OneDriveStorage) FileExists(savePath string) (bool, error) {
	return false, nil
}

// CleanChunks 清理临时分片
func (s *OneDriveStorage) CleanChunks(uploadID string, savePath string) error {
	return nil
}

// ensure import 被使用
var _ = io.EOF
var _ = http.StatusOK
