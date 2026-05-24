package geoip

import (
	"testing"
)

func TestProvinceToRegionName(t *testing.T) {
	tests := []struct {
		province string
		expected string
	}{
		{"广东省", "华南"},
		{"北京市", "华北"},
		{"上海市", "华东"},
		{"江苏省", "华东"},
		{"浙江省", "华东"},
		{"福建省", "华东"},
		{"湖北省", "华中"},
		{"湖南省", "华中"},
		{"四川省", "西南"},
		{"陕西省", "西北"},
		{"辽宁省", "东北"},
		{"香港特别行政区", "华南"},
		{"台湾省", "华东"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.province, func(t *testing.T) {
			got := ProvinceToRegionName(tt.province)
			if got != tt.expected {
				t.Errorf("ProvinceToRegionName(%q) = %q, want %q", tt.province, got, tt.expected)
			}
		})
	}
}

func TestIsBGPNode(t *testing.T) {
	tests := []struct {
		name     string
		ispList  []string
		expected bool
	}{
		{"single ISP", []string{"电信"}, false},
		{"multiple ISP", []string{"电信", "联通"}, true},
		{"empty list", nil, false},
		{"three ISPs", []string{"电信", "联通", "移动"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBGPNode(tt.ispList)
			if got != tt.expected {
				t.Errorf("IsBGPNode(%v) = %v, want %v", tt.ispList, got, tt.expected)
			}
		})
	}
}

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
		{"empty node ignores region", "广东省", "华南", "电信", "", "", "", nil, 50},
		{"empty isp ignores isp", "广东省", "华南", "电信", "广东省", "华南", "", nil, 50},
		{"BGP is prioritized", "广东省", "华南", "联通", "广东省", "华南", "电信", []string{"电信", "联通"}, 95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchScore(tt.clientProvince, tt.clientRegion, tt.clientISP, tt.nodeProvince, tt.nodeRegion, tt.nodeISP, tt.nodeISPList)
			if got != tt.expected {
				t.Errorf("MatchScore() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNormalizeISP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"电信", "电信"},
		{"中国电信", "电信"},
		{"中国电信集团公司", "电信"},
		{"联通", "联通"},
		{"中国联通", "联通"},
		{"网通", "联通"},
		{"移动", "移动"},
		{"中国移动", "移动"},
		{"aws", "其他"},
		{"azure", "其他"},
		{"", "其他"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeISP(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeISP(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNoopGeoIP(t *testing.T) {
	g := &NoopGeoIP{}
	result, err := g.Lookup("1.2.3.4")
	if err != nil {
		t.Errorf("NoopGeoIP.Lookup() error = %v", err)
	}
	if result == nil {
		t.Fatal("NoopGeoIP.Lookup() result is nil")
	}
	if result.Province != "" || result.City != "" || result.ISP != "" {
		t.Errorf("NoopGeoIP.Lookup() = %+v, want empty", result)
	}
}

func TestParseIP2RegionLine(t *testing.T) {
	tests := []struct {
		line     string
		province string
		isp      string
	}{
		{"中国|华南|广东省|广州市|联通|联通", "广东省", "联通"},
		{"中国|华北|北京市|北京市|电信|电信", "北京市", "电信"},
		{"中国|华东|上海市|上海市|移动|移动", "上海市", "移动"},
		{"中国|0|0|0|0|0", "", ""},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := parseIP2RegionLine(tt.line)
			if result.Province != tt.province {
				t.Errorf("parseIP2RegionLine().Province = %q, want %q", result.Province, tt.province)
			}
			if result.ISP != tt.isp {
				t.Errorf("parseIP2RegionLine().ISP = %q, want %q", result.ISP, tt.isp)
			}
		})
	}
}
