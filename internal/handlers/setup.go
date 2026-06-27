// Package handlers 初始化向导页面处理器
// 与 Python 版 main.py 的 /setup 路由保持一致
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ischenyu/internal/config"
	"github.com/ischenyu/internal/database"
	"github.com/ischenyu/internal/middleware"
	"github.com/ischenyu/internal/models"
	"github.com/ischenyu/internal/utils"
)

// SetupHandler 初始化向导处理器
type SetupHandler struct {
	DB  *gorm.DB
	Cfg *config.Settings
}

// NewSetupHandler 创建初始化向导处理器
func NewSetupHandler(db *gorm.DB, cfg *config.Settings) *SetupHandler {
	return &SetupHandler{DB: db, Cfg: cfg}
}

// GetSetupPage 返回初始化页面
func (h *SetupHandler) GetSetupPage(c *gin.Context) {
	// 如果已经初始化，重定向到首页
	if middleware.IsConfigInitialized(h.Cfg) {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildSetupPage()))
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
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildSetupPageWithError("请求格式错误")))
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
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(buildSetupPageWithError("两次输入的管理员密码不一致")))
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
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildSetupSuccessPage()))
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

// buildSetupPage 构建初始化页面 HTML
func buildSetupPage() string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>初始化 FileCodeBox</title>
  <style>
    :root { color-scheme: light; --bg: #f5f5f7; --panel: rgba(255,255,255,.86); --text: #18181b; --muted: #71717a; --line: rgba(228,228,231,.9); }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 14px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: var(--bg); color: var(--text); }
    main { width: min(100%, 580px); padding: 28px; border-radius: 20px; background: rgba(255,255,255,.82); box-shadow: 0 22px 70px -34px rgba(24,24,27,.32); backdrop-filter: blur(22px); }
    .header { display: flex; align-items: center; gap: 16px; margin-bottom: 24px; }
    .brand { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 14px; background: #18181b; color: #fff; font-weight: 800; }
    h1 { margin: 0; font-size: 21px; }
    p { margin: 2px 0 0; color: var(--muted); font-size: 13px; }
    label { display: block; margin: 12px 0 5px; font-weight: 650; font-size: 13px; color: #3f3f46; }
    input { width: 100%; height: 38px; border: 1px solid var(--line); border-radius: 10px; padding: 0 12px; font: inherit; outline: none; background: rgba(255,255,255,.84); }
    input:focus { border-color: #a1a1aa; box-shadow: 0 0 0 3px rgba(24,24,27,.06); }
    button { width: 100%; height: 42px; border: none; border-radius: 10px; background: #18181b; color: #fff; font-weight: 700; cursor: pointer; margin-top: 20px; }
    button:hover { background: #27272a; }
    .alert { padding: 12px; border-radius: 10px; background: #fef2f2; color: #b91c1c; margin-bottom: 16px; font-size: 13px; }
  </style>
</head>
<body>
  <main>
    <div class="header"><div class="brand">FCB</div><div><h1>初始化 FileCodeBox</h1><p>首次配置管理员密码，后续可在后台调整。</p></div></div>
    <form method="post" action="/setup" autocomplete="off">
      <input type="hidden" name="openUpload" value="1">
      <input type="hidden" name="enableChunk" value="0">
      <input type="hidden" name="uploadSize" value="10">
      <label for="site_name">站点名称</label>
      <input id="site_name" name="site_name" maxlength="80" value="文件快递柜 - FileCodeBox">
      <label for="admin_password">管理员密码（至少8位）</label>
      <input id="admin_password" name="admin_password" type="password" minlength="8" required autofocus>
      <label for="confirm_password">确认管理员密码</label>
      <input id="confirm_password" name="confirm_password" type="password" minlength="8" required>
      <button type="submit">完成初始化</button>
    </form>
  </main>
</body>
</html>`
}

func buildSetupPageWithError(errorMsg string) string {
	page := buildSetupPage()
	alert := fmt.Sprintf(`<div class="alert" role="alert">%s</div>`, errorMsg)
	return strings.Replace(page, `<form method="post"`, alert+`<form method="post"`, 1)
}

func buildSetupSuccessPage() string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="2;url=/#/admin">
  <title>初始化完成</title>
  <style>
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 24px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f6f8fb; color: #172033; }
    main { width: min(100%, 420px); padding: 28px; border-radius: 8px; background: #fff; text-align: center; box-shadow: 0 18px 50px rgba(23,32,51,.08); }
    h1 { margin: 0 0 10px; font-size: 24px; }
    p { margin: 0 0 22px; color: #60708a; }
    a { display: inline-flex; align-items: center; justify-content: center; min-height: 42px; padding: 0 18px; border-radius: 6px; background: #2563eb; color: #fff; text-decoration: none; font-weight: 700; }
  </style>
</head>
<body>
  <main>
    <h1>初始化完成</h1>
    <p>管理员密码已设置，请使用刚才的密码登录后台。</p>
    <a href="/#/admin">进入后台</a>
  </main>
</body>
</html>`
}
