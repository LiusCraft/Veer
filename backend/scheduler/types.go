package scheduler

import (
	"log"
	"sort"
	"sync"
	"time"

	"veer/models"
)

type ClientMatchInfo struct {
	Province       string
	Region         string
	ISP            string
	MatchedByGeoIP bool
}

type nodeCandidate struct {
	Node           models.CdnNode
	EffectiveScore float64
	ClusterID      uint
	Strategy       string
}

type NodePerfStats struct {
	AvgResponseTimeMs float64
	P50ResponseTimeMs float64
	P99ResponseTimeMs float64
	SampleCount       int
}

var (
	nodePerfCache   = make(map[uint]NodePerfStats)
	nodePerfCacheMu sync.RWMutex
)

func GetNodePerfStats(nodeID uint) NodePerfStats {
	nodePerfCacheMu.RLock()
	defer nodePerfCacheMu.RUnlock()
	return nodePerfCache[nodeID]
}

func setNodePerfStats(nodeID uint, stats NodePerfStats) {
	nodePerfCacheMu.Lock()
	defer nodePerfCacheMu.Unlock()
	nodePerfCache[nodeID] = stats
}

func refreshNodePerfStats(cache *RuleCache) {
	now := time.Now()
	since := now.Add(-5 * time.Minute)

	type nodeSamples struct {
		vals []int
	}

	nodeMap := make(map[uint]*nodeSamples)
	var logs []models.AccessLog
	cache.db.Where("created_at > ? AND response_time_ms > 0", since).
		Order("created_at DESC").
		Limit(5000).
		Find(&logs)

	for _, l := range logs {
		ns, ok := nodeMap[l.NodeID]
		if !ok {
			ns = &nodeSamples{}
			nodeMap[l.NodeID] = ns
		}
		if len(ns.vals) < 1000 {
			ns.vals = append(ns.vals, l.ResponseTimeMs)
		}
	}

	nodesUpdated := 0
	for nodeID, ns := range nodeMap {
		if len(ns.vals) == 0 {
			continue
		}
		sorted := make([]int, len(ns.vals))
		copy(sorted, ns.vals)
		sort.Ints(sorted)

		sum := 0
		for _, v := range sorted {
			sum += v
		}

		stats := NodePerfStats{
			AvgResponseTimeMs: float64(sum) / float64(len(sorted)),
			SampleCount:       len(sorted),
		}

		p50Idx := len(sorted) * 50 / 100
		if p50Idx >= len(sorted) {
			p50Idx = len(sorted) - 1
		}
		stats.P50ResponseTimeMs = float64(sorted[p50Idx])

		p99Idx := len(sorted) * 99 / 100
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}
		stats.P99ResponseTimeMs = float64(sorted[p99Idx])

		setNodePerfStats(nodeID, stats)
		nodesUpdated++
	}
	log.Printf("[scheduler] perf stats refreshed: %d nodes with data in last 5min", nodesUpdated)
}
