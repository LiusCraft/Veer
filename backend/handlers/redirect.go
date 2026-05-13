// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"encoding/json"
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

// roundRobinCounters 存储每个规则的 round-robin 计数器
var (
	roundRobinCounters = make(map[uint]*int64)
	countersMu         sync.RWMutex
)

// getRoundRobinCounter 返回（或创建）给定规则 ID 的原子计数器
func getRoundRobinCounter(ruleID uint) *int64 {
	countersMu.RLock()
	counter, exists := roundRobinCounters[ruleID]
	countersMu.RUnlock()
	if exists {
		return counter
	}

	countersMu.Lock()
	defer countersMu.Unlock()
	// 获取写锁后再次检查
	if counter, exists = roundRobinCounters[ruleID]; exists {
		return counter
	}
	var zero int64
	roundRobinCounters[ruleID] = &zero
	return &zero
}

// RedirectHandler 处理 GET /r/:ruleKey/*path — 执行 302 重定向到最优 CDN 节点
//
// 支持域名匹配和路径透传：
//   - 优先匹配 ruleKey + domain 相同的规则
//   - 回退到 ruleKey + domain=” 的通用规则
//   - 将剩余路径透传到目标节点 URL
//
// 参数:
//   - db: 数据库连接
//
// 返回:
//   - gin.HandlerFunc: 重定向处理器
func RedirectHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleKey := c.Param("ruleKey")
		pathParam := c.Param("path")

		// 获取请求域名，优先使用 X-Forwarded-Host（支持代理场景）
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}

		// 清理路径：去除前导斜杠
		remainingPath := strings.TrimLeft(pathParam, "/")

		// 查询规则：优先匹配有 domain 的规则，回退到 domain='' 的通用规则
		var rule models.RedirectRule
		err := db.Where("`key` = ?", ruleKey).
			Order("CASE WHEN domain = '' THEN 1 ELSE 0 END, domain DESC").
			First(&rule).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}

		// 如果规则指定了域名且与请求域名不匹配，且没有通用规则，则报错
		if rule.Domain != "" && rule.Domain != host {
			// 查找是否有通用规则
			var genericRule models.RedirectRule
			if err := db.Where("`key` = ? AND domain = ''", ruleKey).First(&genericRule).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "rule not found for this domain"})
				return
			}
			rule = genericRule
		}

		// 解析节点 ID 列表
		var nodeIDs []uint
		if err := json.Unmarshal([]byte(rule.NodeIDs), &nodeIDs); err != nil || len(nodeIDs) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid node configuration"})
			return
		}

		// 获取活跃节点
		var nodes []models.CdnNode
		if err := db.Where("id IN ? AND status = ?", nodeIDs, "active").Find(&nodes).Error; err != nil || len(nodes) == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active nodes available"})
			return
		}

		// 根据策略选择节点
		selectedNode := selectNode(nodes, rule.Strategy, rule.ID)

		// 构建目标 URL：拼接节点 URL 和剩余路径
		var targetURL string
		if remainingPath != "" {
			targetURL = strings.TrimRight(selectedNode.URL, "/") + "/" + remainingPath
		} else {
			targetURL = strings.TrimRight(selectedNode.URL, "/")
		}

		// 异步记录访问日志
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		go func() {
			log := models.AccessLog{
				RuleKey:    ruleKey,
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
			db.Create(&log)
			// 增加命中计数
			db.Model(&rule).UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
		}()

		// 设置缓存控制响应头 (P1-001)
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Vary", "Host")

		// 执行 302 重定向
		c.Redirect(http.StatusFound, targetURL)
	}
}

// selectNode 根据路由策略选择最佳节点
//
// 参数:
//   - nodes: 可用的 CDN 节点列表
//   - strategy: 路由策略 (round-robin/weighted/random)
//   - ruleID: 规则 ID（用于 round-robin 计数器）
//
// 返回:
//   - models.CdnNode: 选中的节点
func selectNode(nodes []models.CdnNode, strategy string, ruleID uint) models.CdnNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	switch strategy {
	case "weighted":
		return selectWeighted(nodes)
	case "random":
		return nodes[rand.Intn(len(nodes))]
	default: // round-robin
		counter := getRoundRobinCounter(ruleID)
		idx := atomic.AddInt64(counter, 1) - 1
		return nodes[idx%int64(len(nodes))]
	}
}

// selectWeighted 执行加权随机选择
//
// 参数:
//   - nodes: CDN 节点列表
//
// 返回:
//   - models.CdnNode: 根据权重随机选中的节点
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
