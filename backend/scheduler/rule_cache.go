package scheduler

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"veer/geoip"
	"veer/models"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var (
	roundRobinCounters = make(map[uint]*int64)
	countersMu         sync.RWMutex
)

func getRoundRobinCounter(ruleID uint) *int64 {
	countersMu.RLock()
	counter, exists := roundRobinCounters[ruleID]
	countersMu.RUnlock()
	if exists {
		return counter
	}

	countersMu.Lock()
	defer countersMu.Unlock()
	if counter, exists = roundRobinCounters[ruleID]; exists {
		return counter
	}
	var zero int64
	roundRobinCounters[ruleID] = &zero
	return &zero
}

type RuleCache struct {
	mu           sync.RWMutex
	rules        map[string][]models.RedirectRule
	nodes        map[uint]models.CdnNode
	clusterNodes map[uint][]models.CdnNode
	ruleClusters []models.RuleCluster
	clusters     map[uint]models.Cluster
	db           *gorm.DB
}

func NewRuleCache(db *gorm.DB, intervalSeconds int) *RuleCache {
	c := &RuleCache{
		rules:        make(map[string][]models.RedirectRule),
		nodes:        make(map[uint]models.CdnNode),
		clusterNodes: make(map[uint][]models.CdnNode),
		db:           db,
	}
	c.refresh()
	if intervalSeconds > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				c.refresh()
			}
		}()
	}
	return c
}

func (c *RuleCache) refresh() {
	var rules []models.RedirectRule
	if err := c.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		log.Printf("[scheduler] failed to refresh rules: %v", err)
		return
	}

	ruleIDs := make([]uint, len(rules))
	for i, r := range rules {
		ruleIDs[i] = r.ID
	}

	var ruleClusters []models.RuleCluster
	if len(ruleIDs) > 0 {
		c.db.Where("rule_id IN ?", ruleIDs).Find(&ruleClusters)
	}

	clusterIDs := make(map[uint]bool)
	for _, rc := range ruleClusters {
		clusterIDs[rc.ClusterID] = true
	}

	var allNodes []models.CdnNode
	c.db.Where("status = ?", "active").Find(&allNodes)

	nodeMap := make(map[uint]models.CdnNode, len(allNodes))
	for _, n := range allNodes {
		nodeMap[n.ID] = n
	}

	clusterNodes := make(map[uint][]models.CdnNode)
	var nodeClusters []models.NodeCluster
	c.db.Find(&nodeClusters)
	for _, nc := range nodeClusters {
		if n, ok := nodeMap[nc.NodeID]; ok {
			clusterNodes[nc.ClusterID] = append(clusterNodes[nc.ClusterID], n)
		}
	}

	ruleMap := make(map[string][]models.RedirectRule)
	for _, r := range rules {
		if r.Domain != "" {
			ruleMap[r.Domain] = append(ruleMap[r.Domain], r)
		}
	}

	var clusterList []models.Cluster
	clusterIDsSlice := make([]uint, 0, len(clusterIDs))
	for id := range clusterIDs {
		clusterIDsSlice = append(clusterIDsSlice, id)
	}
	if len(clusterIDsSlice) > 0 {
		c.db.Where("id IN ?", clusterIDsSlice).Find(&clusterList)
	}
	clusterMap := make(map[uint]models.Cluster, len(clusterList))
	for _, cl := range clusterList {
		clusterMap[cl.ID] = cl
	}

	c.mu.Lock()
	c.rules = ruleMap
	c.nodes = nodeMap
	c.clusterNodes = clusterNodes
	c.ruleClusters = ruleClusters
	c.clusters = clusterMap
	c.mu.Unlock()

	refreshNodePerfStats(c)

	log.Printf("[scheduler] rule cache refreshed: %d rules, %d nodes, %d clusters", len(rules), len(allNodes), len(clusterIDs))
}

func (c *RuleCache) Lookup(domain string) ([]models.RedirectRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rules, ok := c.rules[domain]
	return rules, ok
}

func (c *RuleCache) GetNodes(ids []uint) []models.CdnNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []models.CdnNode
	for _, id := range ids {
		if n, ok := c.nodes[id]; ok {
			result = append(result, n)
		}
	}
	return result
}

func (c *RuleCache) selectNodeForRule(ruleID uint, ruleStrategy string, client ClientMatchInfo) (models.CdnNode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allCandidates []nodeCandidate

	for _, rc := range c.ruleClusters {
		if rc.RuleID != ruleID {
			continue
		}
		cl, ok := c.clusters[rc.ClusterID]
		if !ok {
			continue
		}
		nodes, ok := c.clusterNodes[rc.ClusterID]
		if !ok || len(nodes) == 0 {
			continue
		}
		strategy := ruleStrategy
		if cl.Strategy != "" {
			strategy = cl.Strategy
		}
		bwPrice := cl.BandwidthPrice
		if bwPrice <= 0 {
			bwPrice = 1.0
		}
		for _, n := range nodes {
			nodeProvince := n.Province
			if nodeProvince == "" {
				nodeProvince = n.Region
			}
			nodeRegion := geoip.ProvinceToRegionName(nodeProvince)
			if nodeRegion == "" {
				nodeRegion = n.Region
			}

			ms := geoip.MatchScore(client.Province, client.Region, client.ISP, nodeProvince, nodeRegion, n.ISP, n.ISPList)

			sameProvince := client.Province != "" && client.Province == nodeProvince
			sameRegion := client.Region != "" && nodeRegion != "" && client.Region == nodeRegion && !sameProvince

			targetISP := getSettlementTargetISP(n.ISP, n.ISPList)
			settlementCost := getSettlementCostFactor(client.ISP, targetISP)
			distanceCost := getDistanceCost(sameProvince, sameRegion)

			costFactor := settlementCost * bwPrice * distanceCost
			if costFactor < 1.0 {
				costFactor = 1.0
			}

			allCandidates = append(allCandidates, nodeCandidate{
				Node:           n,
				EffectiveScore: ms / costFactor,
				ClusterID:      rc.ClusterID,
				Strategy:       strategy,
			})
		}
	}

	if len(allCandidates) == 0 {
		return models.CdnNode{}, false
	}

	applyColdStartProtection(allCandidates)
	applyRTTScore(allCandidates)

	nodes := selectByScoreLevels(allCandidates)
	if len(nodes) == 0 {
		return models.CdnNode{}, false
	}

	strategy := allCandidates[0].Strategy
	for _, s := range allCandidates {
		if s.EffectiveScore >= 80 {
			strategy = s.Strategy
			break
		}
	}

	return selectNode(nodes, strategy, ruleID), true
}

func applyColdStartProtection(candidates []nodeCandidate) {
	warmupPeriod := 5 * time.Minute
	now := time.Now()
	for i := range candidates {
		n := &candidates[i].Node
		if !n.CreatedAt.IsZero() && now.Sub(n.CreatedAt) < warmupPeriod {
			if n.Weight > 1 {
				n.Weight = max(1, n.Weight/10)
			}
		}
		if n.LastHeartbeat.IsZero() || now.Sub(n.LastHeartbeat) > 5*time.Minute {
			n.Weight = max(1, n.Weight/5)
		}
	}
}

func applyRTTScore(candidates []nodeCandidate) {
	for i := range candidates {
		if rtt := GetNodeRTT(candidates[i].Node.ID); rtt > 0 {
			candidates[i].Node.Latency = rtt
		}
	}
}

func selectByScoreLevels(candidates []nodeCandidate) []models.CdnNode {
	levels := []struct {
		threshold float64
		name      string
	}{
		{80, "Level 0 (high match)"},
		{60, "Level 1 (degraded)"},
		{20, "Level 2 (further degraded)"},
		{0, "Level 3 (fallback)"},
	}

	for _, level := range levels {
		var filtered []nodeCandidate
		for _, c := range candidates {
			if level.threshold > 0 && c.EffectiveScore >= level.threshold {
				filtered = append(filtered, c)
			} else if level.threshold == 0 {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			nodes := make([]models.CdnNode, len(filtered))
			for i, c := range filtered {
				nodes[i] = c.Node
			}
			return nodes
		}
	}

	nodes := make([]models.CdnNode, len(candidates))
	for i, c := range candidates {
		nodes[i] = c.Node
	}
	return nodes
}

func selectNode(nodes []models.CdnNode, strategy string, ruleID uint) models.CdnNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	switch strategy {
	case "weighted":
		return selectWeighted(nodes)
	case "score":
		return selectNodeByScore(nodes, GetScoreWeights())
	case "random":
		return nodes[rand.Intn(len(nodes))]
	default:
		counter := getRoundRobinCounter(ruleID)
		idx := atomic.AddInt64(counter, 1) - 1
		return nodes[idx%int64(len(nodes))]
	}
}

type ScoreWeights struct {
	Latency      float64
	TxBandwidth  float64
	RxBandwidth  float64
	CPU          float64
	Mem          float64
	Weight       float64
	PerfStats    float64
	CacheHitRate float64
}

func normalMinMax(vals []float64) []float64 {
	n := len(vals)
	result := make([]float64, n)
	min, max := vals[0], vals[0]
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if max == min {
		for i := range result {
			result[i] = 0.5
		}
		return result
	}
	for i, v := range vals {
		result[i] = (v - min) / (max - min)
	}
	return result
}

func normalMinMaxInv(vals []float64) []float64 {
	inv := make([]float64, len(vals))
	for i, v := range vals {
		if v <= 0 {
			inv[i] = 0.0
		} else {
			inv[i] = 1.0 / v
		}
	}
	return normalMinMax(inv)
}

func bandwidthScore(util float64) float64 {
	switch {
	case util >= 1.0:
		return 0.0
	case util >= 0.9:
		return (1.0 - util) * 2
	case util >= 0.7:
		return 0.9 - (util-0.7)/0.2*0.7
	default:
		return 1.0 - util/0.7*0.1
	}
}

func calcBandwidthUtil(node models.CdnNode) (txUtil, rxUtil float64) {
	txBps := float64(node.TxBytes1m) * 8 / 60
	rxBps := float64(node.RxBytes1m) * 8 / 60
	if node.UplinkMbps > 0 {
		txUtil = txBps / (float64(node.UplinkMbps) * 1_000_000)
	}
	if node.DownlinkMbps > 0 {
		rxUtil = rxBps / (float64(node.DownlinkMbps) * 1_000_000)
	}
	return
}

func selectNodeByScore(nodes []models.CdnNode, weights ScoreWeights) models.CdnNode {
	if len(nodes) == 0 {
		return models.CdnNode{}
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	n := len(nodes)
	latencies := make([]float64, n)
	txUtils := make([]float64, n)
	rxUtils := make([]float64, n)
	cpus := make([]float64, n)
	mems := make([]float64, n)
	weightVals := make([]float64, n)
	perfVals := make([]float64, n)
	cacheHitVals := make([]float64, n)

	for i, node := range nodes {
		latencies[i] = float64(node.Latency)
		txUtils[i], rxUtils[i] = calcBandwidthUtil(node)
		cpus[i] = node.CPUUsage
		mems[i] = node.MemUsage
		weightVals[i] = float64(node.Weight)
		cacheHitVals[i] = node.CacheHitRate
		stats := GetNodePerfStats(node.ID)
		if stats.AvgResponseTimeMs > 0 {
			perfVals[i] = stats.AvgResponseTimeMs
		}
	}

	normLat := normalMinMaxInv(latencies)
	normCPU := normalMinMaxInv(cpus)
	normMem := normalMinMaxInv(mems)
	normW := normalMinMax(weightVals)
	normPerf := normalMinMaxInv(perfVals)
	normCacheHit := normalMinMax(cacheHitVals)

	bestScore := -1.0
	bestIdx := 0
	for i := range nodes {
		score := weights.Latency*normLat[i] +
			weights.TxBandwidth*bandwidthScore(txUtils[i]) +
			weights.RxBandwidth*bandwidthScore(rxUtils[i]) +
			weights.CPU*normCPU[i] +
			weights.Mem*normMem[i] +
			weights.Weight*normW[i] +
			weights.PerfStats*normPerf[i] +
			weights.CacheHitRate*normCacheHit[i]
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return nodes[bestIdx]
}

func GetScoreWeights() ScoreWeights {
	return ScoreWeights{
		Latency:      getScoreWeight("latency", 0.25),
		TxBandwidth:  getScoreWeight("tx_bandwidth", 0.10),
		RxBandwidth:  getScoreWeight("rx_bandwidth", 0.10),
		CPU:          getScoreWeight("cpu", 0.12),
		Mem:          getScoreWeight("mem", 0.06),
		Weight:       getScoreWeight("weight", 0.03),
		PerfStats:    getScoreWeight("perf_stats", 0.18),
		CacheHitRate: getScoreWeight("cache_hit_rate", 0.16),
	}
}

func getScoreWeight(name string, defaultVal float64) float64 {
	key := "scheduling.score_weights." + name
	return viper.GetFloat64(key)
}

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
