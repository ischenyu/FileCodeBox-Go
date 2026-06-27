// Package handlers 分享相关处理器（文本/文件上传、获取、下载）
// 与 Python 版 apps/base/views.py 的 share_api 保持一致
package handlers

import (
	"crypto/sha256"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/middleware"
	"github.com/ischenyu/internal/models"
	"github.com/ischenyu/internal/storage"
	"github.com/ischenyu/internal/utils"
)

// ShareHandler 分享处理器
type ShareHandler struct {
	DB      *gorm.DB
	Cfg     *config.Settings
	Storage storage.FileStorageInterface
}

// NewShareHandler 创建分享处理器
func NewShareHandler(db *gorm.DB, cfg *config.Settings, store storage.FileStorageInterface) *ShareHandler {
	return &ShareHandler{DB: db, Cfg: cfg, Storage: store}
}

//  文本分享

// ShareText 分享文本
// POST /share/text/
func (h *ShareHandler) ShareText(c *gin.Context) {
	text := c.PostForm("text")
	expireValue := getIntFormDefault(c, "expire_value", 1)
	expireStyle := c.DefaultPostForm("expire_style", "day")

	// 文本大小限制 222KB
	textSize := len([]byte(text))
	if textSize > 222*1024 {
		c.JSON(http.StatusForbidden, utils.Error(403, "内容过多，建议采用文件形式"))
		return
	}

	// 获取过期信息
	expiredAt, expiredCount, usedCount, code, err := getExpireInfo(h.Cfg, expireValue, expireStyle)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 创建文本记录
	fileCode := models.FileCodes{
		Code:         code,
		Text:         &text,
		ExpiredAt:    expiredAt,
		ExpiredCount: expiredCount,
		UsedCount:    usedCount,
		Size:         int64(textSize),
		Prefix:       "Text",
	}
	if err := h.DB.Create(&fileCode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, "创建文本记录失败"))
		return
	}

	// 记录上传 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{"code": code}))
}

//  文件分享

// ShareFile 分享文件
// POST /share/file/
func (h *ShareHandler) ShareFile(c *gin.Context) {
	expireValue := getIntFormDefault(c, "expire_value", 1)
	expireStyle := c.DefaultPostForm("expire_style", "day")

	// 获取上传文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请选择文件"))
		return
	}

	// 验证文件大小
	uploadSize := h.Cfg.GetInt64("uploadSize")
	if file.Size > uploadSize {
		maxMB := float64(uploadSize) / (1024 * 1024)
		c.JSON(http.StatusForbidden, utils.Error(403, fmt.Sprintf("大小超过限制，最大为 %.2f MB", maxMB)))
		return
	}

	// 验证文件类型
	if err := validateFileType(h.Cfg, file.Filename, file.Header.Get("Content-Type")); err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 验证过期类型
	expireStyles := h.Cfg.GetStringSlice("expireStyle")
	styleValid := false
	for _, s := range expireStyles {
		if s == expireStyle {
			styleValid = true
			break
		}
	}
	if !styleValid {
		c.JSON(http.StatusBadRequest, utils.Error(400, "过期时间类型错误"))
		return
	}

	// 获取过期信息
	expiredAt, expiredCount, usedCount, code, err := getExpireInfo(h.Cfg, expireValue, expireStyle)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.Error(403, err.Error()))
		return
	}

	// 生成文件路径
	path, suffix, prefix, uuidFileName, savePath := generateFilePath(file.Filename, "")

	// 保存文件到存储后端
	if err := h.Storage.SaveFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, fmt.Sprintf("文件保存失败: %v", err)))
		return
	}

	// 创建文件记录
	fileCode := models.FileCodes{
		Code:         code,
		Prefix:       prefix,
		Suffix:       suffix,
		UUIDFileName: &uuidFileName,
		FilePath:     &path,
		Size:         file.Size,
		ExpiredAt:    expiredAt,
		ExpiredCount: expiredCount,
		UsedCount:    usedCount,
	}
	if err := h.DB.Create(&fileCode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, "创建文件记录失败"))
		return
	}

	// 记录上传 IP
	if ip, ok := c.Get("client_ip"); ok {
		middleware.UploadLimit.AddIP(ip.(string))
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{"code": code, "name": file.Filename}))
}

//  文件获取

// GetFileMetadata 获取文件元数据（GET）
// GET /share/metadata/?code=xxx
func (h *ShareHandler) GetFileMetadata(c *gin.Context) {
	code := c.Query("code")
	fileCode, err := h.getCodeFile(code)
	if err != nil {
		recordErrorIP(c)
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, utils.Success(buildFileMetadata(fileCode)))
}

// PostFileMetadata 获取文件元数据（POST）
func (h *ShareHandler) PostFileMetadata(c *gin.Context) {
	var body struct {
		Code string `json:"code" form:"code"`
	}
	c.ShouldBind(&body)
	fileCode, err := h.getCodeFile(body.Code)
	if err != nil {
		recordErrorIP(c)
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, utils.Success(buildFileMetadata(fileCode)))
}

// GetCodeFile 获取文件内容（GET）- 直接下载
// GET /share/select/?code=xxx
func (h *ShareHandler) GetCodeFile(c *gin.Context) {
	code := c.Query("code")
	fileCode, err := h.getCodeFile(code)
	if err != nil {
		recordErrorIP(c)
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}

	// 更新使用次数
	h.updateFileUsage(fileCode)

	// 返回文件响应
	h.serveFileResponse(c, fileCode)
}

// SelectFile 获取文件详情（POST）
// POST /share/select/
func (h *ShareHandler) SelectFile(c *gin.Context) {
	var body struct {
		Code string `json:"code" form:"code"`
	}
	c.ShouldBind(&body)

	fileCode, err := h.getCodeFile(body.Code)
	if err != nil {
		recordErrorIP(c)
		c.JSON(http.StatusNotFound, utils.Error(404, err.Error()))
		return
	}

	// 更新使用次数
	h.updateFileUsage(fileCode)

	// 构建详情
	detail := buildFileMetadata(fileCode)
	if fileCode.Text != nil {
		detail["text"] = *fileCode.Text
		detail["content"] = *fileCode.Text
	} else {
		downloadURL := h.Storage.GetFileURL(fileCode)
		detail["text"] = downloadURL
		detail["download_url"] = downloadURL
	}
	c.JSON(http.StatusOK, utils.Success(detail))
}

// DownloadFile 通过鉴权 token 下载文件
// GET /share/download?key=xxx&code=xxx
func (h *ShareHandler) DownloadFile(c *gin.Context) {
	key := c.Query("key")
	code := c.Query("code")

	// 验证下载 token
	jwtSecret := h.Cfg.GetString("jwt_secret")
	expectedKey := generateSelectToken(code, jwtSecret)
	if key != expectedKey {
		recordErrorIP(c)
		c.JSON(http.StatusForbidden, utils.Error(403, "下载鉴权失败"))
		return
	}

	fileCode, err := h.getCodeFileNoCheck(code)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "文件不存在"))
		return
	}

	// 如果是文本，直接返回文本内容
	if fileCode.Text != nil {
		c.JSON(http.StatusOK, utils.Success(*fileCode.Text))
		return
	}

	// 文件下载
	h.serveFileResponse(c, fileCode)
}

//  辅助方法

// getCodeFile 通过 code 查询文件记录
func (h *ShareHandler) getCodeFile(code string) (*models.FileCodes, error) {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		return nil, fmt.Errorf("文件不存在")
	}
	var fileCode models.FileCodes
	if err := h.DB.Where("code = ?", normalizedCode).First(&fileCode).Error; err != nil {
		return nil, fmt.Errorf("文件不存在")
	}
	if fileCode.IsExpired() {
		return nil, fmt.Errorf("文件已过期")
	}
	return &fileCode, nil
}

func (h *ShareHandler) getCodeFileNoCheck(code string) (*models.FileCodes, error) {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		return nil, fmt.Errorf("文件不存在")
	}
	var fileCode models.FileCodes
	if err := h.DB.Where("code = ?", normalizedCode).First(&fileCode).Error; err != nil {
		return nil, fmt.Errorf("文件不存在")
	}
	return &fileCode, nil
}

func (h *ShareHandler) updateFileUsage(fileCode *models.FileCodes) {
	fileCode.UsedCount++
	if fileCode.ExpiredCount > 0 {
		fileCode.ExpiredCount--
	}
	h.DB.Model(fileCode).Updates(map[string]interface{}{
		"used_count":    fileCode.UsedCount,
		"expired_count": fileCode.ExpiredCount,
	})
}

func (h *ShareHandler) serveFileResponse(c *gin.Context, fileCode *models.FileCodes) {
	if fileCode.Text != nil {
		c.String(http.StatusOK, *fileCode.Text)
		return
	}

	filePath, err := h.Storage.GetFileResponse(fileCode)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, fmt.Sprintf("获取文件失败: %v", err)))
		return
	}

	// 本地文件直接返回
	if filePath != "" {
		filename := fileCode.Prefix + fileCode.Suffix
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, urlEncode(filename)))
		c.File(filePath)
		return
	}

	// 远程存储需要流式代理（S3/WebDAV/OneDrive）
	c.JSON(http.StatusNotImplemented, utils.Error(501, "该存储后端需要通过代理下载"))
}

func recordErrorIP(c *gin.Context) {
	if ip, ok := c.Get("client_ip"); ok {
		middleware.ErrorLimit.AddIP(ip.(string))
	}
}

// buildFileMetadata 构建文件元数据 map
func buildFileMetadata(fileCode *models.FileCodes) map[string]interface{} {
	isText := fileCode.Text != nil
	var remainingDownloads interface{} = nil
	if fileCode.ExpiredCount > 0 {
		remainingDownloads = fileCode.ExpiredCount
	}
	return map[string]interface{}{
		"code":                fileCode.Code,
		"name":                fileCode.Prefix + fileCode.Suffix,
		"size":                fileCode.Size,
		"type":                map[bool]string{true: "text", false: "file"}[isText],
		"is_text":             isText,
		"created_at":          fileCode.CreatedAt,
		"expired_at":          fileCode.ExpiredAt,
		"expires_at":          fileCode.ExpiredAt,
		"expired_count":       fileCode.ExpiredCount,
		"used_count":          fileCode.UsedCount,
		"remaining_downloads": remainingDownloads,
	}
}

// generateFilePath 生成文件存储路径
// 返回: path, suffix, prefix, uuidFileName, savePath
func generateFilePath(filename, uploadID string) (string, string, string, string, string) {
	now := time.Now()
	fileUUID := uploadID
	if fileUUID == "" {
		fileUUID = fmt.Sprintf("%x", sha256.Sum256([]byte(time.Now().String())))[:16]
	}
	safeFilename := utils.SanitizeFilename(filename)
	basePath := fmt.Sprintf("share/data/%s/%s", now.Format("2006/01/02"), fileUUID)
	prefix := ""
	suffix := filepath.Ext(safeFilename)
	if suffix != "" {
		prefix = safeFilename[:len(safeFilename)-len(suffix)]
	} else {
		prefix = safeFilename
	}
	savePath := filepath.ToSlash(filepath.Join(basePath, safeFilename))
	return basePath, suffix, prefix, safeFilename, savePath
}

// getExpireInfo 计算过期信息
// 返回: expiredAt, expiredCount, usedCount, code, error
func getExpireInfo(cfg *config.Settings, expireValue int, expireStyle string) (*time.Time, int, int, string, error) {
	now := time.Now()
	expiredCount := -1
	usedCount := 0

	// 计算最大保存时间
	maxSaveSeconds := cfg.GetInt64("max_save_seconds")
	maxDuration := time.Duration(7 * 24 * time.Hour) // 默认7天
	if maxSaveSeconds > 0 {
		maxDuration = time.Duration(maxSaveSeconds) * time.Second
	}

	var expiredAt *time.Time
	switch expireStyle {
	case "day":
		t := now.Add(time.Duration(expireValue) * 24 * time.Hour)
		expiredAt = &t
	case "hour":
		t := now.Add(time.Duration(expireValue) * time.Hour)
		expiredAt = &t
	case "minute":
		t := now.Add(time.Duration(expireValue) * time.Minute)
		expiredAt = &t
	case "count":
		t := now.Add(24 * time.Hour)
		expiredAt = &t
		expiredCount = expireValue
	case "forever":
		expiredAt = nil
		expiredCount = -1
	default:
		t := now.Add(24 * time.Hour)
		expiredAt = &t
	}

	// 检查是否超过最大保存时间
	if expiredAt != nil && expiredAt.Sub(now) > maxDuration {
		return nil, 0, 0, "", fmt.Errorf("限制最长时间，可换用其他方式")
	}

	// 生成提取码
	codeGenerateType := cfg.GetString("code_generate_type")
	code, err := generateUniqueCode(codeGenerateType, nil)
	if err != nil {
		return nil, 0, 0, "", fmt.Errorf("生成提取码失败: %w", err)
	}

	return expiredAt, expiredCount, usedCount, code, nil
}

// generateUniqueCode 生成唯一提取码
func generateUniqueCode(style string, db *gorm.DB) (string, error) {
	for i := 0; i < 10; i++ {
		code, err := utils.RandomCode(style)
		if err != nil {
			return "", err
		}
		var count int64
		if db != nil {
			db.Model(&models.FileCodes{}).Where("code = ?", code).Count(&count)
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("生成提取码失败，尝试次数过多")
}

// validateFileType 验证文件类型是否在允许列表中
func validateFileType(cfg *config.Settings, filename, contentType string) error {
	allowed := cfg.GetStringSlice("allowed_file_types")
	if len(allowed) == 0 {
		return nil
	}

	// 如果包含 * 则全部允许
	for _, rule := range allowed {
		if rule == "*" || rule == "*/*" {
			return nil
		}
	}

	normalizedName := strings.ToLower(filename)
	normalizedContentType := strings.ToLower(contentType)

	for _, rule := range allowed {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}

		// 检查 contentType 匹配
		if strings.Contains(rule, "/") {
			if matchWildcard(normalizedContentType, rule) {
				return nil
			}
		}

		// 检查扩展名匹配
		ext := rule
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if strings.HasSuffix(normalizedName, ext) {
			return nil
		}
	}

	return fmt.Errorf("不允许上传该类型文件")
}

// matchWildcard 简单通配符匹配
func matchWildcard(value, pattern string) bool {
	if pattern == "*" || pattern == "*/*" {
		return true
	}
	// image/* 匹配 image/png 等
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2]
		return strings.HasPrefix(value, prefix+"/")
	}
	return value == pattern
}

// generateSelectToken 生成下载鉴权 token
func generateSelectToken(code, secret string) string {
	if secret == "" {
		secret = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	timestamp := time.Now().UnixMilli() / 1000
	data := fmt.Sprintf("%s%d000%s", code, timestamp, secret)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:])
}

// urlEncode URL 编码（RFC 3986）
func urlEncode(s string) string {
	// 简单实现，生产环境用 net/url.PathEscape
	result := ""
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '~' {
			result += string(r)
		} else {
			result += fmt.Sprintf("%%%02X", r)
		}
	}
	return result
}

// 工具函数
func getIntFormDefault(c *gin.Context, key string, defaultVal int) int {
	val := c.DefaultPostForm(key, fmt.Sprintf("%d", defaultVal))
	var result int
	fmt.Sscanf(val, "%d", &result)
	if result <= 0 {
		return defaultVal
	}
	return result
}

// 确保 import 使用
var _ = multipart.ErrMessageTooLarge
