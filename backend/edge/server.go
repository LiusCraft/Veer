package edge

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"veer/config"
)

type responseHeaderRule struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
}

type domainRule struct {
	originBaseURL        string
	cacheTTLSeconds      *int
	cacheControlOverride string
	bypassCache          bool
	responseHeaders      []responseHeaderRule
	luaScript            string
	luaProto             *lua.FunctionProto // compiled Lua, nil if no script
}

type ruleCache struct {
	mu    sync.RWMutex
	items map[string]domainRule
}

func newRuleCache() *ruleCache {
	return &ruleCache{items: make(map[string]domainRule)}
}

func (rc *ruleCache) Get(domain string) (domainRule, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	rule, ok := rc.items[domain]
	return rule, ok
}

func (rc *ruleCache) Update(m map[string]domainRule) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.items = m
}

type EdgeServer struct {
	cfg       *config.EdgeConfig
	cache     *TieredCache
	ruleCache *ruleCache
	client    *http.Client
	luaPool   *lStatePool
}

func NewEdgeServer(cfg *config.EdgeConfig) *EdgeServer {
	var err error

	tc, err := NewTieredCache(cfg.Cache)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	return &EdgeServer{
		cfg:       cfg,
		cache:     tc,
		ruleCache: newRuleCache(),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		luaPool: newLStatePool(),
	}
}

func (s *EdgeServer) Stop() {
	s.cache.Stop()
}

func (s *EdgeServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/", s.proxyHandler)
	return mux
}

func (s *EdgeServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Service.Host, s.cfg.Service.Port)
	log.Printf("[edge] %s starting on %s (public: %s, cache TTL: %ds)",
		s.cfg.Name, addr, s.cfg.PublicURL, s.cfg.Cache.TTLSeconds)

	server := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}
	return server.ListenAndServe()
}

func (s *EdgeServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Edge-Secret")
	if token == "" || token != s.cfg.Manager.Secret {
		s.proxyHandler(w, r)
		return
	}

	stats := s.cache.Stats()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","node":"%s","region":"%s","cache_entries":%d,"cache_used_mb":%.1f}`,
		s.cfg.Name, s.cfg.Region, stats["entries"], stats["used_mb"])
}

func (s *EdgeServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	domain, resourcePath := extractDomainAndPath(path)
	if domain == "" {
		w.Header().Set("X-ERROR", "no configured")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	rule, ok := s.ruleCache.Get(domain)
	if !ok || rule.originBaseURL == "" {
		w.Header().Set("X-ERROR", "no configured")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	cacheKey := domain + ":" + resourcePath

	defaultTTL := time.Duration(s.cfg.Cache.TTLSeconds) * time.Second
	if rule.cacheTTLSeconds != nil && *rule.cacheTTLSeconds > 0 {
		defaultTTL = time.Duration(*rule.cacheTTLSeconds) * time.Second
	}

	if entry, ok := s.cache.Get(cacheKey); ok {
		entry = s.transformResponse(rule, r, entry)
		for k, v := range entry.Headers {
			w.Header()[k] = v
		}
		applyResponseHeaders(rule.responseHeaders, w)
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(entry.StatusCode)
		if r.Method == http.MethodGet && len(entry.Body) > 0 {
			w.Write(entry.Body)
		}
		return
	}

	if staleEntry, ok := s.cache.GetStale(cacheKey); ok {
		newEntry, err := s.fetchFromOrigin(rule.originBaseURL, resourcePath, staleEntry)
		if err == nil && newEntry.StatusCode == http.StatusNotModified {
			staleEntry.ExpiresAt = time.Now().Add(parseCacheControlTTL(staleEntry.Headers, defaultTTL))
			s.cache.Set(cacheKey, staleEntry)
			respEntry := s.transformResponse(rule, r, staleEntry)
			for k, v := range respEntry.Headers {
				w.Header()[k] = v
			}
			applyResponseHeaders(rule.responseHeaders, w)
			w.Header().Set("X-Cache", "REVALIDATED")
			w.Header().Set("X-Edge", s.cfg.Name)
			w.WriteHeader(respEntry.StatusCode)
			if r.Method == http.MethodGet {
				w.Write(respEntry.Body)
			}
			return
		}
		if err == nil && newEntry.StatusCode < 500 {
			s.cache.Set(cacheKey, newEntry)
			respEntry := s.transformResponse(rule, r, newEntry)
			for k, v := range respEntry.Headers {
				w.Header()[k] = v
			}
			applyResponseHeaders(rule.responseHeaders, w)
			w.Header().Set("X-Cache", "MISS")
			w.Header().Set("X-Edge", s.cfg.Name)
			w.WriteHeader(respEntry.StatusCode)
			if r.Method == http.MethodGet {
				w.Write(respEntry.Body)
			}
			return
		}
	}

	if rule.bypassCache {
		entry, err := s.fetchFromOrigin(rule.originBaseURL, resourcePath, nil)
		if err != nil {
			log.Printf("[edge] origin fetch failed: path=%s err=%v", resourcePath, err)
			w.Header().Set("X-Cache", "BYPASS")
			w.Header().Set("X-Edge", s.cfg.Name)
			http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
			return
		}
		entry = s.transformResponse(rule, r, entry)
		for k, v := range entry.Headers {
			w.Header()[k] = v
		}
		applyResponseHeaders(rule.responseHeaders, w)
		w.Header().Set("X-Cache", "BYPASS")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(entry.StatusCode)
		if r.Method == http.MethodGet {
			w.Write(entry.Body)
		}
		return
	}

	entry, err := s.fetchFromOrigin(rule.originBaseURL, resourcePath, nil)
	if err != nil {
		log.Printf("[edge] origin fetch failed: path=%s err=%v", resourcePath, err)
		w.Header().Set("X-Cache", "ERROR")
		w.Header().Set("X-Edge", s.cfg.Name)
		http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
		return
	}

	if rule.cacheControlOverride != "" {
		entry.Headers.Set("Cache-Control", rule.cacheControlOverride)
	}
	entry.ExpiresAt = time.Now().Add(parseCacheControlTTL(entry.Headers, defaultTTL))

	if entry.StatusCode < 500 {
		s.cache.Set(cacheKey, entry)
	}

	entry = s.transformResponse(rule, r, entry)
	for k, v := range entry.Headers {
		w.Header()[k] = v
	}
	applyResponseHeaders(rule.responseHeaders, w)
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("X-Edge", s.cfg.Name)
	w.WriteHeader(entry.StatusCode)
	if r.Method == http.MethodGet {
		w.Write(entry.Body)
	}
}

func extractDomainAndPath(fullPath string) (domain, resourcePath string) {
	fullPath = strings.TrimPrefix(fullPath, "/")
	parts := strings.SplitN(fullPath, "/", 2)
	if len(parts) > 0 && parts[0] != "" {
		domain = parts[0]
		if len(parts) > 1 && parts[1] != "" {
			resourcePath = "/" + parts[1]
		} else {
			resourcePath = "/"
		}
	} else {
		resourcePath = "/"
	}
	return
}

func buildScriptReq(r *http.Request) *scriptReq {
	headers := make(map[string]string)
	for k := range r.Header {
		headers[strings.ToLower(k)] = r.Header.Get(k)
	}
	return &scriptReq{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: headers,
	}
}

func (s *EdgeServer) transformResponse(rule domainRule, r *http.Request, entry *CacheEntry) *CacheEntry {
	if rule.luaProto == nil {
		return entry
	}
	// Copy to avoid mutating the cached entry.
	e := *entry
	entry = &e

	headers := make(map[string]string)
	for k := range entry.Headers {
		headers[strings.ToLower(k)] = entry.Headers.Get(k)
	}

	sReq := buildScriptReq(r)
	sResp := &scriptResp{
		StatusCode: entry.StatusCode,
		Headers:    headers,
		Body:       string(entry.Body),
	}

	se := &scriptEngine{proto: rule.luaProto}
	applyLuaScript(se, s.luaPool, sReq, sResp, 0)

	// Apply changes back
	entry.StatusCode = sResp.StatusCode
	entry.Body = []byte(sResp.Body)
	entry.Headers = make(http.Header)
	for k, v := range sResp.Headers {
		entry.Headers.Set(k, v)
	}
	return entry
}

func applyResponseHeaders(rules []responseHeaderRule, w http.ResponseWriter) {
	for _, h := range rules {
		switch h.Action {
		case "set":
			w.Header().Set(h.Name, h.Value)
		case "add":
			w.Header().Add(h.Name, h.Value)
		case "remove":
			w.Header().Del(h.Name)
		}
	}
}

func (s *EdgeServer) fetchFromOrigin(originBaseURL, path string, staleEntry *CacheEntry) (*CacheEntry, error) {
	targetURL := strings.TrimRight(originBaseURL, "/") + path

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if staleEntry != nil {
		if etag := staleEntry.Headers.Get("ETag"); etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if lm := staleEntry.Headers.Get("Last-Modified"); lm != "" {
			req.Header.Set("If-Modified-Since", lm)
		}
		if req.Header.Get("If-None-Match") != "" || req.Header.Get("If-Modified-Since") != "" {
			log.Printf("[edge] conditional fetch: %s", targetURL)
		}
	}

	if staleEntry == nil {
		log.Printf("[edge] fetching from origin: %s", targetURL)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("origin unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &CacheEntry{
			StatusCode: http.StatusNotModified,
			Headers:    make(http.Header),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read origin response: %w", err)
	}

	entry := &CacheEntry{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Headers:     make(http.Header),
		Body:        body,
		CachedAt:    time.Now(),
		HitCount:    0,
	}

	for _, k := range []string{"Content-Type", "Content-Length", "Cache-Control", "ETag", "Last-Modified", "Accept-Ranges"} {
		if v := resp.Header.Get(k); v != "" {
			entry.Headers.Set(k, v)
		}
	}

	return entry, nil
}
