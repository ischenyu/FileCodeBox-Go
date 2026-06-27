// Package utils 提供 JWT token 的创建与验证
// 与 Python 版 apps/admin/dependencies.py 的 create_token / verify_token 保持一致
package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// jwtHeader 是手工 JWT 的固定头部
var jwtHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// CreateToken 创建 JWT token
// secret: 签名密钥
// data: 负载数据，会自动添加 exp 过期时间
// expiresIn: 过期时间（秒），默认 30 天
func CreateToken(secret []byte, data map[string]interface{}, expiresIn time.Duration) (string, error) {
	if expiresIn <= 0 {
		expiresIn = 30 * 24 * time.Hour
	}

	// 拷贝 data 并添加 exp
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["exp"] = time.Now().Add(expiresIn).Unix()

	// 编码 payload
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// 签名: HMAC-SHA256(secret, header.payload)
	toSign := jwtHeader + "." + payloadB64
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(toSign))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return toSign + "." + signature, nil
}

// VerifyToken 验证 JWT token 并返回负载数据
func VerifyToken(secret []byte, token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token格式错误")
	}

	headerB64 := parts[0]
	payloadB64 := parts[1]
	signatureB64 := parts[2]

	// 验证签名
	toSign := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(toSign))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signatureB64), []byte(expectedSig)) {
		return nil, errors.New("token签名无效")
	}

	// 解码 payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, errors.New("token payload解码失败")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errors.New("token payload解析失败")
	}

	// 检查过期
	exp, ok := payload["exp"].(float64)
	if !ok {
		return nil, errors.New("token缺失过期时间")
	}
	if time.Unix(int64(exp), 0).Before(time.Now()) {
		return nil, errors.New("token已过期")
	}

	return payload, nil
}
