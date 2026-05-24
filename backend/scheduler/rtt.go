package scheduler

import (
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	rttCache   = make(map[uint]int)
	rttCacheMu sync.RWMutex
)

func GetNodeRTT(nodeID uint) int {
	rttCacheMu.RLock()
	defer rttCacheMu.RUnlock()
	return rttCache[nodeID]
}

func setNodeRTT(nodeID uint, rtt int) {
	rttCacheMu.Lock()
	defer rttCacheMu.Unlock()
	rttCache[nodeID] = rtt
}

func StartRTTProber(nodeURLs map[uint]string, interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		probeAll(nodeURLs)
		for range ticker.C {
			probeAll(nodeURLs)
		}
	}()
	log.Printf("[rtt] RTT prober started, interval=%s, nodes=%d", interval, len(nodeURLs))
}

func probeAll(nodeURLs map[uint]string) {
	var wg sync.WaitGroup
	for id, url := range nodeURLs {
		wg.Add(1)
		go func(nid uint, u string) {
			defer wg.Done()
			probeNode(nid, u)
		}(id, url)
	}
	wg.Wait()
}

func probeNode(nodeID uint, baseURL string) {
	healthURL := baseURL + "/health"
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Get(healthURL)
	if err != nil {
		log.Printf("[rtt] probe failed: node=%d url=%s err=%v", nodeID, healthURL, err)
		return
	}
	defer resp.Body.Close()

	rtt := int(time.Since(start).Milliseconds())
	if rtt < 1 {
		rtt = 1
	}
	setNodeRTT(nodeID, rtt)
	log.Printf("[rtt] probe done: node=%d rtt=%dms", nodeID, rtt)
}
