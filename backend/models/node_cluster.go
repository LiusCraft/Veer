package models

type NodeCluster struct {
	ID        uint `json:"id" gorm:"primarykey"`
	NodeID    uint `json:"node_id" gorm:"index;not null;uniqueIndex:idx_node_cluster"`
	ClusterID uint `json:"cluster_id" gorm:"index;not null;uniqueIndex:idx_node_cluster"`
}
