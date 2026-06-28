// Package config 提供运行时配置管理
// 与 Python 版 core/settings.py 的 Settings 类保持一致
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// DefaultConfig 默认配置值
// 与 Python 版 DEFAULT_CONFIG 字典完全一致
var DefaultConfig = map[string]interface{}{
	"file_storage":         "local",
	"storage_path":         "",
	"name":                 "文件快递柜 - FileCodeBox-Go",
	"description":          "开箱即用的文件快传系统",
	"notify_title":         "系统通知",
	"notify_content":       `欢迎使用 FileCodeBox-Go, 本程序开源于 <a href="https://github.com/vastsa/FileCodeBox" target="_blank">Github</a> ，欢迎Star和Fork。`,
	"page_explain":         "请勿上传或分享违法内容。根据《中华人民共和国网络安全法》、《中华人民共和国刑法》、《中华人民共和国治安管理处罚法》等相关规定。 传播或存储违法、违规内容，会受到相关处罚，严重者将承担刑事责任。本站坚决配合相关部门，确保网络内容的安全，和谐，打造绿色网络环境。",
	"keywords":             "FileCodeBox, 文件快递柜, 口令传送箱, 匿名口令分享文本, 文件",
	"s3_access_key_id":     "",
	"s3_secret_access_key": "",
	"s3_bucket_name":       "",
	"s3_endpoint_url":      "",
	"s3_region_name":       "auto",
	"s3_signature_version": "s3v2",
	"s3_hostname":          "",
	"s3_addressing_style":  "auto",
	"s3_proxy":             0,
	"max_save_seconds":     0,
	"aws_session_token":    "",
	"onedrive_domain":      "",
	"onedrive_client_id":   "",
	"onedrive_username":    "",
	"onedrive_password":    "",
	"onedrive_root_path":   "filebox_storage",
	"onedrive_proxy":       0,
	"webdav_root_path":     "filebox_storage",
	"webdav_proxy":         0,
	"admin_token":          "",
	"jwt_secret":           "",
	"openUpload":           1,
	"uploadSize":           int64(1024 * 1024 * 10),
	"allowed_file_types":   []interface{}{"*"},
	"expireStyle":          []interface{}{"day", "hour", "minute", "forever", "count"},
	"code_generate_type":   "number",
	"uploadMinute":         1,
	"enableChunk":          0,
	"webdav_url":           "",
	"webdav_password":      "",
	"webdav_username":      "",
	"opacity":              0.9,
	"background":           "",
	"uploadCount":          10,
	"themesSelect":         "themes/2024",
	"errorMinute":          1,
	"errorCount":           10,
	"serverWorkers":        1,
	"serverHost":           "0.0.0.0",
	"serverPort":           12345,
	"showAdminAddr":        0,
	"robotsText":           "User-agent: *\nDisallow: /",
	"trustedProxies":       []interface{}{},
}

// Settings 运行时配置（读优先 user_config，fallback 到 defaults）
type Settings struct {
	mu            sync.RWMutex
	defaultConfig map[string]interface{}
	userConfig    map[string]interface{}
}

// NewSettings 创建 Settings 实例
func NewSettings(defaults map[string]interface{}) *Settings {
	return &Settings{
		defaultConfig: defaults,
		userConfig:    make(map[string]interface{}),
	}
}

// RefreshFromDB 从数据库读取最新配置
func (s *Settings) RefreshFromDB(db *gorm.DB) error {
	type KeyValue struct {
		Key   string
		Value string
	}
	var kv KeyValue
	if err := db.Table("key_value").Where("key = ?", "settings").Select("key, value").First(&kv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if kv.Value == "" {
		return nil
	}

	var userConfig map[string]interface{}
	if err := json.Unmarshal([]byte(kv.Value), &userConfig); err != nil {
		return fmt.Errorf("解析配置JSON失败: %w", err)
	}

	s.mu.Lock()
	s.userConfig = userConfig
	s.mu.Unlock()

	return nil
}

// getRaw 获取原始值（不区分类型）
func (s *Settings) getRaw(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.userConfig[key]; ok {
		return v, true
	}
	v, ok := s.defaultConfig[key]
	return v, ok
}

// GetString 获取字符串配置
func (s *Settings) GetString(key string) string {
	v, ok := s.getRaw(key)
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// GetInt 获取整数配置
func (s *Settings) GetInt(key string) int {
	v, ok := s.getRaw(key)
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	default:
		return 0
	}
}

// GetInt64 获取 int64 配置
func (s *Settings) GetInt64(key string) int64 {
	v, ok := s.getRaw(key)
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

// GetBool 获取布尔配置（0/1 或 true/false）
func (s *Settings) GetBool(key string) bool {
	v, ok := s.getRaw(key)
	if !ok {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return false
	}
}

// GetStringSlice 获取字符串数组配置
func (s *Settings) GetStringSlice(key string) []string {
	v, ok := s.getRaw(key)
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case []string:
		return val
	case string:
		// 逗号或分号分隔
		if strings.Contains(val, ";") {
			parts := strings.Split(val, ";")
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}

// GetFloat64 获取浮点数配置
func (s *Settings) GetFloat64(key string) float64 {
	v, ok := s.getRaw(key)
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	default:
		return 0
	}
}

// UpdateFromMap 从 map 更新用户配置
func (s *Settings) UpdateFromMap(updates map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range updates {
		s.userConfig[k] = v
	}
}

// ExportUserConfig 导出用户配置为 JSON 字符串（用于持久化到数据库）
func (s *Settings) ExportUserConfig() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.Marshal(s.userConfig)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetDataRoot 返回 data 目录路径
func (s *Settings) GetDataRoot() string {
	return filepath.Join(s.GetString("base_dir"), "data")
}

var (
	// GlobSettings 全局配置实例
	GlobSettings *Settings
	// InitLogger 确保日志只打印一次
	initLoggerOnce sync.Once
)

// Initialize 初始化全局配置
func Initialize() {
	GlobSettings = NewSettings(DefaultConfig)
}

// LoadFromDB 从数据库加载配置到全局 Settings
func LoadFromDB(db *gorm.DB) error {
	if err := GlobSettings.RefreshFromDB(db); err != nil {
		return err
	}
	initLoggerOnce.Do(func() {
		slog.Info("运行时配置已加载")
	})
	return nil
}
