// Package handlers 切片上传处理器
// 与 Python 版 apps/base/views.py 的 chunk_api 保持一致
package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/middleware"
	"github.com/ischenyu/FileCodeBox-Go/internal/models"
	"github.com/ischenyu/FileCodeBox-Go/internal/storage"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// ChunkHandler 切片上传处理器
type ChunkHandler struct {
	DB      *gorm.DB
	Cfg     *config.Settings
	Storage storage.FileStorageInterface
}

// NewChunkHandler 创建切片上传处理器
func NewChunkHandler(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface) *ChunkHandler {
	return &ChunkHandler{DB: db, Cfg: cfg, Storage: store}
}

// InitUpload 初始化切片上传
// POST /chunk/upload/init/
func (h *ChunkHandler) InitUpload(c *gin.Context) {
	var body struct {
		FileName  string `json:"file_name" form:"file_name"`
		ChunkSize int    `json:"chunk_size" form:"chunk_size"`
		FileSize  int64  `json:"file_size" form:"file_size"`
		FileHash  string `json:"file_hash" form:"file_hash"`
	}
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请求参数错误"))
		return
	}

	if body.ChunkSize <= 0 {
		body.ChunkSize = 5 * 1024 * 1024
	}

	// 验证文件类型
	if err := validateFileType(h.Cfg, body.FileName, ""); err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 校验文件大小
	totalChunks := (body.FileSize + int64(body.ChunkSize) - 1) / int64(body.ChunkSize)
	maxPossibleSize := totalChunks * int64(body.ChunkSize)
	if maxPossibleSize > h.Cfg.GetInt64("uploadSize") {
		maxMB := float64(h.Cfg.GetInt64("uploadSize")) / (1024 * 1024)
		c.JSON(http.StatusForbidden, utils.Error(403, fmt.Sprintf("文件大小超过限制，最大为 %.2f MB", maxMB)))
		return
	}

	// 断点续传：查找未完成的相同文件上传会话
	var existingSession models.UploadChunk
	err := h.DB.Where("chunk_hash = ? AND chunk_index = ? AND file_size = ? AND file_name = ?",
		body.FileHash, -1, body.FileSize, body.FileName).First(&existingSession).Error
	if err == nil {
		if existingSession.SavePath == nil || *existingSession.SavePath == "" {
			// 无效会话，清理
			h.DB.Where("upload_id = ?", existingSession.UploadID).Delete(&models.UploadChunk{})
		} else {
			// 返回已上传分片列表
			var chunkIndexes []int
			h.DB.Model(&models.UploadChunk{}).
				Where("upload_id = ? AND completed = ?", existingSession.UploadID, true).
				Pluck("chunk_index", &chunkIndexes)

			c.JSON(http.StatusOK, utils.Success(gin.H{
				"existed":         false,
				"upload_id":       existingSession.UploadID,
				"chunk_size":      existingSession.ChunkSize,
				"total_chunks":    existingSession.TotalChunks,
				"uploaded_chunks": chunkIndexes,
			}))
			return
		}
	}

	// 生成新的上传会话
	uploadID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s%d%s", body.FileHash, body.FileSize, body.FileName))))[:16]
	_, _, _, _, savePath := generateFilePath(body.FileName, uploadID)

	// 创建元信息记录 (chunk_index = -1)
	meta := models.UploadChunk{
		UploadID:    uploadID,
		ChunkIndex:  -1,
		ChunkHash:   body.FileHash,
		TotalChunks: int(totalChunks),
		FileSize:    body.FileSize,
		ChunkSize:   body.ChunkSize,
		FileName:    body.FileName,
		SavePath:    &savePath,
		Completed:   false,
	}
	h.DB.Create(&meta)

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"existed":      false,
		"upload_id":    uploadID,
		"chunk_size":   body.ChunkSize,
		"total_chunks": totalChunks,
	}))
}

// UploadChunk 上传分片
// PUT /chunk/upload/:upload_id/:chunk_index
func (h *ChunkHandler) UploadChunk(c *gin.Context) {
	uploadID := c.Param("upload_id")
	chunkIndexStr := c.Param("chunk_index")
	var chunkIndex int
	fmt.Sscanf(chunkIndexStr, "%d", &chunkIndex)

	// 获取上传会话元信息
	var chunkInfo models.UploadChunk
	if err := h.DB.Where("upload_id = ? AND chunk_index = ?", uploadID, -1).First(&chunkInfo).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	// 检查分片索引有效性
	if chunkIndex < 0 || chunkIndex >= chunkInfo.TotalChunks {
		c.JSON(http.StatusBadRequest, utils.Error(400, "无效的分片索引"))
		return
	}

	// 检查是否已上传（断点续传）
	var existingChunk models.UploadChunk
	err := h.DB.Where("upload_id = ? AND chunk_index = ? AND completed = ?",
		uploadID, chunkIndex, true).First(&existingChunk).Error
	if err == nil {
		c.JSON(http.StatusOK, utils.Success(gin.H{
			"chunk_hash": existingChunk.ChunkHash,
			"skipped":    true,
		}))
		return
	}

	// 读取分片数据
	chunkData, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "读取分片数据失败"))
		return
	}
	chunkSize := len(chunkData)

	if chunkSize > chunkInfo.ChunkSize {
		c.JSON(http.StatusBadRequest, utils.Error(400,
			fmt.Sprintf("分片大小超过声明值: 最大 %d, 实际 %d", chunkInfo.ChunkSize, chunkSize)))
		return
	}

	// 校验累计上传大小
	var uploadedCount int64
	h.DB.Model(&models.UploadChunk{}).
		Where("upload_id = ? AND completed = ?", uploadID, true).
		Count(&uploadedCount)
	maxUploadedSize := int64(uploadedCount)*int64(chunkInfo.ChunkSize) + int64(chunkSize)
	if maxUploadedSize > h.Cfg.GetInt64("uploadSize") {
		maxMB := float64(h.Cfg.GetInt64("uploadSize")) / (1024 * 1024)
		c.JSON(http.StatusForbidden, utils.Error(403, fmt.Sprintf("累计上传大小超过限制，最大为 %.2f MB", maxMB)))
		return
	}

	// 计算分片哈希
	chunkHasher := sha256.New()
	chunkHasher.Write(chunkData)
	chunkHash := fmt.Sprintf("%x", chunkHasher.Sum(nil))

	// 保存分片
	savePath := ""
	if chunkInfo.SavePath != nil {
		savePath = *chunkInfo.SavePath
	}
	if err := h.Storage.SaveChunk(uploadID, chunkIndex, chunkData, chunkHash, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, fmt.Sprintf("分片保存失败: %v", err)))
		return
	}

	// 创建或更新分片记录
	chunk := models.UploadChunk{
		UploadID:    uploadID,
		ChunkIndex:  chunkIndex,
		ChunkHash:   chunkHash,
		TotalChunks: chunkInfo.TotalChunks,
		FileSize:    chunkInfo.FileSize,
		ChunkSize:   chunkInfo.ChunkSize,
		FileName:    chunkInfo.FileName,
		SavePath:    chunkInfo.SavePath,
		Completed:   true,
	}
	h.DB.Where("upload_id = ? AND chunk_index = ?", uploadID, chunkIndex).Assign(chunk).FirstOrCreate(&chunk)

	c.JSON(http.StatusOK, utils.Success(gin.H{"chunk_hash": chunkHash}))
}

// CancelUpload 取消上传
// DELETE /chunk/upload/:upload_id
func (h *ChunkHandler) CancelUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	var chunkInfo models.UploadChunk
	if err := h.DB.Where("upload_id = ? AND chunk_index = ?", uploadID, -1).First(&chunkInfo).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	// 清理存储中的临时文件
	if chunkInfo.SavePath != nil && *chunkInfo.SavePath != "" {
		h.Storage.CleanChunks(uploadID, *chunkInfo.SavePath)
	}

	// 清理数据库记录
	h.DB.Where("upload_id = ?", uploadID).Delete(&models.UploadChunk{})

	c.JSON(http.StatusOK, utils.Success(gin.H{"message": "上传已取消"}))
}

// GetUploadStatus 获取上传状态
// GET /chunk/upload/status/:upload_id
func (h *ChunkHandler) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")

	var chunkInfo models.UploadChunk
	if err := h.DB.Where("upload_id = ? AND chunk_index = ?", uploadID, -1).First(&chunkInfo).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	var chunkIndexes []int
	h.DB.Model(&models.UploadChunk{}).
		Where("upload_id = ? AND completed = ?", uploadID, true).
		Pluck("chunk_index", &chunkIndexes)

	progress := float64(len(chunkIndexes)) / float64(chunkInfo.TotalChunks) * 100

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"upload_id":       uploadID,
		"file_name":       chunkInfo.FileName,
		"file_size":       chunkInfo.FileSize,
		"chunk_size":      chunkInfo.ChunkSize,
		"total_chunks":    chunkInfo.TotalChunks,
		"uploaded_chunks": chunkIndexes,
		"progress":        progress,
	}))
}

// CompleteUpload 完成上传并合并分片
// POST /chunk/upload/complete/:upload_id
func (h *ChunkHandler) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	var body struct {
		ExpireValue int    `json:"expire_value" form:"expire_value"`
		ExpireStyle string `json:"expire_style" form:"expire_style"`
	}
	c.ShouldBind(&body)

	// 获取上传会话元信息
	var chunkInfo models.UploadChunk
	if err := h.DB.Where("upload_id = ? AND chunk_index = ?", uploadID, -1).First(&chunkInfo).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	// 验证所有分片
	var completedCount int64
	h.DB.Model(&models.UploadChunk{}).Where("upload_id = ? AND completed = ?", uploadID, true).Count(&completedCount)
	if int(completedCount) != chunkInfo.TotalChunks {
		c.JSON(http.StatusBadRequest, utils.Error(400, "分片不完整"))
		return
	}

	savePath := ""
	if chunkInfo.SavePath != nil {
		savePath = *chunkInfo.SavePath
	}

	// 合并分片
	_, fileHash, err := h.Storage.MergeChunks(uploadID, &chunkInfo, savePath)
	if err != nil {
		// 合并失败，清理
		h.Storage.CleanChunks(uploadID, savePath)
		c.JSON(http.StatusInternalServerError, utils.Error(500, fmt.Sprintf("文件合并失败: %v", err)))
		return
	}

	// 获取过期信息
	expiredAt, expiredCount, usedCount, code, err := getExpireInfo(h.Cfg, body.ExpireValue, body.ExpireStyle)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 获取文件路径
	path := ""
	if chunkInfo.SavePath != nil {
		path = dirOf(*chunkInfo.SavePath)
	}
	prefix, suffix := splitFilename(chunkInfo.FileName)

	// 创建文件记录
	fileCode := models.FileCodes{
		Code:         code,
		FileHash:     &fileHash,
		IsChunked:    true,
		UploadID:     &uploadID,
		Size:         chunkInfo.FileSize,
		ExpiredAt:    expiredAt,
		ExpiredCount: expiredCount,
		UsedCount:    usedCount,
		FilePath:     &path,
		UUIDFileName: &chunkInfo.FileName,
		Prefix:       prefix,
		Suffix:       suffix,
	}
	h.DB.Create(&fileCode)

	// 清理临时分片
	h.Storage.CleanChunks(uploadID, savePath)
	h.DB.Where("upload_id = ?", uploadID).Delete(&models.UploadChunk{})

	// 记录 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{"code": code, "name": chunkInfo.FileName}))
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return ""
}

func splitFilename(filename string) (string, string) {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[:i], filename[i:]
		}
	}
	return filename, ""
}
