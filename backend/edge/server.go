package edge

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"veer/config"
)

type domainRule struct {
	originBaseURL string
}

type ruleCache struct {
	mu    sync.RWMutex
	items map[string]string
}

func newRuleCache() *ruleCache {
	return &ruleCache{items: make(map[string]string)}
}

func (rc *ruleCache) Get(domain string) (string, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	origin, ok := rc.items[domain]
	return origin, ok
}

func (rc *ruleCache) Update(m map[string]string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.items = m
}

type EdgeServer struct {
	cfg       *config.EdgeConfig
	cache     *MemoryCache
	ruleCache *ruleCache
	client    *http.Client
}

func NewEdgeServer(cfg *config.EdgeConfig) *EdgeServer {
	return &EdgeServer{
		cfg:       cfg,
		cache:     NewMemoryCache(cfg.Cache),
		ruleCache: newRuleCache(),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
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
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
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

	ruleOrigin, ok := s.ruleCache.Get(domain)
	if !ok || ruleOrigin == "" {
		w.Header().Set("X-ERROR", "no configured")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	cacheKey := domain + ":" + resourcePath

	if entry, ok := s.cache.Get(cacheKey); ok {
		for k, v := range entry.Headers {
			w.Header()[k] = v
		}
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("X-Edge", s.cfg.Name)
		w.WriteHeader(entry.StatusCode)
		if r.Method == http.MethodGet {
			w.Write(entry.Body)
		}
		return
	}

	entry, err := s.fetchFromOrigin(ruleOrigin, resourcePath)
	if err != nil {
		log.Printf("[edge] origin fetch failed: path=%s err=%v", resourcePath, err)
		w.Header().Set("X-Cache", "ERROR")
		w.Header().Set("X-Edge", s.cfg.Name)
		http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
		return
	}

	if entry.StatusCode < 500 {
		s.cache.Set(cacheKey, entry)
	}

	for k, v := range entry.Headers {
		w.Header()[k] = v
	}
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

func (s *EdgeServer) fetchFromOrigin(originBaseURL, path string) (*CacheEntry, error) {
	targetURL := strings.TrimRight(originBaseURL, "/") + path
	log.Printf("[edge] fetching from origin: %s", targetURL)

	resp, err := s.client.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("origin unreachable: %w", err)
	}
	defer resp.Body.Close()

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
