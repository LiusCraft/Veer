package geoip

import "strings"

type LookupResult struct {
	Province string
	City     string
	ISP      string
	Region   string
}

type GeoIP interface {
	Lookup(ip string) (*LookupResult, error)
}

var ProvinceToRegion = map[string]string{
	"上海市": "华东", "江苏省": "华东", "浙江省": "华东",
	"安徽省": "华东", "福建省": "华东", "江西省": "华东", "山东省": "华东",
	"北京市": "华北", "天津市": "华北", "河北省": "华北",
	"山西省": "华北", "内蒙古自治区": "华北",
	"广东省": "华南", "广西壮族自治区": "华南", "海南省": "华南",
	"湖北省": "华中", "湖南省": "华中", "河南省": "华中",
	"重庆市": "西南", "四川省": "西南", "贵州省": "西南",
	"云南省": "西南", "西藏自治区": "西南",
	"陕西省": "西北", "甘肃省": "西北", "青海省": "西北",
	"宁夏回族自治区": "西北", "新疆维吾尔自治区": "西北",
	"辽宁省": "东北", "吉林省": "东北", "黑龙江省": "东北",
	"香港特别行政区": "华南", "澳门特别行政区": "华南", "台湾省": "华东",
}

func ProvinceToRegionName(province string) string {
	if region, ok := ProvinceToRegion[province]; ok {
		return region
	}
	if region, ok := ProvinceToRegion[strings.TrimRight(province, "省")+"省"]; ok {
		return region
	}
	return ""
}

func IsBGPNode(ispList []string) bool {
	return len(ispList) > 1
}

func MatchScore(clientProvince, clientRegion, clientISP string, nodeProvince, nodeRegion string, nodeISP string, nodeISPList []string) float64 {
	if nodeProvince == "" && nodeRegion == "" || nodeISP == "" {
		return 50
	}

	sameProvince := clientProvince != "" && clientProvince == nodeProvince
	sameRegion := false
	if clientRegion != "" && nodeRegion != "" {
		sameRegion = clientRegion == nodeRegion && !sameProvince
	}
	crossRegion := clientRegion != "" && nodeRegion != "" && !sameProvince && !sameRegion

	sameISP := clientISP != "" && nodeISP == clientISP
	crossISP := clientISP != "" && nodeISP != clientISP

	isBGP := IsBGPNode(nodeISPList)

	if isBGP {
		switch {
		case sameProvince:
			return 95
		case sameRegion:
			return 75
		default:
			return 55
		}
	}

	switch {
	case sameProvince && sameISP:
		return 100
	case sameProvince && crossISP:
		return 40
	case sameRegion && sameISP:
		return 80
	case sameRegion && crossISP:
		return 30
	case crossRegion && sameISP:
		return 60
	case crossRegion && crossISP:
		return 20
	default:
		return 50
	}
}
