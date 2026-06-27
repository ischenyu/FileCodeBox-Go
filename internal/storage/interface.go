// Package storage 定义文件存储接口和所有后端实现
// 与 Python 版 core/storage.py 保持一致
package storage

import (
	"mime/multipart"

	"github.com/ischenyu/internal/models"
)

// FileStorageInterface 文件存储接口
// 所有存储后端必须实现此接口
type FileStorageInterface interface {
	// SaveFile 保存文件到存储后端
	// file: 上传文件 multipart header
	// savePath: 目标保存路径
	SaveFile(file *multipart.FileHeader, savePath string) error

	// DeleteFile 从存储后端删除文件
	// fileCode: 文件记录，用于确定文件路径
	DeleteFile(fileCode *models.FileCodes) error

	// GetFileURL 获取文件分享 URL（中转下载地址）
	// 对于不支持直接访问的存储，返回服务器中转 URL
	GetFileURL(fileCode *models.FileCodes) string

	// GetFileResponse 获取文件响应（文件路径/流）
	// 返回本地文件路径用于直接响应，或空字符串表示需要流式传输
	GetFileResponse(fileCode *models.FileCodes) (string, error)

	// SaveChunk 保存分片到存储后端
	// uploadID: 上传会话ID
	// chunkIndex: 分片索引
	// chunkData: 分片数据
	// chunkHash: 分片哈希
	// savePath: 目标保存路径
	SaveChunk(uploadID string, chunkIndex int, chunkData []byte, chunkHash string, savePath string) error

	// MergeChunks 合并所有分片
	// uploadID: 上传会话ID
	// chunkInfo: 分片会话元信息
	// savePath: 目标保存路径
	// 返回: (保存路径, 文件完整哈希, error)
	MergeChunks(uploadID string, chunkInfo *models.UploadChunk, savePath string) (string, string, error)

	// GeneratePresignedUploadURL 生成预签名上传 URL
	// 仅 S3 等支持直传的后端实现，其他返回空字符串
	// savePath: 目标保存路径
	// expiresIn: 过期时间（秒）
	GeneratePresignedUploadURL(savePath string, expiresIn int) (string, error)

	// FileExists 检查文件是否存在于存储后端
	FileExists(savePath string) (bool, error)

	// CleanChunks 清理临时分片文件
	CleanChunks(uploadID string, savePath string) error
}
