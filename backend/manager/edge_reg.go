package manager

import (
	"encoding/json"
	"log"
	"net/http"

	"veer/config"
	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type edgeResponseHeaders struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
}

type EdgeRule struct {
	Domain               string                `json:"domain"`
	OriginBaseURL        string                `json:"origin_base_url"`
	CacheTTLSeconds      *int                  `json:"cache_ttl_seconds,omitempty"`
	CacheControlOverride string                `json:"cache_control_override,omitempty"`
	BypassCache          bool                  `json:"bypass_cache"`
	ResponseHeaders      []edgeResponseHeaders `json:"response_headers,omitempty"`
	LuaScript            string                `json:"lua_script,omitempty"`
}

type EdgeRegisterRequest struct {
	Name        string `json:"name" binding:"required"`
	Region      string `json:"region"`
	PublicURL   string `json:"public_url" binding:"required"`
	InternalURL string `json:"internal_url"`
	Secret      string `json:"secret" binding:"required"`
}

type EdgeRegisterResponse struct {
	NodeID          uint   `json:"node_id"`
	OriginBaseURL   string `json:"origin_base_url"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
	CacheMaxSizeMB  int    `json:"cache_max_size_mb"`
}

func RegisterEdgeHandler(db *gorm.DB, cfg *config.ManagerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req EdgeRegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Secret != cfg.Edge.Secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid secret"})
			return
		}

		if ok, errMsg := validateName(req.Name); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		if ok, errMsg := validateURL(req.PublicURL); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		var node models.CdnNode
		result := db.Where("name = ?", req.Name).First(&node)

		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				node = models.CdnNode{
					Name:          req.Name,
					URL:           req.PublicURL,
					InternalURL:   req.InternalURL,
					Region:        req.Region,
					Status:        "active",
					Weight:        1,
					OriginBaseURL: cfg.Edge.OriginBaseURL,
					CacheTTL:      cfg.Edge.Cache.TTLSeconds,
				}
				if err := db.Create(&node).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register node"})
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
		} else {
			updates := map[string]interface{}{
				"url":          req.PublicURL,
				"internal_url": req.InternalURL,
				"region":       req.Region,
				"status":       "active",
			}
			if err := db.Model(&node).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update node"})
				return
			}
		}

		cacheMaxMB := cfg.Edge.Cache.MaxSizeMB
		if cacheMaxMB <= 0 {
			cacheMaxMB = 512
		}

		resp := EdgeRegisterResponse{
			NodeID:          node.ID,
			OriginBaseURL:   cfg.Edge.OriginBaseURL,
			CacheTTLSeconds: cfg.Edge.Cache.TTLSeconds,
			CacheMaxSizeMB:  cacheMaxMB,
		}

		c.JSON(http.StatusOK, gin.H{"data": resp})
	}
}

func ListEdgeRulesHandler(db *gorm.DB, cfg *config.ManagerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Edge-Secret")
		if secret == "" {
			secret = c.Query("secret")
		}
		if secret != cfg.Edge.Secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid secret"})
			return
		}

		var rules []models.RedirectRule
		if err := db.Where("rule_type = ? AND enabled = ?", "domain_routing", true).Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		edgeRules := make([]EdgeRule, 0, len(rules))
		for _, r := range rules {
			var respHeaders []edgeResponseHeaders
			if r.ResponseHeaderRules != "" {
				if err := json.Unmarshal([]byte(r.ResponseHeaderRules), &respHeaders); err != nil {
					log.Printf("[manager] failed to parse response_header_rules for rule %d (domain=%s): %v",
						r.ID, r.Domain, err)
				}
			}
			edgeRules = append(edgeRules, EdgeRule{
				Domain:               r.Domain,
				OriginBaseURL:        r.OriginBaseURL,
				CacheTTLSeconds:      r.CacheTTLSeconds,
				CacheControlOverride: r.CacheControlOverride,
				BypassCache:          r.BypassCache,
				ResponseHeaders:      respHeaders,
				LuaScript:            r.LuaScript,
			})
		}

		c.JSON(http.StatusOK, gin.H{"data": edgeRules})
	}
}
