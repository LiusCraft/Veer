package scheduler

import (
	"testing"
)

func TestGetSettlementCostFactor(t *testing.T) {
	tests := []struct {
		name      string
		clientISP string
		nodeISP   string
		expected  float64
	}{
		{"电信→电信", "电信", "电信", 1.0},
		{"电信→联通", "电信", "联通", 2.5},
		{"电信→移动", "电信", "移动", 1.8},
		{"联通→电信", "联通", "电信", 2.0},
		{"联通→联通", "联通", "联通", 1.0},
		{"联通→移动", "联通", "移动", 1.5},
		{"移动→电信", "移动", "电信", 1.5},
		{"移动→联通", "移动", "联通", 1.3},
		{"移动→移动", "移动", "移动", 1.0},
		{"空值", "", "电信", 1.0},
		{"空值2", "电信", "", 1.0},
		{"未知ISP", "aws", "电信", 1.5},
		{"BGP", "电信", "BGP", 1.0},
		{"未知→未知", "aws", "azure", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSettlementCostFactor(tt.clientISP, tt.nodeISP)
			if got != tt.expected {
				t.Errorf("getSettlementCostFactor(%q, %q) = %v, want %v",
					tt.clientISP, tt.nodeISP, got, tt.expected)
			}
		})
	}
}

func TestGetSettlementTargetISP(t *testing.T) {
	tests := []struct {
		name     string
		nodeISP  string
		ispList  []string
		expected string
	}{
		{"single ISP", "电信", []string{"电信"}, "电信"},
		{"BGP node", "电信", []string{"电信", "联通"}, "BGP"},
		{"empty ISPList", "联通", nil, "联通"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSettlementTargetISP(tt.nodeISP, tt.ispList)
			if got != tt.expected {
				t.Errorf("getSettlementTargetISP(%q, %v) = %q, want %q",
					tt.nodeISP, tt.ispList, got, tt.expected)
			}
		})
	}
}
