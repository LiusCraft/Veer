package scheduler

import (
	"math"
	"testing"
	"time"

	"veer/geoip"
	"veer/models"
)

func TestMatchScore(t *testing.T) {
	tests := []struct {
		name           string
		clientProvince string
		clientRegion   string
		clientISP      string
		nodeProvince   string
		nodeRegion     string
		nodeISP        string
		nodeISPList    []string
		expected       float64
	}{
		{"same province same ISP", "广东省", "华南", "电信", "广东省", "华南", "电信", nil, 100},
		{"same province BGP", "广东省", "华南", "电信", "广东省", "华南", "电信", []string{"电信", "联通"}, 95},
		{"same province cross ISP", "广东省", "华南", "电信", "广东省", "华南", "联通", nil, 40},
		{"same region same ISP", "广东省", "华南", "电信", "福建省", "华南", "电信", nil, 80},
		{"same region BGP", "广东省", "华南", "电信", "福建省", "华南", "电信", []string{"电信", "联通"}, 75},
		{"same region cross ISP", "广东省", "华南", "电信", "福建省", "华南", "联通", nil, 30},
		{"cross region same ISP", "广东省", "华南", "电信", "北京市", "华北", "电信", nil, 60},
		{"cross region BGP", "广东省", "华南", "电信", "北京市", "华北", "电信", []string{"电信", "联通"}, 55},
		{"cross region cross ISP", "广东省", "华南", "电信", "北京市", "华北", "联通", nil, 20},
		{"empty node region", "广东省", "华南", "电信", "", "", "电信", nil, 50},
		{"empty isp", "广东省", "华南", "电信", "广东省", "华南", "", nil, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := geoip.MatchScore(tt.clientProvince, tt.clientRegion, tt.clientISP, tt.nodeProvince, tt.nodeRegion, tt.nodeISP, tt.nodeISPList)
			if got != tt.expected {
				t.Errorf("MatchScore() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProvinceToRegion(t *testing.T) {
	tests := []struct {
		province string
		expected string
	}{
		{"广东省", "华南"},
		{"北京市", "华北"},
		{"上海市", "华东"},
		{"江苏省", "华东"},
		{"湖北省", "华中"},
		{"四川省", "西南"},
		{"陕西省", "西北"},
		{"辽宁省", "东北"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.province, func(t *testing.T) {
			got := geoip.ProvinceToRegionName(tt.province)
			if got != tt.expected {
				t.Errorf("ProvinceToRegionName(%q) = %q, want %q", tt.province, got, tt.expected)
			}
		})
	}
}

func TestBandwidthScore(t *testing.T) {
	tests := []struct {
		util     float64
		expected float64
	}{
		{0.0, 1.0},
		{0.5, 0.9285714285714286},
		{0.7, 0.9},
		{0.8, 0.55},
		{0.9, 0.2},
		{0.95, 0.1},
		{1.0, 0.0},
		{1.2, 0.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := bandwidthScore(tt.util)
			if math.Abs(got-tt.expected) > 1e-12 {
				t.Errorf("bandwidthScore(%v) = %v, want %v", tt.util, got, tt.expected)
			}
		})
	}
}

func TestNormalMinMax(t *testing.T) {
	tests := []struct {
		name     string
		vals     []float64
		expected []float64
	}{
		{"normal", []float64{1, 2, 3, 4, 5}, []float64{0.0, 0.25, 0.5, 0.75, 1.0}},
		{"all equal", []float64{3, 3, 3}, []float64{0.5, 0.5, 0.5}},
		{"negative values", []float64{-2, 0, 2}, []float64{0.0, 0.5, 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalMinMax(tt.vals)
			for i := range got {
				if math.Abs(got[i]-tt.expected[i]) > 1e-12 {
					t.Errorf("normalMinMax()[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestNormalMinMaxInv(t *testing.T) {
	vals := []float64{1, 2, 4, 8}
	inv := make([]float64, 4)
	for i, v := range vals {
		inv[i] = 1.0 / v
	}
	expectedInv := normalMinMax(inv)
	got := normalMinMaxInv(vals)
	for i := range got {
		if math.Abs(got[i]-expectedInv[i]) > 1e-12 {
			t.Errorf("normalMinMaxInv()[%d] = %v, want %v", i, got[i], expectedInv[i])
		}
	}
}

func TestCalcBandwidthUtil(t *testing.T) {
	node := models.CdnNode{
		TxBytes1m:    75_000_000,
		RxBytes1m:    150_000_000,
		UplinkMbps:   100,
		DownlinkMbps: 200,
	}
	txUtil, rxUtil := calcBandwidthUtil(node)

	expectedTx := float64(75_000_000) * 8 / 60 / (100 * 1_000_000)
	expectedRx := float64(150_000_000) * 8 / 60 / (200 * 1_000_000)

	if math.Abs(txUtil-expectedTx) > 1e-12 {
		t.Errorf("txUtil = %v, want %v", txUtil, expectedTx)
	}
	if math.Abs(rxUtil-expectedRx) > 1e-12 {
		t.Errorf("rxUtil = %v, want %v", rxUtil, expectedRx)
	}
}

func TestSelectNodeByScore(t *testing.T) {
	nodes := []models.CdnNode{
		{Name: "best", Latency: 5, CPUUsage: 0.1, MemUsage: 0.2, Weight: 10, TxBytes1m: 0, RxBytes1m: 0, UplinkMbps: 1000, DownlinkMbps: 1000},
		{Name: "medium", Latency: 50, CPUUsage: 0.5, MemUsage: 0.6, Weight: 5, TxBytes1m: 0, RxBytes1m: 0, UplinkMbps: 1000, DownlinkMbps: 1000},
		{Name: "worst", Latency: 200, CPUUsage: 0.9, MemUsage: 0.95, Weight: 1, TxBytes1m: 0, RxBytes1m: 0, UplinkMbps: 1000, DownlinkMbps: 1000},
	}

	selected := selectNodeByScore(nodes, GetScoreWeights())
	if selected.Name != "best" {
		t.Errorf("selectNodeByScore() = %q, want %q", selected.Name, "best")
	}
}

func TestSelectWeighted(t *testing.T) {
	nodes := []models.CdnNode{
		{Name: "heavy", Weight: 100},
		{Name: "light", Weight: 1},
	}

	heavyCount := 0
	iterations := 10000
	for i := 0; i < iterations; i++ {
		selected := selectWeighted(nodes)
		if selected.Name == "heavy" {
			heavyCount++
		}
	}

	ratio := float64(heavyCount) / float64(iterations)
	if ratio < 0.95 {
		t.Errorf("heavy node selected %v of the time, want ~0.99", ratio)
	}
}

func TestSelectNodeRoundRobin(t *testing.T) {
	nodes := []models.CdnNode{
		{Name: "A"},
		{Name: "B"},
		{Name: "C"},
	}

	results := make([]string, 6)
	for i := 0; i < 6; i++ {
		selected := selectNode(nodes, "round-robin", 999)
		results[i] = selected.Name
	}

	expected := []string{"A", "B", "C", "A", "B", "C"}
	for i, r := range results {
		if r != expected[i] {
			t.Errorf("round-robin[%d] = %q, want %q", i, r, expected[i])
		}
	}
}

func TestSelectByScoreLevels(t *testing.T) {
	candidates := []nodeCandidate{
		{Node: models.CdnNode{Name: "high1"}, EffectiveScore: 95},
		{Node: models.CdnNode{Name: "high2"}, EffectiveScore: 85},
		{Node: models.CdnNode{Name: "medium"}, EffectiveScore: 65},
		{Node: models.CdnNode{Name: "low"}, EffectiveScore: 25},
	}

	nodes := selectByScoreLevels(candidates)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes from Level 0, got %d", len(nodes))
	}
}

func TestSettlementCost(t *testing.T) {
	tests := []struct {
		clientISP string
		nodeISP   string
		expected  float64
	}{
		{"电信", "电信", 1.0},
		{"电信", "联通", 2.5},
		{"电信", "移动", 1.8},
		{"联通", "电信", 2.0},
		{"联通", "联通", 1.0},
		{"联通", "移动", 1.5},
		{"移动", "电信", 1.5},
		{"移动", "联通", 1.3},
		{"移动", "移动", 1.0},
		{"", "电信", 1.0},
		{"电信", "", 1.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := getSettlementCostFactor(tt.clientISP, tt.nodeISP)
			if got != tt.expected {
				t.Errorf("getSettlementCostFactor(%q, %q) = %v, want %v", tt.clientISP, tt.nodeISP, got, tt.expected)
			}
		})
	}
}

func TestColdStartProtection(t *testing.T) {
	now := time.Now()

	candidates := []nodeCandidate{
		{Node: models.CdnNode{Name: "new", Weight: 100, CreatedAt: now.Add(-1 * time.Minute)}},
		{Node: models.CdnNode{Name: "stale", Weight: 50, LastHeartbeat: now.Add(-10 * time.Minute)}},
		{Node: models.CdnNode{Name: "normal", Weight: 30, CreatedAt: now.Add(-1 * time.Hour), LastHeartbeat: now.Add(-1 * time.Minute)}},
	}

	applyColdStartProtection(candidates)

	if candidates[0].Node.Weight != 10 {
		t.Logf("new node weight = %d", candidates[0].Node.Weight)
	}
	if candidates[1].Node.Weight != 10 {
		t.Logf("stale node weight = %d", candidates[1].Node.Weight)
	}
}

func TestGetNodeRTT(t *testing.T) {
	setNodeRTT(1, 42)
	rtt := GetNodeRTT(1)
	if rtt != 42 {
		t.Errorf("GetNodeRTT() = %d, want 42", rtt)
	}

	rtt = GetNodeRTT(999)
	if rtt != 0 {
		t.Errorf("GetNodeRTT() for unknown = %d, want 0", rtt)
	}
}

func TestNodePerfStats(t *testing.T) {
	stats := NodePerfStats{AvgResponseTimeMs: 150.5, SampleCount: 100}
	setNodePerfStats(1, stats)

	got := GetNodePerfStats(1)
	if got.AvgResponseTimeMs != 150.5 {
		t.Errorf("AvgResponseTimeMs = %v, want 150.5", got.AvgResponseTimeMs)
	}
	if got.SampleCount != 100 {
		t.Errorf("SampleCount = %d, want 100", got.SampleCount)
	}

	got = GetNodePerfStats(999)
	if got.SampleCount != 0 {
		t.Errorf("expected empty stats for unknown node, got %+v", got)
	}
}

func TestPerfStatsInSelectNodeByScore(t *testing.T) {
	setNodePerfStats(1, NodePerfStats{AvgResponseTimeMs: 10, SampleCount: 50})
	setNodePerfStats(2, NodePerfStats{AvgResponseTimeMs: 200, SampleCount: 50})

	nodes := []models.CdnNode{
		{Name: "fast", Latency: 5, CPUUsage: 0.1, MemUsage: 0.2, Weight: 10, UplinkMbps: 1000, DownlinkMbps: 1000},
		{Name: "slow", Latency: 5, CPUUsage: 0.1, MemUsage: 0.2, Weight: 10, UplinkMbps: 1000, DownlinkMbps: 1000},
	}
	nodes[0].ID = 1
	nodes[1].ID = 2

	weights := ScoreWeights{
		Latency:     0.3,
		TxBandwidth: 0.12,
		RxBandwidth: 0.12,
		CPU:         0.15,
		Mem:         0.08,
		Weight:      0.03,
		PerfStats:   0.20,
	}

	selected := selectNodeByScore(nodes, weights)
	if selected.Name != "fast" {
		t.Errorf("expected 'fast' node (avg 10ms), got %q", selected.Name)
	}
}
