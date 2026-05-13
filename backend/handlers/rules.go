// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// keyPattern 验证 key 字段的正则表达式：3-64个字符，小写字母、数字、连字符
var keyPattern = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

// validStrategies 有效的策略列表
var validStrategies = []string{"round-robin", "weighted", "random"}

// validateKey 验证 key 字段格式
//
// 参数:
//   - key: 待验证的 key 值
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateKey(key string) (bool, string) {
	if key == "" {
		return false, "key 不能为空"
	}
	if !keyPattern.MatchString(key) {
		return false, "key 必须为 3-64 个字符，只能包含小写字母、数字和连字符"
	}
	return true, ""
}

// validateDomain 验证 domain 字段格式
//
// 参数:
//   - domain: 待验证的域名值（可为空）
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateDomain(domain string) (bool, string) {
	if domain == "" {
		return true, "" // 允许为空，表示通用规则
	}
	if len(domain) > 253 {
		return false, "domain 最大 253 个字符"
	}
	return true, ""
}

// validateStrategy 验证 strategy 字段
//
// 参数:
//   - strategy: 待验证的策略值
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateStrategy(strategy string) (bool, string) {
	for _, valid := range validStrategies {
		if strategy == valid {
			return true, ""
		}
	}
	return false, "strategy 必须是 round-robin、weighted 或 random 之一"
}

// validateNodeIDs 验证 node_ids 字段是否为有效的 JSON 数组
//
// 参数:
//   - nodeIDs: 待验证的 node_ids 值
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateNodeIDs(nodeIDs string) (bool, string) {
	if nodeIDs == "" {
		return false, "node_ids 不能为空"
	}
	var arr []uint
	if err := json.Unmarshal([]byte(nodeIDs), &arr); err != nil {
		return false, "node_ids 必须是有效的 JSON 数组格式，如 [1,2,3]"
	}
	if len(arr) == 0 {
		return false, "node_ids 必须至少包含一个节点 ID"
	}
	return true, ""
}

// ListRules 处理 GET /api/rules — 列出所有重定向规则
func ListRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rules []models.RedirectRule
		if err := db.Order("created_at desc").Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rules, "total": len(rules)})
	}
}

// CreateRule 处理 POST /api/rules — 创建新的重定向规则
//
// 请求体新增 domain 字段支持
func CreateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Key         string `json:"key" binding:"required"`
			Domain      string `json:"domain"`
			Description string `json:"description"`
			Strategy    string `json:"strategy"`
			NodeIDs     string `json:"node_ids"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 手动校验 key 格式
		if ok, errMsg := validateKey(body.Key); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 校验 domain 格式
		if ok, errMsg := validateDomain(body.Domain); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 设置默认策略并校验
		if body.Strategy == "" {
			body.Strategy = "round-robin"
		}
		if ok, errMsg := validateStrategy(body.Strategy); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 校验 node_ids 格式
		if ok, errMsg := validateNodeIDs(body.NodeIDs); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 创建规则
		rule := models.RedirectRule{
			Key:         body.Key,
			Domain:      body.Domain,
			Description: body.Description,
			Strategy:    body.Strategy,
			NodeIDs:     body.NodeIDs,
		}

		if err := db.Create(&rule).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": rule})
	}
}

// UpdateRule 处理 PUT /api/rules/:id — 更新重定向规则
//
// 请求体新增 domain 字段支持
func UpdateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var rule models.RedirectRule
		if err := db.First(&rule, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}

		var body struct {
			Key         string `json:"key"`
			Domain      string `json:"domain"`
			Description string `json:"description"`
			Strategy    string `json:"strategy"`
			NodeIDs     string `json:"node_ids"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 校验 key 格式（如果提供了）
		if body.Key != "" {
			if ok, errMsg := validateKey(body.Key); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Key = body.Key
		}

		// 校验 domain 格式（如果提供了）
		if body.Domain != "" {
			if ok, errMsg := validateDomain(body.Domain); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Domain = body.Domain
		}

		// 校验 strategy（如果提供了）
		if body.Strategy != "" {
			if ok, errMsg := validateStrategy(body.Strategy); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Strategy = body.Strategy
		}

		// 校验 node_ids 格式（如果提供了）
		if body.NodeIDs != "" {
			if ok, errMsg := validateNodeIDs(body.NodeIDs); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.NodeIDs = body.NodeIDs
		}

		rule.Description = body.Description

		if err := db.Save(&rule).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rule})
	}
}

// DeleteRule 处理 DELETE /api/rules/:id — 删除重定向规则
func DeleteRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		if err := db.Delete(&models.RedirectRule{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
	}
}

// BatchDeleteRules 处理 DELETE /api/rules/batch — 批量删除重定向规则
func BatchDeleteRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IDs []uint `json:"ids" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if len(body.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
			return
		}

		if err := db.Delete(&models.RedirectRule{}, body.IDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "batch delete successful", "deleted": len(body.IDs)})
	}
}
