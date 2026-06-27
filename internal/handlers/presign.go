// Package handlers 预签名上传处理器
// 与 Python 版 apps/base/views.py 的 presign_api 保持一致
package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/middleware"
	"github.com/ischenyu/internal/models"
	"github.com/ischenyu/internal/storage"
	"github.com/ischenyu/internal/utils"
)

// PresignSessionExpires 预签名会话过期时间（秒）
const PresignSessionExpires = 900 // 15分钟

// PresignHandler 预签名上传处理器
type PresignHandler struct {
	DB      *gorm.DB
	Cfg     *config.Settings
	Storage storage.FileStorageInterface
}

// NewPresignHandler 创建预签名上传处理器
func NewPresignHandler(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface) *PresignHandler {
	return &PresignHandler{DB: db, Cfg: cfg, Storage: store}
}

// InitUpload 初始化预签名上传
// POST /presign/upload/init
func (h *PresignHandler) InitUpload(c *gin.Context) {
	var body struct {
		FileName    string `json:"file_name" form:"file_name"`
		FileSize    int64  `json:"file_size" form:"file_size"`
		ExpireValue int    `json:"expire_value" form:"expire_value"`
		ExpireStyle string `json:"expire_style" form:"expire_style"`
	}
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请求参数错误"))
		return
	}

	if body.ExpireValue <= 0 {
		body.ExpireValue = 1
	}
	if body.ExpireStyle == "" {
		body.ExpireStyle = "day"
	}

	// 验证文件类型
	if err := validateFileType(h.Cfg, body.FileName, ""); err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 验证文件大小
	if body.FileSize > h.Cfg.GetInt64("uploadSize") {
		maxMB := float64(h.Cfg.GetInt64("uploadSize")) / (1024 * 1024)
		c.JSON(http.StatusForbidden, utils.Error(403, fmt.Sprintf("文件大小超过限制，最大为 %.2f MB", maxMB)))
		return
	}

	// 验证过期类型
	expireStyles := h.Cfg.GetStringSlice("expireStyle")
	styleValid := false
	for _, s := range expireStyles {
		if s == body.ExpireStyle {
			styleValid = true
			break
		}
	}
	if !styleValid {
		c.JSON(http.StatusBadRequest, utils.Error(400, "过期时间类型错误"))
		return
	}

	// 生成上传ID和文件路径
	_, _, _, filename, savePath := generateFilePath(body.FileName, "")
	uploadID := fmt.Sprintf("%x", time.Now().UnixNano())[:16]
	_, _, _, _, savePath = generateFilePath(body.FileName, uploadID)

	// 尝试生成预签名URL
	presignedURL, err := h.Storage.GeneratePresignedUploadURL(savePath, PresignSessionExpires)
	if err != nil {
		presignedURL = ""
	}

	mode := "proxy"
	if presignedURL != "" {
		mode = "direct"
	}

	proxyUploadURL := fmt.Sprintf("/presign/upload/proxy/%s", uploadID)

	// 创建上传会话
	session := models.PresignUploadSession{
		UploadID:    uploadID,
		FileName:    filename,
		FileSize:    body.FileSize,
		SavePath:    savePath,
		Mode:        mode,
		ExpireValue: body.ExpireValue,
		ExpireStyle: body.ExpireStyle,
		ExpiresAt:   time.Now().Add(time.Duration(PresignSessionExpires) * time.Second),
	}
	h.DB.Create(&session)

	// 记录 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	uploadURL := presignedURL
	if uploadURL == "" {
		uploadURL = proxyUploadURL
	}

	detail := gin.H{
		"upload_id":  uploadID,
		"upload_url": uploadURL,
		"mode":       mode,
		"expires_in": PresignSessionExpires,
	}
	if mode == "proxy" {
		detail["proxy_upload_url"] = proxyUploadURL
		detail["legacy_proxy_upload_url"] = "/api" + proxyUploadURL
	}

	c.JSON(http.StatusOK, utils.Success(detail))
}

// ProxyUpload 代理模式上传
// PUT /presign/upload/proxy/:upload_id
func (h *PresignHandler) ProxyUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	// 获取并验证会话
	session, err := getValidSession(h.DB, uploadID, "proxy")
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}

	// 获取上传文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请选择文件"))
		return
	}

	// 验证文件大小
	if file.Size > h.Cfg.GetInt64("uploadSize") {
		maxMB := float64(h.Cfg.GetInt64("uploadSize")) / (1024 * 1024)
		c.JSON(http.StatusForbidden, utils.Error(403, fmt.Sprintf("大小超过限制，最大为 %.2f MB", maxMB)))
		return
	}

	// 验证文件大小与声明一致
	if absDiff(file.Size, session.FileSize) > 1024 {
		c.JSON(http.StatusBadRequest, utils.Error(400, "文件大小与声明不符"))
		return
	}

	// 保存文件到存储后端
	if err := h.Storage.SaveFile(file, session.SavePath); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, fmt.Sprintf("文件保存失败: %v", err)))
		return
	}

	// 获取过期信息
	expiredAt, expiredCount, usedCount, code, err := getExpireInfo(h.Cfg, session.ExpireValue, session.ExpireStyle)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 文件路径处理
	dir := filepath.Dir(session.SavePath)
	prefix, suffix := splitFilename(session.FileName)

	// 创建文件记录
	fileCode := models.FileCodes{
		Code:         code,
		Prefix:       prefix,
		Suffix:       suffix,
		UUIDFileName: &session.FileName,
		FilePath:     &dir,
		Size:         file.Size,
		ExpiredAt:    expiredAt,
		ExpiredCount: expiredCount,
		UsedCount:    usedCount,
	}
	h.DB.Create(&fileCode)

	// 删除会话
	h.DB.Delete(&session)

	// 记录 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{"code": code, "name": session.FileName}))
}

// ConfirmUpload 直传确认（S3 直传后调用）
// POST /presign/upload/confirm/:upload_id
func (h *PresignHandler) ConfirmUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	session, err := getValidSession(h.DB, uploadID, "direct")
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}

	// 检查文件是否已上传到 S3
	exists, err := h.Storage.FileExists(session.SavePath)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, utils.Error(404, "文件未上传或上传失败"))
		return
	}

	// 获取过期信息
	expiredAt, expiredCount, usedCount, code, err := getExpireInfo(h.Cfg, session.ExpireValue, session.ExpireStyle)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	dir := filepath.Dir(session.SavePath)
	prefix, suffix := splitFilename(session.FileName)

	fileCode := models.FileCodes{
		Code:         code,
		Prefix:       prefix,
		Suffix:       suffix,
		UUIDFileName: &session.FileName,
		FilePath:     &dir,
		Size:         session.FileSize,
		ExpiredAt:    expiredAt,
		ExpiredCount: expiredCount,
		UsedCount:    usedCount,
	}
	h.DB.Create(&fileCode)
	h.DB.Delete(&session)

	// 记录 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{"code": code, "name": session.FileName}))
}

// GetUploadStatus 查询上传会话状态
// GET /presign/upload/status/:upload_id
func (h *PresignHandler) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")

	var session models.PresignUploadSession
	if err := h.DB.Where("upload_id = ?", uploadID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"upload_id":  session.UploadID,
		"file_name":  session.FileName,
		"file_size":  session.FileSize,
		"mode":       session.Mode,
		"created_at": session.CreatedAt.Format(time.RFC3339),
		"expires_at": session.ExpiresAt.Format(time.RFC3339),
		"is_expired": session.IsExpired(),
	}))
}

// CancelUpload 取消上传会话
// DELETE /presign/upload/:upload_id
func (h *PresignHandler) CancelUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	var session models.PresignUploadSession
	if err := h.DB.Where("upload_id = ?", uploadID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "上传会话不存在"))
		return
	}

	// 如果是直传模式，尝试删除已上传的文件
	if session.Mode == "direct" {
		tempCode := &models.FileCodes{
			FilePath:     &session.SavePath,
			UUIDFileName: &session.FileName,
		}
		// 尝试清理
		h.Storage.DeleteFile(tempCode)
	}

	h.DB.Delete(&session)
	c.JSON(http.StatusOK, utils.Success(gin.H{"message": "上传会话已取消"}))
}

// getValidSession 获取并验证预签名上传会话
func getValidSession(db *gorm.DB, uploadID, expectedMode string) (*models.PresignUploadSession, error) {
	var session models.PresignUploadSession
	if err := db.Where("upload_id = ?", uploadID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("上传会话不存在")
	}
	if session.IsExpired() {
		db.Delete(&session)
		return nil, fmt.Errorf("上传会话已过期")
	}
	if expectedMode != "" && session.Mode != expectedMode {
		return nil, fmt.Errorf("此会话不支持%s模式", expectedMode)
	}
	return &session, nil
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}
