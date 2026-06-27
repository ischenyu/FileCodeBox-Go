// Package handlers 管理后台处理器
// 与 Python 版 apps/admin/views.py 的 admin_api 保持一致
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/models"
	"github.com/ischenyu/internal/utils"
)

// AdminHandler 管理后台处理器
type AdminHandler struct {
	DB  *gorm.DB
	Cfg *config.Settings
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(db *gorm.DB, cfg *config.Settings) *AdminHandler {
	return &AdminHandler{DB: db, Cfg: cfg}
}

// Login 管理员登录
// POST /admin/login
func (h *AdminHandler) Login(c *gin.Context) {
	var body struct {
		Password string `json:"password" form:"password"`
	}
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请提供密码"))
		return
	}

	adminToken := h.Cfg.GetString("admin_token")
	if !utils.VerifyPassword(body.Password, adminToken) {
		c.JSON(http.StatusUnauthorized, utils.Error(401, "密码错误"))
		return
	}

	// 创建 JWT token
	jwtSecret := h.Cfg.GetString("jwt_secret")
	token, err := utils.CreateToken([]byte(jwtSecret), map[string]interface{}{
		"is_admin": true,
	}, 30*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"token":      token,
		"token_type": "Bearer",
	}))
}

// Verify 验证管理员身份
// GET /admin/verify
func (h *AdminHandler) Verify(c *gin.Context) {
	payload, _ := c.Get("admin_payload")
	c.JSON(http.StatusOK, utils.Success(payload))
}

// Logout 管理员登出
func (h *AdminHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, utils.Success(gin.H{"ok": true}))
}

// Dashboard 管理后台仪表盘
// GET /admin/dashboard
func (h *AdminHandler) Dashboard(c *gin.Context) {
	var allCodes []models.FileCodes
	h.DB.Find(&allCodes)

	// 统计
	var totalSize int64
	expiredCount := 0
	textCount := 0
	chunkedCount := 0
	totalUsedCount := 0
	suffixCounter := make(map[string]int)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	var yesterdayCount, todayCount int
	var yesterdaySize, todaySize int64

	type recentFile struct {
		ID           int64      `json:"id"`
		Code         string     `json:"code"`
		Name         string     `json:"name"`
		Suffix       string     `json:"suffix"`
		Size         int64      `json:"size"`
		IsText       bool       `json:"text"`
		ExpiredAt    *time.Time `json:"expiredAt"`
		ExpiredCount int        `json:"expiredCount"`
		UsedCount    int        `json:"usedCount"`
		CreatedAt    time.Time  `json:"createdAt"`
		IsExpired    bool       `json:"isExpired"`
	}

	var recentFiles []recentFile
	for _, fc := range allCodes {
		totalSize += fc.Size
		totalUsedCount += fc.UsedCount

		if fc.IsExpired() {
			expiredCount++
		}
		if fc.Text != nil {
			textCount++
			suffixCounter["Text"]++
		} else {
			s := fc.Suffix
			if s == "" {
				s = "file"
			}
			suffixCounter[s]++
		}
		if fc.IsChunked {
			chunkedCount++
		}

		if fc.CreatedAt.After(yesterdayStart) && fc.CreatedAt.Before(todayStart) {
			yesterdayCount++
			yesterdaySize += fc.Size
		}
		if fc.CreatedAt.After(todayStart) || fc.CreatedAt.Equal(todayStart) {
			todayCount++
			todaySize += fc.Size
		}
	}

	// 排序获取最近的 8 条
	sortByCreatedAt(allCodes)
	for i := 0; i < len(allCodes) && i < 8; i++ {
		fc := allCodes[i]
		recentFiles = append(recentFiles, recentFile{
			ID:           fc.ID,
			Code:         fc.Code,
			Name:         fc.Prefix + fc.Suffix,
			Suffix:       fc.Suffix,
			Size:         fc.Size,
			IsText:       fc.Text != nil,
			ExpiredAt:    fc.ExpiredAt,
			ExpiredCount: fc.ExpiredCount,
			UsedCount:    fc.UsedCount,
			CreatedAt:    fc.CreatedAt,
			IsExpired:    fc.IsExpired(),
		})
	}

	// 系统启动时间
	var sysStart models.KeyValue
	sysUptime := interface{}(nil)
	if err := h.DB.Where("key = ?", "sys_start").First(&sysStart).Error; err == nil && sysStart.Value.Valid {
		sysUptime = sysStart.Value.String
	}

	// 构建后缀统计
	type suffixStat struct {
		Suffix string `json:"suffix"`
		Count  int    `json:"count"`
	}
	var topSuffixes []suffixStat
	for suffix, count := range suffixCounter {
		topSuffixes = append(topSuffixes, suffixStat{Suffix: suffix, Count: count})
	}
	// 按数量降序排列（简化）
	for i := 0; i < len(topSuffixes); i++ {
		for j := i + 1; j < len(topSuffixes); j++ {
			if topSuffixes[j].Count > topSuffixes[i].Count {
				topSuffixes[i], topSuffixes[j] = topSuffixes[j], topSuffixes[i]
			}
		}
	}
	if len(topSuffixes) > 8 {
		topSuffixes = topSuffixes[:8]
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"totalFiles":       len(allCodes),
		"storageUsed":      fmt.Sprintf("%d", totalSize),
		"sysUptime":        sysUptime,
		"yesterdayCount":   yesterdayCount,
		"yesterdaySize":    fmt.Sprintf("%d", yesterdaySize),
		"todayCount":       todayCount,
		"todaySize":        fmt.Sprintf("%d", todaySize),
		"activeCount":      len(allCodes) - expiredCount,
		"expiredCount":     expiredCount,
		"textCount":        textCount,
		"fileCount":        len(allCodes) - textCount,
		"chunkedCount":     chunkedCount,
		"usedCount":        totalUsedCount,
		"storageBackend":   h.Cfg.GetString("file_storage"),
		"uploadSizeLimit":  h.Cfg.GetInt64("uploadSize"),
		"openUpload":       h.Cfg.GetBool("openUpload"),
		"enableChunk":      h.Cfg.GetBool("enableChunk"),
		"maxSaveSeconds":   h.Cfg.GetInt64("max_save_seconds"),
		"topSuffixes":      topSuffixes,
		"recentFiles":      recentFiles,
		"recentActivities": []interface{}{},
	}))
}

// FileList 文件列表
// GET /admin/file/list?page=1&size=10&keyword=&status=&type=
func (h *AdminHandler) FileList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	keyword := c.Query("keyword")
	status := c.Query("status")
	fileType := c.Query("type")
	sortBy := c.DefaultQuery("sortBy", "created_at")
	sortOrder := c.DefaultQuery("sortOrder", "desc")

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	query := h.DB.Model(&models.FileCodes{})

	// 关键词搜索
	if keyword != "" {
		query = query.Where("code LIKE ? OR prefix LIKE ? OR suffix LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 状态过滤
	switch status {
	case "active":
		query = query.Where("(expired_at IS NULL OR expired_at > ?) AND (expired_count > 0 OR expired_count = -1)", time.Now())
	case "expired":
		query = query.Where("(expired_at IS NOT NULL AND expired_at <= ?) OR expired_count = 0", time.Now())
	}

	// 类型过滤
	switch fileType {
	case "file":
		query = query.Where("text IS NULL")
	case "text":
		query = query.Where("text IS NOT NULL")
	case "chunked":
		query = query.Where("is_chunked = ?", true)
	}

	// 排序
	allowedSorts := map[string]string{
		"created_at": "created_at",
		"createdat":  "created_at",
		"expired_at": "expired_at",
		"expiredat":  "expired_at",
		"name":       "prefix",
		"size":       "size",
		"used_count": "used_count",
		"usedcount":  "used_count",
		"code":       "code",
	}
	sortField, ok := allowedSorts[sortBy]
	if !ok {
		sortField = "created_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortField, sortOrder))

	// 计数
	var total int64
	query.Count(&total)

	// 分页
	var files []models.FileCodes
	query.Offset((page - 1) * size).Limit(size).Find(&files)

	// 构建文件数据
	type fileItem struct {
		ID           int64      `json:"id"`
		Code         string     `json:"code"`
		Name         string     `json:"name"`
		Suffix       string     `json:"suffix"`
		Size         int64      `json:"size"`
		IsText       bool       `json:"isText"`
		IsChunked    bool       `json:"isChunked"`
		ExpiredAt    *time.Time `json:"expiredAt"`
		ExpiredCount int        `json:"expiredCount"`
		UsedCount    int        `json:"usedCount"`
		CreatedAt    time.Time  `json:"createdAt"`
		IsExpired    bool       `json:"isExpired"`
		FileHash     *string    `json:"fileHash"`
	}
	var data []fileItem
	for _, f := range files {
		data = append(data, fileItem{
			ID:           f.ID,
			Code:         f.Code,
			Name:         f.Prefix + f.Suffix,
			Suffix:       f.Suffix,
			Size:         f.Size,
			IsText:       f.Text != nil,
			IsChunked:    f.IsChunked,
			ExpiredAt:    f.ExpiredAt,
			ExpiredCount: f.ExpiredCount,
			UsedCount:    f.UsedCount,
			CreatedAt:    f.CreatedAt,
			IsExpired:    f.IsExpired(),
			FileHash:     f.FileHash,
		})
	}

	c.JSON(http.StatusOK, utils.Success(gin.H{
		"page":  page,
		"size":  size,
		"data":  data,
		"total": total,
		"summary": gin.H{
			"total": total,
			"page":  page,
			"size":  size,
		},
	}))
}

// FileDelete 删除文件
// DELETE /admin/file/delete
func (h *AdminHandler) FileDelete(c *gin.Context) {
	var body struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBind(&body); err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请提供文件ID"))
		return
	}

	var fc models.FileCodes
	if err := h.DB.First(&fc, body.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, utils.Error(404, "文件不存在"))
		return
	}

	h.DB.Delete(&fc)
	c.JSON(http.StatusOK, utils.Success(nil))
}

// ConfigGet 获取配置
// GET /admin/config/get
func (h *AdminHandler) ConfigGet(c *gin.Context) {
	// 返回当前运行时配置（敏感信息已脱敏）
	cfg := map[string]interface{}{
		"file_storage":       h.Cfg.GetString("file_storage"),
		"storage_path":       h.Cfg.GetString("storage_path"),
		"name":               h.Cfg.GetString("name"),
		"description":        h.Cfg.GetString("description"),
		"notify_title":       h.Cfg.GetString("notify_title"),
		"notify_content":     h.Cfg.GetString("notify_content"),
		"page_explain":       h.Cfg.GetString("page_explain"),
		"keywords":           h.Cfg.GetString("keywords"),
		"openUpload":         h.Cfg.GetBool("openUpload"),
		"uploadSize":         h.Cfg.GetInt64("uploadSize"),
		"max_save_seconds":   h.Cfg.GetInt64("max_save_seconds"),
		"enableChunk":        h.Cfg.GetBool("enableChunk"),
		"showAdminAddr":      h.Cfg.GetBool("showAdminAddr"),
		"expireStyle":        h.Cfg.GetStringSlice("expireStyle"),
		"allowed_file_types": h.Cfg.GetStringSlice("allowed_file_types"),
		"code_generate_type": h.Cfg.GetString("code_generate_type"),
		"uploadMinute":       h.Cfg.GetInt("uploadMinute"),
		"uploadCount":        h.Cfg.GetInt("uploadCount"),
		"errorMinute":        h.Cfg.GetInt("errorMinute"),
		"errorCount":         h.Cfg.GetInt("errorCount"),
		"opacity":            h.Cfg.GetFloat64("opacity"),
		"background":         h.Cfg.GetString("background"),
		"robotsText":         h.Cfg.GetString("robotsText"),
		"themesSelect":       h.Cfg.GetString("themesSelect"),
	}
	c.JSON(http.StatusOK, utils.Success(cfg))
}

// ConfigUpdate 更新配置
// PATCH /admin/config/update
func (h *AdminHandler) ConfigUpdate(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, utils.Error(400, "请求格式错误"))
		return
	}

	// 移除 themesChoices（前端可能发过来但不允许修改）
	delete(updates, "themesChoices")

	// 读取当前配置
	var kv models.KeyValue
	if err := h.DB.Where("key = ?", "settings").First(&kv).Error; err != nil {
		c.JSON(http.StatusInternalServerError, utils.Error(500, "配置不存在"))
		return
	}

	currentConfig := make(map[string]interface{})
	if kv.Value.Valid {
		json.Unmarshal([]byte(kv.Value.String), &currentConfig)
	}

	// 处理密码更新
	if newPassword, ok := updates["adminPassword"].(string); ok && newPassword != "" {
		updates["admin_token"] = utils.HashPassword(newPassword)
		delete(updates, "adminPassword")
	}

	// 合并更新
	for k, v := range updates {
		currentConfig[k] = v
	}

	// 序列化保存
	configJSON, _ := json.Marshal(currentConfig)
	kv.Value.String = string(configJSON)
	kv.Value.Valid = true
	h.DB.Save(&kv)

	// 重新加载配置
	h.Cfg.RefreshFromDB(h.DB)

	c.JSON(http.StatusOK, utils.Success(nil))
}

// sortByCreatedAt 按创建时间降序排序
func sortByCreatedAt(codes []models.FileCodes) {
	for i := 0; i < len(codes); i++ {
		for j := i + 1; j < len(codes); j++ {
			if codes[j].CreatedAt.After(codes[i].CreatedAt) {
				codes[i], codes[j] = codes[j], codes[i]
			}
		}
	}
}
