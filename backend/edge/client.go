package edge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"veer/config"
)

type registerRequest struct {
	Name         string `json:"name"`
	Region       string `json:"region"`
	PublicURL    string `json:"public_url"`
	InternalURL  string `json:"internal_url,omitempty"`
	Secret       string `json:"secret"`
	CPUCores     int    `json:"cpu_cores"`
	MemoryMB     int64  `json:"memory_mb"`
	DiskSizeMB   int64  `json:"disk_size_mb"`
	UplinkMbps   int    `json:"uplink_mbps"`
	DownlinkMbps int    `json:"downlink_mbps"`
}

type registerResponseData struct {
	NodeID          uint   `json:"node_id"`
	OriginBaseURL   string `json:"origin_base_url"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
	CacheMaxSizeMB  int    `json:"cache_max_size_mb"`
}

type registerResponse struct {
	Code int                  `json:"code"`
	Data registerResponseData `json:"data"`
}

type rulesResponseData struct {
	Domain               string               `json:"domain"`
	OriginBaseURL        string               `json:"origin_base_url"`
	CacheTTLSeconds      *int                 `json:"cache_ttl_seconds,omitempty"`
	CacheControlOverride string               `json:"cache_control_override,omitempty"`
	BypassCache          bool                 `json:"bypass_cache"`
	ResponseHeaders      []responseHeaderRule `json:"response_headers,omitempty"`
	RequestHeaders       []responseHeaderRule `json:"request_headers,omitempty"`
	RewriteFrom          string               `json:"rewrite_from,omitempty"`
	RewriteTo            string               `json:"rewrite_to,omitempty"`
	LuaScript            string               `json:"lua_script,omitempty"`
	ScriptTimeoutMs      *int                 `json:"script_timeout_ms,omitempty"`
}

type rulesResponse struct {
	Code int                 `json:"code"`
	Data []rulesResponseData `json:"data"`
}

type heartbeatRequest struct {
	CPUUsage         float64 `json:"cpu_usage"`
	MemUsage         float64 `json:"mem_usage"`
	DiskUsage        float64 `json:"disk_usage"`
	LoadAvg          float64 `json:"load_avg"`
	RequestCount1m   int64   `json:"request_count_1m"`
	BandwidthBytes1m int64   `json:"bandwidth_bytes_1m"`
	TxBytes1m        int64   `json:"tx_bytes_1m"`
	RxBytes1m        int64   `json:"rx_bytes_1m"`
}

type managerClient struct {
	baseURL string
	secret  string
	nodeID  uint
	client  *http.Client
}

func newManagerClient(mgr config.EdgeManagerConfig) *managerClient {
	return &managerClient{
		baseURL: mgr.URL,
		secret:  mgr.Secret,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (mc *managerClient) sendHeartbeat(m systemMetrics) error {
	body := heartbeatRequest{
		CPUUsage:  m.CPUUsage,
		MemUsage:  m.MemUsage,
		DiskUsage: m.DiskUsage,
		LoadAvg:   m.LoadAvg,
		TxBytes1m: m.TxBytes,
		RxBytes1m: m.RxBytes,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	url := fmt.Sprintf("%s/api/nodes/%d/heartbeat", mc.baseURL, mc.nodeID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Edge-Secret", mc.secret)

	resp, err := mc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (mc *managerClient) register(name, region, publicURL, internalURL string, cpuCores int, memoryMB, diskSizeMB int64, uplinkMbps, downlinkMbps int) (*registerResponseData, error) {
	reqBody := registerRequest{
		Name:         name,
		Region:       region,
		PublicURL:    publicURL,
		InternalURL:  internalURL,
		Secret:       mc.secret,
		CPUCores:     cpuCores,
		MemoryMB:     memoryMB,
		DiskSizeMB:   diskSizeMB,
		UplinkMbps:   uplinkMbps,
		DownlinkMbps: downlinkMbps,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := mc.baseURL + "/api/edge/register"
	resp, err := mc.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to call manager: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manager response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manager returned status %d: %s", resp.StatusCode, string(body))
	}

	var result registerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse manager response: %w", err)
	}

	return &result.Data, nil
}

func (mc *managerClient) fetchRules() ([]rulesResponseData, error) {
	req, err := http.NewRequest("GET", mc.baseURL+"/api/edge/rules", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Edge-Secret", mc.secret)

	resp, err := mc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manager returned status %d: %s", resp.StatusCode, string(body))
	}

	var result rulesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse rules response: %w", err)
	}

	return result.Data, nil
}

func RegisterWithManager(cfg *config.EdgeConfig) error {
	log.Printf("[edge] registering with manager at %s ...", cfg.Manager.URL)

	mc := newManagerClient(cfg.Manager)
	hw := detectHardware(cfg.Cache.Disk.Path)
	resp, err := mc.register(cfg.Name, cfg.Region, cfg.PublicURL, cfg.InternalURL,
		hw.CPUCores, hw.MemoryMB, hw.DiskSizeMB, cfg.UplinkMbps, cfg.DownlinkMbps)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Printf("[edge] registered as node ID %d", resp.NodeID)
	cfg.NodeID = resp.NodeID
	mc.nodeID = resp.NodeID

	if resp.OriginBaseURL != "" {
		cfg.OriginBaseURL = resp.OriginBaseURL
		log.Printf("[edge] origin set from manager: %s", resp.OriginBaseURL)
	}
	if resp.CacheTTLSeconds > 0 {
		cfg.Cache.TTLSeconds = resp.CacheTTLSeconds
	}
	if resp.CacheMaxSizeMB > 0 {
		cfg.Cache.MaxSizeMB = resp.CacheMaxSizeMB
	}

	log.Printf("[edge] config synced from manager (origin=%s, cache_ttl=%ds, cache_max=%dMB)",
		cfg.OriginBaseURL, cfg.Cache.TTLSeconds, cfg.Cache.MaxSizeMB)

	return nil
}

func SyncRules(srv *EdgeServer) error {
	if srv.cfg.Manager.URL == "" {
		return nil
	}

	mc := newManagerClient(srv.cfg.Manager)
	rules, err := mc.fetchRules()
	if err != nil {
		return fmt.Errorf("failed to sync rules: %w", err)
	}

	ruleMap := make(map[string]domainRule, len(rules))
	for _, r := range rules {
		dr := domainRule{
			originBaseURL:        r.OriginBaseURL,
			cacheTTLSeconds:      r.CacheTTLSeconds,
			cacheControlOverride: r.CacheControlOverride,
			bypassCache:          r.BypassCache,
			responseHeaders:      r.ResponseHeaders,
			requestHeaders:       r.RequestHeaders,
			rewriteFrom:          r.RewriteFrom,
			rewriteTo:            r.RewriteTo,
			luaScript:            r.LuaScript,
			scriptTimeoutMs:      r.ScriptTimeoutMs,
		}
		if r.LuaScript != "" {
			se := NewScriptEngine(r.LuaScript)
			if se != nil {
				dr.luaProto = se.proto
			}
		}
		ruleMap[r.Domain] = dr
	}

	srv.ruleCache.Update(ruleMap)
	log.Printf("[edge] synced %d domain->origin mappings from manager", len(rules))

	return nil
}

func StartHeartbeatLoop(cfg *config.EdgeConfig) {
	if cfg.Manager.URL == "" || cfg.NodeID == 0 {
		log.Println("[edge] heartbeat loop skipped (no manager or node ID)")
		return
	}

	mc := newManagerClient(cfg.Manager)
	mc.nodeID = cfg.NodeID
	collector := newMetricsCollector(cfg.Cache.Disk.Path)

	log.Printf("[edge] heartbeat loop started (interval=30s, node_id=%d)", cfg.NodeID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// send first heartbeat immediately
	m := collector.collect()
	if err := mc.sendHeartbeat(m); err != nil {
		log.Printf("[edge] initial heartbeat failed: %v", err)
	} else {
		log.Printf("[edge] heartbeat sent (cpu=%.1f%%, mem=%.1f%%, disk=%.1f%%, load=%.1f)",
			m.CPUUsage, m.MemUsage, m.DiskUsage, m.LoadAvg)
	}

	for range ticker.C {
		m := collector.collect()
		if err := mc.sendHeartbeat(m); err != nil {
			log.Printf("[edge] heartbeat failed: %v", err)
		} else {
			log.Printf("[edge] heartbeat sent (cpu=%.1f%%, mem=%.1f%%, disk=%.1f%%, load=%.1f)",
				m.CPUUsage, m.MemUsage, m.DiskUsage, m.LoadAvg)
		}
	}
}
