package middleware

import (
	"net/http"
	"strings"

	"dongwai_backend/internal/pkg/auth"

	"github.com/gin-gonic/gin"
)

// JWTAuth 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenHeader := c.GetHeader("Authorization")
		if tokenHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证 Token"})
			c.Abort()
			return
		}

		// 支持 "Bearer <token>" 格式
		parts := strings.SplitN(tokenHeader, " ", 2)
		var tokenStr string
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenStr = parts[1]
		} else {
			tokenStr = tokenHeader
		}

		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token 无效或已过期"})
			c.Abort()
			return
		}

		// 将用户信息存入上下文，后续 Handler 可用
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)

		c.Next()
	}
}
