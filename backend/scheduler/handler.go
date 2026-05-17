package scheduler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func matchPath(sourcePath, requestPath, matchType string) bool {
	switch matchType {
	case "exact":
		return sourcePath == requestPath
	case "prefix":
		return strings.HasPrefix(requestPath, sourcePath)
	case "regex":
		return strings.HasPrefix(requestPath, sourcePath)
	default:
		return false
	}
}

func resolveTargetPath(tmpl string, requestPath, sourcePrefix string) string {
	if tmpl == "" {
		return requestPath
	}
	if strings.Contains(tmpl, "$1") {
		suffix := strings.TrimPrefix(requestPath, sourcePrefix)
		if suffix == "" {
			suffix = "/"
		}
		return strings.ReplaceAll(tmpl, "$1", suffix)
	}
	return tmpl
}

func SchedulerHandler(cache *RuleCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		host = strings.Split(host, ":")[0]

		requestPath := c.Request.URL.Path

		rules, ok := cache.Lookup(host)
		if !ok || len(rules) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no rule found for domain: " + host})
			return
		}

		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}

			if rule.RuleType == "domain_routing" {
				reqRegion := c.GetHeader("X-Region")
				reqISP := c.GetHeader("X-ISP")
				selectedNode, ok := cache.selectNodeForRule(rule.ID, rule.Strategy, reqRegion, reqISP)
				if !ok {
					var nodeIDs []uint
					if err := json.Unmarshal([]byte(rule.NodeIDs), &nodeIDs); err == nil && len(nodeIDs) > 0 {
						nodes := cache.GetNodes(nodeIDs)
						if len(nodes) > 0 {
							selectedNode = selectNode(nodes, rule.Strategy, rule.ID)
							ok = true
						}
					}
				}
				if !ok {
					continue
				}

				remainingPath := strings.TrimLeft(requestPath, "/")
				targetURL := strings.TrimRight(selectedNode.URL, "/")
				targetURL += "/" + host
				if remainingPath != "" {
					targetURL += "/" + remainingPath
				}

				clientIP := c.ClientIP()
				userAgent := c.Request.UserAgent()

				go func() {
					log := models.AccessLog{
						Domain:     host,
						Path:       requestPath,
						NodeID:     selectedNode.ID,
						NodeName:   selectedNode.Name,
						TargetURL:  targetURL,
						ClientIP:   clientIP,
						UserAgent:  userAgent,
						StatusCode: http.StatusFound,
						CreatedAt:  time.Now(),
					}
					cache.db.Create(&log)
					cache.db.Model(&rule).UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
				}()

				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Vary", "Host")
				c.Redirect(http.StatusFound, targetURL)
				return
			}

			if rule.RuleType == "url_redirect" {
				if !matchPath(rule.SourcePath, requestPath, rule.MatchType) {
					continue
				}

				targetPath := resolveTargetPath(rule.TargetPath, requestPath, rule.SourcePath)
				targetHost := rule.TargetHost
				if targetHost == "" {
					targetHost = host
				}

				var targetURL string
				if strings.HasPrefix(targetHost, "http://") || strings.HasPrefix(targetHost, "https://") {
					targetURL = targetHost + targetPath
				} else {
					scheme := "http"
					if c.Request.TLS != nil {
						scheme = "https"
					}
					targetURL = scheme + "://" + targetHost + targetPath
				}

				redirectCode := rule.RedirectCode
				if redirectCode != 301 && redirectCode != 302 {
					redirectCode = 302
				}

				clientIP := c.ClientIP()
				userAgent := c.Request.UserAgent()

				go func() {
					log := models.AccessLog{
						Domain:     host,
						Path:       requestPath,
						TargetURL:  targetURL,
						ClientIP:   clientIP,
						UserAgent:  userAgent,
						StatusCode: redirectCode,
						CreatedAt:  time.Now(),
					}
					cache.db.Create(&log)
					cache.db.Model(&rule).UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
				}()

				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Vary", "Host")
				c.Redirect(redirectCode, targetURL)
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "no matching redirect for: " + host + requestPath})
	}
}
