package scheduler

import (
	"log"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

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

type clusterSelectInfo struct {
	clusterID uint
	weight    int
	priority  int
	strategy  string
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

func (c *RuleCache) getNodesForRule(ruleID uint) []models.CdnNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []models.CdnNode
	for _, rc := range c.ruleClusters {
		if rc.RuleID == ruleID {
			if nodes, ok := c.clusterNodes[rc.ClusterID]; ok {
				result = append(result, nodes...)
			}
		}
	}
	return result
}

func (c *RuleCache) selectNodeForRule(ruleID uint, ruleStrategy, region, isp string) (models.CdnNode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var infos []clusterSelectInfo
	for _, rc := range c.ruleClusters {
		if rc.RuleID == ruleID {
			cl, ok := c.clusters[rc.ClusterID]
			if !ok {
				continue
			}
			if !clusterMatches(cl, region, isp) {
				continue
			}
			strategy := ruleStrategy
			if cl.Strategy != "" {
				strategy = cl.Strategy
			}
			infos = append(infos, clusterSelectInfo{
				clusterID: rc.ClusterID,
				weight:    rc.Weight,
				priority:  rc.Priority,
				strategy:  strategy,
			})
		}
	}

	if len(infos) == 0 {
		return models.CdnNode{}, false
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].priority != infos[j].priority {
			return infos[i].priority < infos[j].priority
		}
		return infos[i].clusterID < infos[j].clusterID
	})

	currentPri := infos[0].priority
	start := 0
	for i := 0; i <= len(infos); i++ {
		if i == len(infos) || infos[i].priority != currentPri {
			level := infos[start:i]
			var candidates []clusterSelectInfo
			for _, info := range level {
				nodes, ok := c.clusterNodes[info.clusterID]
				if !ok {
					continue
				}
				filtered := filterNodesByRegionISP(nodes, region, isp)
				if len(filtered) > 0 {
					candidates = append(candidates, info)
				}
			}
			if len(candidates) > 0 {
				selected := selectClusterByWeight(candidates)
				nodes := c.clusterNodes[selected.clusterID]
				filtered := filterNodesByRegionISP(nodes, region, isp)
				return selectNode(filtered, selected.strategy, ruleID), true
			}
			if i < len(infos) {
				currentPri = infos[i].priority
				start = i
			}
		}
	}

	return models.CdnNode{}, false
}

func clusterMatches(cl models.Cluster, region, isp string) bool {
	if region == "" && isp == "" {
		return true
	}
	regionMatch := region == ""
	for _, r := range cl.Region {
		if r == region || r == "其他" {
			regionMatch = true
			break
		}
	}
	ispMatch := isp == ""
	for _, i := range cl.ISP {
		if i == isp || i == "其他" {
			ispMatch = true
			break
		}
	}
	return regionMatch && ispMatch
}

func filterNodesByRegionISP(nodes []models.CdnNode, region, isp string) []models.CdnNode {
	if region == "" && isp == "" {
		return nodes
	}
	var result []models.CdnNode
	for _, n := range nodes {
		if region != "" && n.Region != region && n.Region != "其他" {
			continue
		}
		if isp != "" && n.ISP != isp && n.ISP != "其他" {
			continue
		}
		result = append(result, n)
	}
	return result
}

func selectClusterByWeight(infos []clusterSelectInfo) clusterSelectInfo {
	if len(infos) == 1 {
		return infos[0]
	}
	totalWeight := 0
	for _, info := range infos {
		w := info.weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, info := range infos {
		w := info.weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			return info
		}
	}
	return infos[0]
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

// ScoreWeights holds the weight coefficients for multi-metric scoring.
type ScoreWeights struct {
	Latency     float64
	TxBandwidth float64
	RxBandwidth float64
	CPU         float64
	Mem         float64
	Weight      float64
}

// normalMinMax normalizes values to [0,1] where larger is better.
// Returns 0.5 for all elements when all inputs are equal.
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

// normalMinMaxInv normalizes inverse values to [0,1] where smaller is better.
// Values <= 0 (unprobed/invalid) get the lowest score.
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

// bandwidthScore applies piecewise penalty based on bandwidth utilization.
// 0% → 1.0, 70% → 0.9, 90% → 0.2, 100% → 0.0, >100% → 0.0
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

	for i, node := range nodes {
		latencies[i] = float64(node.Latency)
		txUtils[i], rxUtils[i] = calcBandwidthUtil(node)
		cpus[i] = node.CPUUsage
		mems[i] = node.MemUsage
		weightVals[i] = float64(node.Weight)
	}

	normLat := normalMinMaxInv(latencies)
	normCPU := normalMinMaxInv(cpus)
	normMem := normalMinMaxInv(mems)
	normW := normalMinMax(weightVals)

	bestScore := -1.0
	bestIdx := 0
	for i := range nodes {
		score := weights.Latency*normLat[i] +
			weights.TxBandwidth*bandwidthScore(txUtils[i]) +
			weights.RxBandwidth*bandwidthScore(rxUtils[i]) +
			weights.CPU*normCPU[i] +
			weights.Mem*normMem[i] +
			weights.Weight*normW[i]
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return nodes[bestIdx]
}

// GetScoreWeights returns the score weights from Viper config (env var > config.yaml > default).
func GetScoreWeights() ScoreWeights {
	return ScoreWeights{
		Latency:     getScoreWeight("latency", 0.35),
		TxBandwidth: getScoreWeight("tx_bandwidth", 0.15),
		RxBandwidth: getScoreWeight("rx_bandwidth", 0.15),
		CPU:         getScoreWeight("cpu", 0.20),
		Mem:         getScoreWeight("mem", 0.10),
		Weight:      getScoreWeight("weight", 0.05),
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
