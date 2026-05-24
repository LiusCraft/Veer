package manager

import (
	"log"
	"net/http"
	"sync"
	"time"

	"veer/config"
	"veer/models"

	"gorm.io/gorm"
)

type HealthCheckManager struct {
	db         *gorm.DB
	cfg        *config.HealthCheckConfig
	edgeSecret string
	ticker     *time.Ticker
	stopChan   chan struct{}
	mu         sync.RWMutex
	running    bool
}

func NewHealthCheckManager(db *gorm.DB, cfg *config.HealthCheckConfig, edgeSecret string) *HealthCheckManager {
	return &HealthCheckManager{
		db:         db,
		cfg:        cfg,
		edgeSecret: edgeSecret,
		stopChan:   make(chan struct{}),
	}
}

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

func (m *HealthCheckManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *HealthCheckManager) CheckNow() {
	m.checkAllNodes()
}

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

	m.updateClusterStatus()
}

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

	healthURL := node.URL
	if node.InternalURL != "" {
		healthURL = node.InternalURL
	}
	healthURL += "/health"
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
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

func (m *HealthCheckManager) updateClusterStatus() {
	var clusters []models.Cluster
	if err := m.db.Find(&clusters).Error; err != nil {
		log.Printf("Health check: failed to fetch clusters for status update: %v", err)
		return
	}
	for _, cl := range clusters {
		var stats struct {
			Total  int
			Active int
		}
		m.db.Raw("SELECT COUNT(*) as total FROM node_clusters WHERE cluster_id = ?", cl.ID).Scan(&stats)
		m.db.Raw("SELECT COUNT(*) as active FROM node_clusters nc JOIN cdn_nodes n ON nc.node_id = n.id WHERE nc.cluster_id = ? AND n.status = 'active'", cl.ID).Scan(&stats)

		if stats.Total == 0 {
			continue
		}
		newStatus := cl.Status
		switch {
		case stats.Active == 0:
			newStatus = "inactive"
		case stats.Active < stats.Total:
			newStatus = "degraded"
		default:
			newStatus = "active"
		}

		if newStatus != cl.Status {
			m.db.Model(&cl).Update("status", newStatus)
			log.Printf("Health check: cluster %q status changed: %s → %s", cl.Name, cl.Status, newStatus)
		}
	}
}
