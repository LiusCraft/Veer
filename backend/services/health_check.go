// Package services provides business logic services for the Veer system.
package services

import (
	"log"
	"net/http"
	"sync"
	"time"
	"veer/config"
	"veer/models"

	"gorm.io/gorm"
)

// HealthCheckManager 管理所有 CDN 节点的自动健康检测
type HealthCheckManager struct {
	db         *gorm.DB
	cfg        *config.HealthCheckConfig
	edgeSecret string
	ticker     *time.Ticker
	stopChan   chan struct{}
	mu         sync.RWMutex
	running    bool
}

// NewHealthCheckManager 创建新的健康检测管理器
//
// 参数:
//   - db: 数据库连接对象
//   - cfg: 健康检测配置
//   - edgeSecret: 边缘节点管理密钥，用于健康探测鉴权
//
// 返回:
//   - *HealthCheckManager: 健康检测管理器实例
func NewHealthCheckManager(db *gorm.DB, cfg *config.HealthCheckConfig, edgeSecret string) *HealthCheckManager {
	return &HealthCheckManager{
		db:         db,
		cfg:        cfg,
		edgeSecret: edgeSecret,
		stopChan:   make(chan struct{}),
	}
}

// Start 启动后台健康检测 Goroutine
//
// 如果配置未启用或已处于运行状态，则直接返回。
// 启动后会立即执行一次全量检测，然后按配置的间隔周期性执行。
func (m *HealthCheckManager) Start() {
	if m.cfg == nil || !m.cfg.Enabled {
		log.Println("Health check is disabled by configuration")
		return
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	interval := time.Duration(m.cfg.IntervalSeconds) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	m.ticker = time.NewTicker(interval)

	log.Printf("Health check started with interval: %s", interval)

	// 首次立即检测一次
	m.checkAllNodes()

	go func() {
		for {
			select {
			case <-m.ticker.C:
				m.checkAllNodes()
			case <-m.stopChan:
				m.ticker.Stop()
				return
			}
		}
	}()
}

// Stop 停止健康检测
//
// 如果健康检测未处于运行状态，则直接返回。
func (m *HealthCheckManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	log.Println("Health check stopped")
}

// IsRunning 返回健康检测是否正在运行
func (m *HealthCheckManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// CheckNow 手动触发一次健康检测
func (m *HealthCheckManager) CheckNow() {
	m.checkAllNodes()
}

// checkAllNodes 检测所有 active 和 inactive 状态的节点
//
// 并发地对每个节点执行健康检测，等待所有检测完成后返回。
func (m *HealthCheckManager) checkAllNodes() {
	var nodes []models.CdnNode
	if err := m.db.Where("status IN ?", []string{"active", "inactive"}).Find(&nodes).Error; err != nil {
		log.Printf("Health check: failed to fetch nodes: %v", err)
		return
	}

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(node models.CdnNode) {
			defer wg.Done()
			m.checkNode(node)
		}(nodes[i])
	}
	wg.Wait()
}

// checkNode 检测单个节点的健康状态
//
// 执行 HTTP GET 请求到节点的 URL，根据响应结果更新节点状态：
//   - 请求失败或返回 5xx：增加连续失败计数，达到阈值时标记为 inactive
//   - 请求成功：重置失败计数，更新延迟，恢复为 active 状态
func (m *HealthCheckManager) checkNode(node models.CdnNode) {
	timeout := time.Duration(m.cfg.TimeoutSeconds) * time.Second
	if timeout < time.Second {
		timeout = 5 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	healthURL := node.URL + "/health"
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		// 请求构建失败，增加连续失败计数
		m.db.Model(&node).Updates(map[string]interface{}{
			"consecutive_fails": node.ConsecutiveFails + 1,
		})
		newFails := node.ConsecutiveFails + 1
		if m.cfg.FailThreshold > 0 && newFails >= m.cfg.FailThreshold {
			if node.Status != "inactive" {
				m.db.Model(&node).Update("status", "inactive")
				log.Printf("Health check: node %q (%s) marked inactive after %d consecutive failures", node.Name, node.URL, newFails)
			}
		}
		return
	}
	if m.edgeSecret != "" {
		req.Header.Set("X-Edge-Secret", m.edgeSecret)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		// 请求失败，增加连续失败计数
		m.db.Model(&node).Updates(map[string]interface{}{
			"consecutive_fails": node.ConsecutiveFails + 1,
		})

		newFails := node.ConsecutiveFails + 1
		if m.cfg.FailThreshold > 0 && newFails >= m.cfg.FailThreshold {
			if node.Status != "inactive" {
				m.db.Model(&node).Update("status", "inactive")
				log.Printf("Health check: node %q (%s) marked inactive after %d consecutive failures", node.Name, node.URL, newFails)
			}
		} else {
			log.Printf("Health check: node %q (%s) failed (%d/%d): %v", node.Name, node.URL, newFails, m.cfg.FailThreshold, err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		// 5xx 错误也算失败
		m.db.Model(&node).Updates(map[string]interface{}{
			"consecutive_fails": node.ConsecutiveFails + 1,
		})

		newFails := node.ConsecutiveFails + 1
		if m.cfg.FailThreshold > 0 && newFails >= m.cfg.FailThreshold {
			if node.Status != "inactive" {
				m.db.Model(&node).Update("status", "inactive")
				log.Printf("Health check: node %q (%s) marked inactive after %d consecutive 5xx errors", node.Name, node.URL, newFails)
			}
		}
		return
	}

	// 检测成功：重置失败计数，更新延迟，恢复 active 状态
	updates := map[string]interface{}{
		"consecutive_fails": 0,
		"latency":           latency,
	}
	if node.Status != "active" {
		updates["status"] = "active"
		log.Printf("Health check: node %q (%s) recovered to active (latency: %dms)", node.Name, node.URL, latency)
	}
	m.db.Model(&node).Updates(updates)
}
