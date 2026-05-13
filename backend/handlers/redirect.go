package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	roundRobinCounters = make(map[uint]*int64)
	countersMu         sync.RWMutex
)

func getRoundRobinCounter(ruleID uint) *int64 {
	countersMu.RLock()
	counter, exists := roundRobinCounters[ruleID]
	countersMu.RUnlock()
	if exists {
		return counter
	}

	countersMu.Lock()
	defer countersMu.Unlock()
	if counter, exists = roundRobinCounters[ruleID]; exists {
		return counter
	}
	var zero int64
	roundRobinCounters[ruleID] = &zero
	return &zero
}

type RuleCache struct {
	mu           sync.RWMutex
	rules        map[string][]models.RedirectRule
	nodes        map[uint]models.CdnNode
	clusterNodes map[uint][]models.CdnNode
	ruleClusters []models.RuleCluster
	db           *gorm.DB
}

func NewRuleCache(db *gorm.DB, intervalSeconds int) *RuleCache {
	c := &RuleCache{
		rules:        make(map[string][]models.RedirectRule),
		nodes:        make(map[uint]models.CdnNode),
		clusterNodes: make(map[uint][]models.CdnNode),
		db:           db,
	}
	c.refresh()
	if intervalSeconds > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				c.refresh()
			}
		}()
	}
	return c
}

func (c *RuleCache) refresh() {
	var rules []models.RedirectRule
	if err := c.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		log.Printf("[scheduler] failed to refresh rules: %v", err)
		return
	}

	ruleIDs := make([]uint, len(rules))
	for i, r := range rules {
		ruleIDs[i] = r.ID
	}

	var ruleClusters []models.RuleCluster
	if len(ruleIDs) > 0 {
		c.db.Where("rule_id IN ?", ruleIDs).Find(&ruleClusters)
	}

	clusterIDs := make(map[uint]bool)
	for _, rc := range ruleClusters {
		clusterIDs[rc.ClusterID] = true
	}

	var allNodes []models.CdnNode
	c.db.Where("status = ?", "active").Find(&allNodes)

	nodeMap := make(map[uint]models.CdnNode, len(allNodes))
	clusterNodes := make(map[uint][]models.CdnNode)
	for _, n := range allNodes {
		nodeMap[n.ID] = n
		clusterNodes[n.ClusterID] = append(clusterNodes[n.ClusterID], n)
	}

	ruleMap := make(map[string][]models.RedirectRule)
	for _, r := range rules {
		if r.Domain != "" {
			ruleMap[r.Domain] = append(ruleMap[r.Domain], r)
		}
	}

	c.mu.Lock()
	c.rules = ruleMap
	c.nodes = nodeMap
	c.clusterNodes = clusterNodes
	c.ruleClusters = ruleClusters
	c.mu.Unlock()

	log.Printf("[scheduler] rule cache refreshed: %d rules, %d nodes, %d clusters", len(rules), len(allNodes), len(clusterIDs))
}

func (c *RuleCache) Lookup(domain string) ([]models.RedirectRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rules, ok := c.rules[domain]
	return rules, ok
}

func (c *RuleCache) GetNodes(ids []uint) []models.CdnNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []models.CdnNode
	for _, id := range ids {
		if n, ok := c.nodes[id]; ok {
			result = append(result, n)
		}
	}
	return result
}

func (c *RuleCache) getNodesForRule(ruleID uint) []models.CdnNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []models.CdnNode
	for _, rc := range c.ruleClusters {
		if rc.RuleID == ruleID {
			if nodes, ok := c.clusterNodes[rc.ClusterID]; ok {
				result = append(result, nodes...)
			}
		}
	}
	return result
}

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
				nodes := cache.getNodesForRule(rule.ID)
				if len(nodes) == 0 {
					var nodeIDs []uint
					if err := json.Unmarshal([]byte(rule.NodeIDs), &nodeIDs); err == nil && len(nodeIDs) > 0 {
						nodes = cache.GetNodes(nodeIDs)
					}
				}
				if len(nodes) == 0 {
					continue
				}

				selectedNode := selectNode(nodes, rule.Strategy, rule.ID)

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

func selectNode(nodes []models.CdnNode, strategy string, ruleID uint) models.CdnNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	switch strategy {
	case "weighted":
		return selectWeighted(nodes)
	case "random":
		return nodes[rand.Intn(len(nodes))]
	default:
		counter := getRoundRobinCounter(ruleID)
		idx := atomic.AddInt64(counter, 1) - 1
		return nodes[idx%int64(len(nodes))]
	}
}

func selectWeighted(nodes []models.CdnNode) models.CdnNode {
	totalWeight := 0
	for _, n := range nodes {
		w := n.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, n := range nodes {
		w := n.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			return n
		}
	}
	return nodes[0]
}
