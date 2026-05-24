package scheduler

import (
	"fmt"

	"veer/config"

	"github.com/spf13/viper"
)

func getSettlementCostFactor(clientISP, nodeISP string) float64 {
	if nodeISP == "" || clientISP == "" {
		return 1.0
	}

	v := viper.GetFloat64(fmt.Sprintf("scheduling.cost.settlement.%s_%s", clientISP, nodeISP))
	if v > 0 {
		return v
	}

	row, ok := config.DefaultSettlement[clientISP]
	if !ok {
		return 1.5
	}
	targetISP := nodeISP
	if _, exists := row[targetISP]; !exists {
		targetISP = "其他"
	}
	if val, exists := row[targetISP]; exists {
		return val
	}
	return 1.5
}

func getDistanceCost(sameProvince, sameRegion bool) float64 {
	if sameProvince {
		return viper.GetFloat64("scheduling.cost.distance.same_province")
	}
	if sameRegion {
		return viper.GetFloat64("scheduling.cost.distance.same_region")
	}
	return viper.GetFloat64("scheduling.cost.distance.cross_region")
}

func getSettlementTargetISP(nodeISP string, nodeISPList []string) string {
	if len(nodeISPList) > 1 {
		return "BGP"
	}
	return nodeISP
}
