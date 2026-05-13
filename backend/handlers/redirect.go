package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/url"
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

// RuleCache 缓存 domain → RedirectRule 的映射，定时从 DB 刷新
type RuleCache struct {
	mu    sync.RWMutex
	rules map[string]models.RedirectRule
	nodes map[uint]models.CdnNode
	db    *gorm.DB
}

// NewRuleCache 创建并启动规则缓存
func NewRuleCache(db *gorm.DB, intervalSeconds int) *RuleCache {
	c := &RuleCache{
		rules: make(map[string]models.RedirectRule),
		nodes: make(map[uint]models.CdnNode),
		db:    db,
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
	if err := c.db.Find(&rules).Error; err != nil {
		log.Printf("[scheduler] failed to refresh rules: %v", err)
		return
	}

	var allNodes []models.CdnNode
	if err := c.db.Where("status = ?", "active").Find(&allNodes).Error; err != nil {
		log.Printf("[scheduler] failed to refresh nodes: %v", err)
		return
	}

	nodeMap := make(map[uint]models.CdnNode, len(allNodes))
	for _, n := range allNodes {
		nodeMap[n.ID] = n
	}

	ruleMap := make(map[string]models.RedirectRule, len(rules))
	for _, r := range rules {
		if r.Domain != "" {
			ruleMap[r.Domain] = r
		}
	}

	c.mu.Lock()
	c.rules = ruleMap
	c.nodes = nodeMap
	c.mu.Unlock()

	log.Printf("[scheduler] rule cache refreshed: %d rules, %d nodes", len(rules), len(allNodes))
}

// Lookup 根据域名查找规则
func (c *RuleCache) Lookup(domain string) (models.RedirectRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rule, ok := c.rules[domain]
	return rule, ok
}

// GetNodes 批量获取节点
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

// SchedulerHandler 处理调度请求 — 根据 Host 域名匹配规则并 302 重定向
//
// 工作流:
//  1. 从 Host 头提取域名
//  2. 从缓存查找匹配的 RedirectRule
//  3. 根据策略选择 CDN 节点
//  4. 拼接目标 URL 后 302 重定向
func SchedulerHandler(cache *RuleCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		host = strings.Split(host, ":")[0]

		path := c.Request.URL.Path

		rule, ok := cache.Lookup(host)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "no rule found for domain: " + host})
			return
		}

		var nodeIDs []uint
		if err := json.Unmarshal([]byte(rule.NodeIDs), &nodeIDs); err != nil || len(nodeIDs) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid node configuration"})
			return
		}

		nodes := cache.GetNodes(nodeIDs)
		if len(nodes) == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active nodes available"})
			return
		}

		selectedNode := selectNode(nodes, rule.Strategy, rule.ID)

		remainingPath := strings.TrimLeft(path, "/")
		targetURL := strings.TrimRight(selectedNode.URL, "/")
		targetURL += "/" + url.PathEscape(host)
		if remainingPath != "" {
			targetURL += "/" + remainingPath
		}

		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		go func() {
			log := models.AccessLog{
				Domain:     host,
				Path:       remainingPath,
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
