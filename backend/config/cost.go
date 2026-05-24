package config

var DefaultSettlement = map[string]map[string]float64{
	"电信": {"电信": 1.0, "联通": 2.5, "移动": 1.8, "BGP": 1.0, "其他": 1.5},
	"联通": {"电信": 2.0, "联通": 1.0, "移动": 1.5, "BGP": 1.0, "其他": 1.3},
	"移动": {"电信": 1.5, "联通": 1.3, "移动": 1.0, "BGP": 1.0, "其他": 1.2},
	"其他": {"电信": 1.5, "联通": 1.5, "移动": 1.2, "BGP": 1.0, "其他": 1.0},
}

type DistanceCostConfig struct {
	SameProvince float64 `mapstructure:"same_province"`
	SameRegion   float64 `mapstructure:"same_region"`
	CrossRegion  float64 `mapstructure:"cross_region"`
}
