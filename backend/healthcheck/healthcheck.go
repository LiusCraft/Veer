// Package healthcheck provides proactive health checking for CDN nodes.
//
// 本模块通过后台 goroutine 定期对所有 CDN 节点执行 HTTP HEAD 请求，
// 根据检查结果自动调整节点状态：
//   - 连续失败次数达到阈值时自动标记为 inactive
//   - 节点恢复时自动标记回 active
//
// 使用 sync.RWMutex 保证并发安全。
package healthcheck

import (
	"log"
	"net/http"
	"sync"
	"time"

	"veer/models"

	"gorm.io/gorm"
)

// HealthChecker 健康检查器，管理所有 CDN 节点的主动探测
type HealthChecker struct {
	db            *gorm.DB
	interval      time.Duration // 检查间隔
	failThreshold int           // 连续失败阈值
	timeout       time.Duration // HTTP 请求超时
	client        *http.Client  // HTTP 客户端
	mu            sync.RWMutex  // 保护状态变更的读写锁
	stopCh        chan struct{} // 停止信号通道
}

// NewHealthChecker 创建新的健康检查器实例
//
// 参数:
//   - db: 数据库连接
//   - intervalSeconds: 检查间隔（秒）
//   - failThreshold: 连续失败阈值（默认3次标记为 inactive）
//   - timeoutSeconds: HTTP 超时时间（秒）
//
// 返回:
//   - *HealthChecker: 初始化的健康检查器
func NewHealthChecker(db *gorm.DB, intervalSeconds int, failThreshold int, timeoutSeconds int) *HealthChecker {
	if intervalSeconds <= 0 {
		intervalSeconds = 30 // 默认 30 秒
	}
	if failThreshold <= 0 {
		failThreshold = 3 // 默认 3 次
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5 // 默认 5 秒超时
	}

	return &HealthChecker{
		db:            db,
		interval:      time.Duration(intervalSeconds) * time.Second,
		failThreshold: failThreshold,
		timeout:       time.Duration(timeoutSeconds) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动后台健康检查循环
//
// 在独立的 goroutine 中运行定期健康检查。
// 每轮检查所有 active 或 ConsecutiveFails > 0 的节点。
// 可通过 Stop() 方法终止。
func (hc *HealthChecker) Start() {
	log.Printf("Health checker started: interval=%v, fail_threshold=%d, timeout=%v",
		hc.interval, hc.failThreshold, hc.timeout)

	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.checkAllNodes()
			case <-hc.stopCh:
				log.Println("Health checker stopped")
				return
			}
		}
	}()
}

// Stop 停止后台健康检查循环
func (hc *HealthChecker) Stop() {
	select {
	case hc.stopCh <- struct{}{}:
	default:
		// 已停止或正在停止
	}
}

// checkAllNodes 对所有需要检查的节点执行健康检查
//
// 只检查以下节点（减少不必要的请求）:
//   - 状态为 active 的节点（需要持续监控）
//   - ConsecutiveFails > 0 的节点（可能恢复中）
func (hc *HealthChecker) checkAllNodes() {
	var nodes []models.CdnNode

	// 查询需要检查的节点：active 或有连续失败记录
	if err := hc.db.Where("status = ? OR consecutive_fails > ?", "active", 0).
		Find(&nodes).Error; err != nil {
		log.Printf("Health check: failed to query nodes: %v", err)
		return
	}

	if len(nodes) == 0 {
		return
	}

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(node models.CdnNode) {
			defer wg.Done()
			hc.checkSingleNode(&node)
		}(nodes[i])
	}
	wg.Wait()
}

// checkSingleNde 对单个 CDN 节点执行一次 HTTP HEAD 健康检查
//
// 检查逻辑：
//  1. 发送 HTTP HEAD 请求到节点 URL
//  2. 如果响应状态码 < 500，视为成功：
//     - 重置 ConsecutiveFails 为 0
//     - 更新延迟记录
//     - 如果之前是 inactive 且 ConsecutiveFails >= 阈值，恢复为 active
//  3. 如果请求失败或状态码 >= 500：
//     - ConsecutiveFails +1
//     - 达到阈值时自动标记为 inactive
//
// 使用 sync.RWMutex 保证同一节点的并发安全更新
func (hc *HealthChecker) checkSingleNode(node *models.CdnNode) {
	start := time.Now()

	req, err := http.NewRequest("HEAD", node.URL, nil)
	if err != nil {
		hc.handleFailure(node, -1)
		return
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		hc.handleFailure(node, -1)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	latencyMs := int(latency.Milliseconds())

	// HTTP 状态码 < 500 视为服务可用（允许 4xx 客户端错误）
	if resp.StatusCode < 500 {
		hc.handleSuccess(node, latencyMs)
	} else {
		// 5xx 服务端错误视为不可用
		hc.handleFailure(node, latencyMs)
	}
}

// handleSuccess 处理健康检查成功的情况
//
// 成功时重置连续失败计数器并更新延迟，
// 如果节点之前因健康检查失败被降级，则自动恢复
func (hc *HealthChecker) handleSuccess(node *models.CdnNode, latencyMs int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	wasInactive := node.Status == "inactive"
	failsBeforeReset := node.ConsecutiveFails

	// 重置连续失败计数
	node.ConsecutiveFails = 0
	node.Latency = latencyMs
	node.Status = "active" // 恢复活跃状态

	if err := hc.db.Model(node).Updates(map[string]interface{}{
		"consecutive_fails": 0,
		"latency":           latencyMs,
		"status":            "active",
	}).Error; err != nil {
		log.Printf("Health check: failed to update node %d after success: %v", node.ID, err)
		return
	}

	if wasInactive && failsBeforeReset >= hc.failThreshold {
		log.Printf("Health check: node '%s' (ID:%d) recovered, status -> active, latency=%dms",
			node.Name, node.ID, latencyMs)
	}
}

// handleFailure 处理健康检查失败的情况
//
// 失败时递增 ConsecutiveFails 计数器，
// 达到阈值时自动将节点标记为 inactive
func (hc *HealthChecker) handleFailure(node *models.CdnNode, latencyMs int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	node.ConsecutiveFails++

	if latencyMs >= 0 {
		node.Latency = latencyMs
	}

	updates := map[string]interface{}{
		"consecutive_fails": node.ConsecutiveFails,
	}
	if latencyMs >= 0 {
		updates["latency"] = latencyMs
	}

	// 检查是否达到降级阈值
	if node.ConsecutiveFails >= hc.failThreshold && node.Status != "inactive" {
		node.Status = "inactive"
		updates["status"] = "inactive"

		if err := hc.db.Model(node).Updates(updates).Error; err != nil {
			log.Printf("Health check: failed to update node %d after failure: %v", node.ID, err)
			return
		}
		log.Printf("Health check: node '%s' (ID:%d) marked inactive (consecutive_fails=%d/%d)",
			node.Name, node.ID, node.ConsecutiveFails, hc.failThreshold)
		return
	}

	// 尚未达到阈值，仅更新计数和延迟
	if err := hc.db.Model(node).Updates(updates).Error; err != nil {
		log.Printf("Health check: failed to update node %d after failure: %v", node.ID, err)
	}
}
