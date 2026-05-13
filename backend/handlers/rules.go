package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var validStrategies = []string{"round-robin", "weighted", "random"}

var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`)

func validateDomain(domain string) (bool, string) {
	if domain == "" {
		return false, "domain 不能为空"
	}
	if len(domain) > 253 {
		return false, "domain 最大 253 个字符"
	}
	if !domainPattern.MatchString(domain) {
		return false, "domain 格式无效，应为合法的域名，如 cdn.example.com"
	}
	return true, ""
}

func validateStrategy(strategy string) (bool, string) {
	for _, valid := range validStrategies {
		if strategy == valid {
			return true, ""
		}
	}
	return false, "strategy 必须是 round-robin、weighted 或 random 之一"
}

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

func validateOriginURL(origin string) (bool, string) {
	if origin == "" {
		return true, ""
	}
	if len(origin) > 512 {
		return false, "origin_base_url 最大 512 个字符"
	}
	if origin != "" && !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
		return false, "origin_base_url 必须以 http:// 或 https:// 开头"
	}
	return true, ""
}

func CreateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Domain        string `json:"domain" binding:"required"`
			Description   string `json:"description"`
			Strategy      string `json:"strategy"`
			NodeIDs       string `json:"node_ids"`
			OriginBaseURL string `json:"origin_base_url"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if ok, errMsg := validateDomain(body.Domain); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if body.Strategy == "" {
			body.Strategy = "round-robin"
		}
		if ok, errMsg := validateStrategy(body.Strategy); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if ok, errMsg := validateNodeIDs(body.NodeIDs); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if ok, errMsg := validateOriginURL(body.OriginBaseURL); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		rule := models.RedirectRule{
			Domain:        body.Domain,
			Description:   body.Description,
			Strategy:      body.Strategy,
			NodeIDs:       body.NodeIDs,
			OriginBaseURL: body.OriginBaseURL,
		}

		if err := db.Create(&rule).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": rule})
	}
}

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
			Domain        string `json:"domain"`
			Description   string `json:"description"`
			Strategy      string `json:"strategy"`
			NodeIDs       string `json:"node_ids"`
			OriginBaseURL string `json:"origin_base_url"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Domain != "" {
			if ok, errMsg := validateDomain(body.Domain); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Domain = body.Domain
		}

		if body.Strategy != "" {
			if ok, errMsg := validateStrategy(body.Strategy); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Strategy = body.Strategy
		}

		if body.NodeIDs != "" {
			if ok, errMsg := validateNodeIDs(body.NodeIDs); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.NodeIDs = body.NodeIDs
		}

		if body.OriginBaseURL != rule.OriginBaseURL {
			if ok, errMsg := validateOriginURL(body.OriginBaseURL); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.OriginBaseURL = body.OriginBaseURL
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
