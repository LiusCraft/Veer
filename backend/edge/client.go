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
	Name      string `json:"name"`
	Region    string `json:"region"`
	PublicURL string `json:"public_url"`
	Secret    string `json:"secret"`
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
	Domain               string `json:"domain"`
	OriginBaseURL        string `json:"origin_base_url"`
	CacheTTLSeconds      *int   `json:"cache_ttl_seconds,omitempty"`
	CacheControlOverride string `json:"cache_control_override,omitempty"`
	BypassCache          bool   `json:"bypass_cache"`
}

type rulesResponse struct {
	Code int                 `json:"code"`
	Data []rulesResponseData `json:"data"`
}

type managerClient struct {
	baseURL string
	secret  string
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

func (mc *managerClient) register(name, region, publicURL string) (*registerResponseData, error) {
	reqBody := registerRequest{
		Name:      name,
		Region:    region,
		PublicURL: publicURL,
		Secret:    mc.secret,
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
	resp, err := mc.register(cfg.Name, cfg.Region, cfg.PublicURL)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Printf("[edge] registered as node ID %d", resp.NodeID)

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
		ruleMap[r.Domain] = domainRule{
			originBaseURL:        r.OriginBaseURL,
			cacheTTLSeconds:      r.CacheTTLSeconds,
			cacheControlOverride: r.CacheControlOverride,
			bypassCache:          r.BypassCache,
		}
	}

	srv.ruleCache.Update(ruleMap)
	log.Printf("[edge] synced %d domain->origin mappings from manager", len(rules))

	return nil
}
