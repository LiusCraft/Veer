package geoip

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type NoopGeoIP struct{}

func (n *NoopGeoIP) Lookup(ip string) (*LookupResult, error) {
	return &LookupResult{}, nil
}

var (
	globalSearcher GeoIP
	searcherOnce   sync.Once
)

func InitGlobalGeoIP(dbPath string) {
	searcherOnce.Do(func() {
		if dbPath == "" {
			globalSearcher = &NoopGeoIP{}
			return
		}
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Printf("[geoip] ip2region.xdb not found at %s, using noop fallback", dbPath)
			globalSearcher = &NoopGeoIP{}
			return
		}
		s, err := newIP2RegionSearcher(dbPath)
		if err != nil {
			log.Printf("[geoip] failed to init ip2region: %v, using noop fallback", err)
			globalSearcher = &NoopGeoIP{}
			return
		}
		globalSearcher = s
		log.Printf("[geoip] ip2region searcher initialized from %s", dbPath)
	})
}

func GlobalGeoIP() GeoIP {
	if globalSearcher == nil {
		return &NoopGeoIP{}
	}
	return globalSearcher
}

func LookupIP(addr string) (province, city string, err error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "", "", nil
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return "", "", nil
	}

	result, err := GlobalGeoIP().Lookup(host)
	if err != nil {
		return "", "", err
	}
	return result.Province, result.City, nil
}

func LookupClientInfo(addr string) (province, region, isp string) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "", "", ""
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return "", "", ""
	}

	result, err := GlobalGeoIP().Lookup(host)
	if err != nil || result == nil {
		return "", "", ""
	}

	region = ProvinceToRegionName(result.Province)
	return result.Province, region, result.ISP
}

func newIP2RegionSearcher(dbPath string) (*ip2RegionSearcher, error) {
	data, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, err
	}
	return &ip2RegionSearcher{data: data}, nil
}

type ip2RegionSearcher struct {
	data []byte
}

func (s *ip2RegionSearcher) Lookup(ipStr string) (*LookupResult, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return &LookupResult{}, nil
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return &LookupResult{}, nil
	}

	ipLong := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])

	region, err := s.binarySearch(ipLong)
	if err != nil {
		return &LookupResult{}, err
	}

	result := parseIP2RegionLine(region)
	return result, nil
}

func parseIP2RegionLine(line string) *LookupResult {
	parts := strings.Split(line, "|")
	result := &LookupResult{}
	if len(parts) > 1 {
		country := strings.TrimSpace(parts[0])
		region := strings.TrimSpace(parts[1])
		province := strings.TrimSpace(parts[2])
		city := strings.TrimSpace(parts[3])
		isp := strings.TrimSpace(parts[5])

		if province != "" && province != "0" {
			result.Province = province
		} else if region != "" && region != "0" && country == "中国" {
			result.Province = region
		}
		if city != "" && city != "0" {
			result.City = city
		}
		if isp != "" && isp != "0" {
			result.ISP = normalizeISP(isp)
		}
	}
	return result
}

func normalizeISP(isp string) string {
	isp = strings.ToLower(isp)
	switch {
	case strings.Contains(isp, "电信"):
		return "电信"
	case strings.Contains(isp, "联通"), strings.Contains(isp, "网通"):
		return "联通"
	case strings.Contains(isp, "移动"):
		return "移动"
	default:
		return "其他"
	}
}

func (s *ip2RegionSearcher) binarySearch(ip uint32) (string, error) {
	data := s.data
	headerLen := 256 * 4 * 4
	if len(data) < headerLen {
		return "", nil
	}

	var startIP, endIP, startPtr uint32

	firstIdxPtr := bytesToUint32(data[0:4])
	lastIdxPtr := bytesToUint32(data[4:8])
	totalBlocks := (lastIdxPtr-firstIdxPtr)/12 + 1

	low := uint32(0)
	high := totalBlocks - 1

	for low <= high {
		mid := (low + high) / 2
		ptr := firstIdxPtr + mid*12

		startIP = bytesToUint32(data[ptr : ptr+4])
		endIP = bytesToUint32(data[ptr+4 : ptr+8])

		if ip >= startIP && ip <= endIP {
			startPtr = bytesToUint32(data[ptr+8 : ptr+12])
			break
		} else if ip < startIP {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	if startPtr == 0 {
		return "", nil
	}

	regionLen := int(data[startPtr])
	regionBytes := data[startPtr+1 : startPtr+1+uint32(regionLen)]
	return string(regionBytes), nil
}

func bytesToUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
