// Package middleware provides HTTP middleware functions for the Veer backend.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"veer/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken 生成 JWT 令牌
//
// 使用指定的密钥和过期时间生成一个新的 JWT 令牌。
//
// 参数:
//   - userID: 用户 ID
//   - username: 用户名
//   - secret: JWT 签名密钥
//   - expiryHours: 令牌过期时间（小时）
//
// 返回:
//   - string: 生成的 JWT 令牌
//   - error: 生成失败时的错误信息
func GenerateToken(userID uint, username string, secret string, expiryHours int) (string, error) {
	// 创建 JWT 声明
	claims := config.JWTClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "veer",
			Subject:   username,
		},
	}

	// 创建令牌对象
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名生成令牌
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// JWTAuthMiddleware JWT 认证中间件
//
// 从 Authorization Header 中提取并验证 Bearer Token。
// 验证通过后，将 user_id 和 username 设置到 gin.Context 中供后续处理函数使用。
// 验证失败或令牌过期时，返回 401 Unauthorized。
//
// 参数:
//   - secret: JWT 签名密钥
//
// 返回:
//   - gin.HandlerFunc: Gin 中间件处理函数
func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Authorization Header 获取令牌
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header is required",
			})
			return
		}

		// 检查 Bearer 前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header format must be Bearer {token}",
			})
			return
		}

		tokenString := parts[1]

		// 解析和验证令牌
		token, err := jwt.ParseWithClaims(tokenString, &config.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			// 验证签名方法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired token: " + err.Error(),
			})
			return
		}

		// 提取声明并验证
		claims, ok := token.Claims.(*config.JWTClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid token claims",
			})
			return
		}

		// 将用户信息设置到 Context 中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		// 继续处理后续请求
		c.Next()
	}
}

// GetUserIDFromContext 从 gin.Context 中获取当前用户的 ID
//
// 参数:
//   - c: Gin 上下文对象
//
// 返回:
//   - uint: 用户 ID
//   - bool: 是否成功获取
func GetUserIDFromContext(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

// GetUsernameFromContext 从 gin.Context 中获取当前用户的用户名
//
// 参数:
//   - c: Gin 上下文对象
//
// 返回:
//   - string: 用户名
//   - bool: 是否成功获取
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}
