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

func validateMatchType(mt string) (bool, string) {
	switch mt {
	case "prefix", "exact", "regex":
		return true, ""
	}
	return false, "match_type 必须是 prefix、exact 或 regex 之一"
}

func validateRedirectCode(code int) (bool, string) {
	if code == 301 || code == 302 {
		return true, ""
	}
	return false, "redirect_code 必须是 301 或 302"
}

func ListRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.RedirectRule{})

		ruleType := c.Query("rule_type")
		if ruleType != "" {
			query = query.Where("rule_type = ?", ruleType)
		}

		enabled := c.Query("enabled")
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}

		var rules []models.RedirectRule
		if err := query.Order("priority asc, created_at desc").Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rules, "total": len(rules)})
	}
}

func checkDomainUnique(db *gorm.DB, domain string, excludeID uint) bool {
	query := db.Model(&models.RedirectRule{}).Where("domain = ? AND rule_type = 'domain_routing'", domain)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	var count int64
	query.Count(&count)
	return count == 0
}

func CreateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Enabled       *bool  `json:"enabled"`
			RuleType      string `json:"rule_type"`
			Domain        string `json:"domain"`
			Strategy      string `json:"strategy"`
			NodeIDs       string `json:"node_ids"`
			OriginBaseURL string `json:"origin_base_url"`
			MatchType     string `json:"match_type"`
			SourcePath    string `json:"source_path"`
			TargetHost    string `json:"target_host"`
			TargetPath    string `json:"target_path"`
			RedirectCode  int    `json:"redirect_code"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.RuleType == "" {
			body.RuleType = "domain_routing"
		}
		if body.RuleType != "domain_routing" && body.RuleType != "url_redirect" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rule_type 必须是 domain_routing 或 url_redirect"})
			return
		}

		rule := models.RedirectRule{
			Name:        body.Name,
			Description: body.Description,
			RuleType:    body.RuleType,
		}

		if body.Enabled != nil {
			rule.Enabled = *body.Enabled
		} else {
			rule.Enabled = true
		}

		if body.RuleType == "domain_routing" {
			if ok, errMsg := validateDomain(body.Domain); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			if !checkDomainUnique(db, body.Domain, 0) {
				c.JSON(http.StatusConflict, gin.H{"error": "域名字段冲突：该域名已存在"})
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
			rule.Domain = body.Domain
			rule.Strategy = body.Strategy
			rule.NodeIDs = body.NodeIDs
			rule.OriginBaseURL = body.OriginBaseURL
		} else {
			if body.MatchType == "" {
				body.MatchType = "prefix"
			}
			if ok, errMsg := validateMatchType(body.MatchType); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			if body.SourcePath == "" {
				body.SourcePath = "/"
			}
			if !strings.HasPrefix(body.SourcePath, "/") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "source_path 必须以 / 开头"})
				return
			}
			if body.RedirectCode == 0 {
				body.RedirectCode = 302
			}
			if ok, errMsg := validateRedirectCode(body.RedirectCode); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			rule.Domain = body.Domain
			rule.MatchType = body.MatchType
			rule.SourcePath = body.SourcePath
			rule.TargetHost = body.TargetHost
			rule.TargetPath = body.TargetPath
			rule.RedirectCode = body.RedirectCode
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
			Name          *string `json:"name"`
			Description   *string `json:"description"`
			Enabled       *bool   `json:"enabled"`
			Priority      *int    `json:"priority"`
			Domain        *string `json:"domain"`
			Strategy      *string `json:"strategy"`
			NodeIDs       *string `json:"node_ids"`
			OriginBaseURL *string `json:"origin_base_url"`
			MatchType     *string `json:"match_type"`
			SourcePath    *string `json:"source_path"`
			TargetHost    *string `json:"target_host"`
			TargetPath    *string `json:"target_path"`
			RedirectCode  *int    `json:"redirect_code"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Name != nil {
			rule.Name = *body.Name
		}
		if body.Description != nil {
			rule.Description = *body.Description
		}
		if body.Enabled != nil {
			rule.Enabled = *body.Enabled
		}
		if body.Priority != nil {
			rule.Priority = *body.Priority
		}

		if rule.RuleType == "domain_routing" {
			if body.Domain != nil {
				if ok, errMsg := validateDomain(*body.Domain); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				if *body.Domain != rule.Domain && !checkDomainUnique(db, *body.Domain, rule.ID) {
					c.JSON(http.StatusConflict, gin.H{"error": "域名字段冲突：该域名已存在"})
					return
				}
				rule.Domain = *body.Domain
			}
			if body.Strategy != nil {
				if ok, errMsg := validateStrategy(*body.Strategy); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.Strategy = *body.Strategy
			}
			if body.NodeIDs != nil {
				if ok, errMsg := validateNodeIDs(*body.NodeIDs); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.NodeIDs = *body.NodeIDs
			}
			if body.OriginBaseURL != nil {
				if ok, errMsg := validateOriginURL(*body.OriginBaseURL); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.OriginBaseURL = *body.OriginBaseURL
			}
		} else {
			if body.Domain != nil {
				if ok, errMsg := validateDomain(*body.Domain); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.Domain = *body.Domain
			}
			if body.MatchType != nil {
				if ok, errMsg := validateMatchType(*body.MatchType); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.MatchType = *body.MatchType
			}
			if body.SourcePath != nil {
				if !strings.HasPrefix(*body.SourcePath, "/") {
					c.JSON(http.StatusBadRequest, gin.H{"error": "source_path 必须以 / 开头"})
					return
				}
				rule.SourcePath = *body.SourcePath
			}
			if body.TargetHost != nil {
				rule.TargetHost = *body.TargetHost
			}
			if body.TargetPath != nil {
				rule.TargetPath = *body.TargetPath
			}
			if body.RedirectCode != nil {
				if ok, errMsg := validateRedirectCode(*body.RedirectCode); !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}
				rule.RedirectCode = *body.RedirectCode
			}
		}

		if err := db.Save(&rule).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rule})
	}
}

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

func BatchToggleRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IDs     []uint `json:"ids" binding:"required"`
			Enabled bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if len(body.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
			return
		}

		if err := db.Model(&models.RedirectRule{}).Where("id IN ?", body.IDs).Update("enabled", body.Enabled).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		action := "启用"
		if !body.Enabled {
			action = "停用"
		}
		c.JSON(http.StatusOK, gin.H{"message": "batch " + action + " successful"})
	}
}

func ReorderRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IDs      []uint `json:"ids" binding:"required"`
			RuleType string `json:"rule_type"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if len(body.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
			return
		}

		for i, id := range body.IDs {
			db.Model(&models.RedirectRule{}).Where("id = ?", id).Update("priority", i)
		}
		c.JSON(http.StatusOK, gin.H{"message": "reorder successful"})
	}
}
