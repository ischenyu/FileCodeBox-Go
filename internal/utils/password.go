// Package utils 提供密码哈希与验证功能
// 与 Python 版 core/utils.py 的 hash_password / verify_password 保持一致
package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// HashPassword 使用 SHA256 + 随机 salt 哈希密码
// 返回格式: sha256$<hex_salt>$<hex_hash>
func HashPassword(password string) string {
	saltBytes := make([]byte, 16)
	// 使用 crypto/rand 读取随机盐
	rand.Read(saltBytes)
	salt := hex.EncodeToString(saltBytes)
	hash := sha256.Sum256([]byte(salt + password))
	return "sha256$" + salt + "$" + hex.EncodeToString(hash[:])
}

// VerifyPassword 验证密码是否匹配
// 支持: sha256$salt$hash 格式，以及与原文直接比较（兼容历史数据）
func VerifyPassword(password, hashed string) bool {
	if hashed == "" {
		return false
	}

	// 新格式: sha256$salt$hash
	if strings.HasPrefix(hashed, "sha256$") {
		parts := strings.Split(hashed, "$")
		if len(parts) != 3 {
			return false
		}
		salt := parts[1]
		storedHash := parts[2]
		hash := sha256.Sum256([]byte(salt + password))
		computedHash := hex.EncodeToString(hash[:])
		return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedHash)) == 1
	}

	// 旧格式: 明文比较（兼容迁移前数据）
	return subtle.ConstantTimeCompare([]byte(password), []byte(hashed)) == 1
}

// IsPasswordHashed 检查密码是否已经是哈希格式
func IsPasswordHashed(password string) bool {
	return strings.HasPrefix(password, "sha256$") && len(strings.Split(password, "$")) == 3
}
