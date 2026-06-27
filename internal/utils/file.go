// Package utils 提供安全文件名处理
// 与 Python 版 core/utils.py 的 sanitize_filename 保持一致
package utils

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// 非法文件名字符正则: \ / * ? : " < > | 以及控制字符 \x00-\x1F
	illegalCharRe = regexp.MustCompile(`[\\/*?:"<>|\x00-\x1F]`)
	// 连续下划线正则
	multiUnderscoreRe = regexp.MustCompile(`_+`)
)

// SanitizeFilename 安全处理文件名:
//  1. 剥离路径只保留文件名
//  2. 替换非法字符为下划线
//  3. 替换空格为下划线
//  4. 合并连续下划线
//  5. 去除首尾特殊字符
//  6. 空文件名替换为 "unnamed_file"
//  7. 限制长度 255
func SanitizeFilename(filename string) string {
	// 1. 只保留文件名
	filename = filepath.Base(filename)

	// 2. 替换非法字符
	filename = illegalCharRe.ReplaceAllString(filename, "_")

	// 3. 替换空格
	filename = strings.ReplaceAll(filename, " ", "_")

	// 4. 合并连续下划线
	filename = multiUnderscoreRe.ReplaceAllString(filename, "_")

	// 5. 去除首尾 . _
	filename = strings.Trim(filename, "._")

	// 6. 处理空文件名
	if filename == "" {
		filename = "unnamed_file"
	}

	// 7. 长度限制
	runes := []rune(filename)
	if len(runes) > 255 {
		return string(runes[:255])
	}

	return filename
}

// GetFileURL 生成服务器中转下载 URL
// 与 Python 版 core/utils.py 的 get_file_url 保持一致
// jwtSecret 用于生成下载 token，为空时使用时间戳
func GetFileURL(code, jwtSecret string) string {
	key := generateSelectToken(code, jwtSecret)
	return fmt.Sprintf("/share/download?key=%s&code=%s", key, code)
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
