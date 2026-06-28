// Package handlers 初始化向导页面处理器
// 与 Python 版 main.py 的 /setup 路由保持一致
package handlers

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/database"
	"github.com/ischenyu/FileCodeBox-Go/internal/middleware"
	"github.com/ischenyu/FileCodeBox-Go/internal/models"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// SetupHandler 初始化向导处理器
type SetupHandler struct {
	DB             *gorm.DB
	Cfg            *config.Settings
	embeddedAssets *embed.FS
}

// NewSetupHandler 创建初始化向导处理器
func NewSetupHandler(db *gorm.DB, cfg *config.Settings, embeddedAssets *embed.FS) *SetupHandler {
	return &SetupHandler{DB: db, Cfg: cfg, embeddedAssets: embeddedAssets}
}

// GetSetupPage 返回初始化页面
func (h *SetupHandler) GetSetupPage(c *gin.Context) {
	// 优先嵌入资源，回退到磁盘文件
	if h.embeddedAssets != nil {
		data, err := fs.ReadFile(h.embeddedAssets, "assets/setup.html")
		if err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			return
		}
	}
	c.File("assets/setup.html")
}

// PostSetup 处理初始化表单提交
func (h *SetupHandler) PostSetup(c *gin.Context) {
	if middleware.IsConfigInitialized(h.Cfg) {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	// 解析请求体（支持 form 和 JSON）
	contentType := c.GetHeader("Content-Type")
	var data map[string]interface{}

	if strings.Contains(contentType, "application/json") {
		body, _ := io.ReadAll(c.Request.Body)
		json.Unmarshal(body, &data)
	} else {
		if err := c.Request.ParseForm(); err != nil {
			c.Redirect(http.StatusSeeOther, "/setup?error="+urlEncode("请求格式错误"))
			return
		}
		data = make(map[string]interface{})
		for k, v := range c.Request.PostForm {
			if len(v) == 1 {
				data[k] = v[0]
			} else {
				data[k] = v
			}
		}
	}

	adminPassword := getFormValue(data, "admin_password", "")
	confirmPassword := getFormValue(data, "confirm_password", "")

	if adminPassword != confirmPassword {
		c.Redirect(http.StatusSeeOther, "/setup?error="+urlEncode("两次输入的管理员密码不一致"))
		return
	}

	// 执行初始化
	database.StartupLock.Lock()
	defer database.StartupLock.Unlock()

	// 获取当前配置
	var kv models.KeyValue
	currentConfig := make(map[string]interface{})
	for k, v := range config.DefaultConfig {
		currentConfig[k] = v
	}
	if err := h.DB.Where("key = ?", "settings").First(&kv).Error; err == nil && kv.Value.Valid {
		json.Unmarshal([]byte(kv.Value.String), &currentConfig)
	}

	// 应用表单设置
	siteName := getFormValue(data, "site_name", "")
	if siteName != "" {
		currentConfig["name"] = siteName
	}

	// 哈希管理员密码
	currentConfig["admin_token"] = utils.HashPassword(adminPassword)

	// 生成 JWT secret
	if currentConfig["jwt_secret"] == nil || currentConfig["jwt_secret"] == "" {
		currentConfig["jwt_secret"] = generateJWTSecret()
	}

	// 保存配置
	configJSON, _ := json.Marshal(currentConfig)
	kv = models.KeyValue{
		Key: "settings",
		Value: sql.NullString{
			String: string(configJSON),
			Valid:  true,
		},
	}
	h.DB.Where("key = ?", "settings").Assign(kv).FirstOrCreate(&kv)

	// 重新加载配置
	h.Cfg.RefreshFromDB(h.DB)

	// 返回结果
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, utils.Success(gin.H{"ok": true, "admin": "/#/admin"}))
		return
	}
	c.Redirect(http.StatusSeeOther, "/#/admin")
}

func getFormValue(data map[string]interface{}, key, defaultVal string) string {
	v, ok := data[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case string:
		return val
	case []string:
		if len(val) > 0 {
			return val[len(val)-1]
		}
	case []interface{}:
		if len(val) > 0 {
			return fmt.Sprintf("%v", val[len(val)-1])
		}
	}
	return fmt.Sprintf("%v", v)
}

func generateJWTSecret() string {
	return utils.HashPassword("jwt_secret_" + fmt.Sprintf("%d", time.Now().UnixNano()))
}
