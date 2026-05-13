package models

type RuleCluster struct {
	ID        uint `json:"id" gorm:"primarykey"`
	RuleID    uint `json:"rule_id" gorm:"index;not null;uniqueIndex:idx_rule_cluster"`
	ClusterID uint `json:"cluster_id" gorm:"index;not null;uniqueIndex:idx_rule_cluster"`
	Weight    int  `json:"weight" gorm:"default:1"`
	Priority  int  `json:"priority" gorm:"default:0"`
}
