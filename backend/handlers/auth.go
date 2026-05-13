// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"net/http"
	"time"

	"veer/config"
	"veer/middleware"
	"veer/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Username  string `json:"username"`
}

// LoginHandler 处理用户登录请求
// POST /api/auth/login
//
// 参数:
//   - db: 数据库连接
//   - cfg: 应用程序配置
//
// 功能:
//  1. 验证用户名和密码
//  2. 使用 bcrypt 比较密码
//  3. 生成 JWT 令牌并返回
func LoginHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		// 在 admin_users 表中查找用户
		var user models.AdminUser
		if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		// 使用 bcrypt 验证密码
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		// 生成 JWT 令牌
		token, err := middleware.GenerateToken(user.ID, user.Username, cfg.JWT.Secret, cfg.JWT.ExpiryHours)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		// 计算过期时间
		expiresAt := time.Now().Add(time.Duration(cfg.JWT.ExpiryHours) * time.Hour)

		c.JSON(http.StatusOK, LoginResponse{
			Token:     token,
			ExpiresAt: expiresAt.Format(time.RFC3339),
			Username:  user.Username,
		})
	}
}

// LogoutHandler 处理用户登出请求
// POST /api/auth/logout
//
// 注意: JWT 是无状态的，此函数仅返回成功消息
// 前端需要在收到此响应后清除本地存储的 Token
func LogoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}

// GetCurrentUserHandler 获取当前登录用户信息
// GET /api/auth/me
//
// 从 gin.Context 中获取 JWT 中间件设置的用户信息并返回
func GetCurrentUserHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
			return
		}

		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "username not found in context"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":       userID,
			"username": username,
		})
	}
}
