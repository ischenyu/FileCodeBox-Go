// Package utils 提供随机提取码生成
// 与 Python 版 core/utils.py 的 get_random_num / get_random_string 保持一致
package utils

import (
	"crypto/rand"
	"math/big"
)

const randomStringChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomNum 生成 5 位随机数字 (10000 - 99999)
func RandomNum() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(90000))
	if err != nil {
		return "", err
	}
	return n.Add(n, big.NewInt(10000)).String(), nil
}

// RandomString 生成 5 位随机大写字母+数字字符串
func RandomString() (string, error) {
	result := make([]byte, 5)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomStringChars))))
		if err != nil {
			return "", err
		}
		result[i] = randomStringChars[n.Int64()]
	}
	return string(result), nil
}

// RandomCode 根据 style 生成随机提取码
// style: "number" → 5 位数字; "secret"/"string" → 5 位大写字母+数字
func RandomCode(style string) (string, error) {
	switch style {
	case "secret", "string":
		return RandomString()
	default:
		return RandomNum()
	}
}
