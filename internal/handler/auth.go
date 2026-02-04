package handler

import (
	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/auth"
	"dongwai_backend/internal/pkg/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Login(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		var admin model.UserRole
		if err := db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}

		if !utils.CheckPassword(req.Password, admin.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}

		// ✅ 修正：使用数据库中真实的 admin.ID (string 类型)
		token, err := auth.GenerateToken(admin.ID, auth.AuthRole(admin.Role))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"role":  admin.Role,
			"id":    admin.ID, // 可选：返回 ID 给前端
		})
	}
}
