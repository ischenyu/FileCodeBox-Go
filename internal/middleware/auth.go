// Package middleware 提供 JWT 认证中间件
// 与 Python 版 apps/admin/dependencies.py 保持一致
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ischenyu/FileCodeBox-Go/internal/config"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// AdminRequired 验证管理员权限中间件
// 检查 Authorization: Bearer <token> 并验证 JWT 签名和 is_admin 声明
func AdminRequired(cfg *config.Settings) gin.HandlerFunc {
	return func(c *gin.Context) {
		jwtSecret := cfg.GetString("jwt_secret")
		if jwtSecret == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, utils.Error(500, "JWT签名密钥未初始化"))
			return
		}

		// 允许公开的端点（登录接口）
		if c.Request.Method == "POST" && c.Request.URL.Path == "/admin/login" {
			c.Next()
			return
		}

		// 提取 Bearer Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.Error(401, "未授权或授权校验失败"))
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 验证 Token
		payload, err := utils.VerifyToken([]byte(jwtSecret), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.Error(401, err.Error()))
			return
		}

		// 检查 is_admin 声明
		isAdmin, ok := payload["is_admin"].(bool)
		if !ok || !isAdmin {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.Error(401, "未授权或授权校验失败"))
			return
		}

		// 将 payload 和 token 存入上下文
		c.Set("admin_payload", payload)
		c.Set("admin_token_raw", token)
		c.Next()
	}
}

// ShareRequiredLogin 分享上传权限验证中间件
// 当 openUpload=false 时，要求管理员认证才能上传
func ShareRequiredLogin(cfg *config.Settings) gin.HandlerFunc {
	return func(c *gin.Context) {
		openUpload := cfg.GetBool("openUpload")
		if !openUpload {
			jwtSecret := cfg.GetString("jwt_secret")
			if jwtSecret == "" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, utils.Error(500, "JWT签名密钥未初始化"))
				return
			}

			authHeader := c.GetHeader("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				c.AbortWithStatusJSON(http.StatusForbidden, utils.Error(403, "本站未开启游客上传，如需上传请先登录后台"))
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")

			payload, err := utils.VerifyToken([]byte(jwtSecret), token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, utils.Error(403, "认证失败: "+err.Error()))
				return
			}

			isAdmin, ok := payload["is_admin"].(bool)
			if !ok || !isAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, utils.Error(403, "需要管理员权限"))
				return
			}
		}
		c.Next()
	}
}
