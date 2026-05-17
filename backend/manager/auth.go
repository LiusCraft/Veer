package manager

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

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Username  string `json:"username"`
}

func LoginHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		var user models.AdminUser
		if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		token, err := middleware.GenerateToken(user.ID, user.Username, cfg.JWT.Secret, cfg.JWT.ExpiryHours)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		expiresAt := time.Now().Add(time.Duration(cfg.JWT.ExpiryHours) * time.Hour)

		c.JSON(http.StatusOK, LoginResponse{
			Token:     token,
			ExpiresAt: expiresAt.Format(time.RFC3339),
			Username:  user.Username,
		})
	}
}

func LogoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}

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
