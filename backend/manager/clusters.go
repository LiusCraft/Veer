package manager

import (
	"math"
	"net/http"
	"strconv"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListClusters(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.Cluster{})

		region := c.Query("region")
		if region != "" {
			query = query.Where("region LIKE ?", `%`+region+`%`)
		}
		isp := c.Query("isp")
		if isp != "" {
			query = query.Where("isp LIKE ?", `%`+isp+`%`)
		}
		status := c.Query("status")
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var clusters []models.Cluster
		if err := query.Order("created_at desc").Find(&clusters).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type NodeCount struct {
			ClusterID uint
			Count     int
			Active    int
		}
		var counts []NodeCount
		db.Table("node_clusters").
			Select("node_clusters.cluster_id, COUNT(*) as count, SUM(CASE WHEN cdn_nodes.status = 'active' THEN 1 ELSE 0 END) as active").
			Joins("LEFT JOIN cdn_nodes ON cdn_nodes.id = node_clusters.node_id").
			Group("node_clusters.cluster_id").Scan(&counts)
		countMap := make(map[uint]NodeCount)
		for _, c := range counts {
			countMap[c.ClusterID] = c
		}

		type ClusterRow struct {
			models.Cluster
			NodeCount  int     `json:"node_count"`
			HealthRate float64 `json:"health_rate"`
		}
		rows := make([]ClusterRow, len(clusters))
		for i, cl := range clusters {
			nc := countMap[cl.ID]
			rate := 0.0
			if nc.Count > 0 {
				rate = float64(nc.Active) / float64(nc.Count) * 100
			}
			rows[i] = ClusterRow{
				Cluster:    cl,
				NodeCount:  nc.Count,
				HealthRate: math.Round(rate*100) / 100,
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(clusters)})
	}
}

func CreateCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name           string   `json:"name"`
			Description    string   `json:"description"`
			Strategy       string   `json:"strategy"`
			Region         []string `json:"region"`
			ISP            []string `json:"isp"`
			Provider       string   `json:"provider"`
			Status         string   `json:"status"`
			BandwidthPrice float64  `json:"bandwidth_price"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Name == "" || len(body.Name) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name 必须为 1-64 个字符"})
			return
		}
		if len(body.Region) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "region 不能为空"})
			return
		}
		if len(body.ISP) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "isp 不能为空"})
			return
		}

		var count int64
		db.Model(&models.Cluster{}).Where("name = ?", body.Name).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "集群名称已存在"})
			return
		}

		strategy := body.Strategy
		if strategy == "" {
			strategy = "round-robin"
		}
		status := body.Status
		if status == "" {
			status = "active"
		}

		bwPrice := body.BandwidthPrice
		if bwPrice <= 0 {
			bwPrice = 1.0
		}

		cluster := models.Cluster{
			Name:           body.Name,
			Description:    body.Description,
			Strategy:       strategy,
			Region:         body.Region,
			ISP:            body.ISP,
			Provider:       body.Provider,
			Status:         status,
			BandwidthPrice: bwPrice,
		}

		if err := db.Create(&cluster).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": cluster})
	}
}

func GetCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var cluster models.Cluster
		if err := db.First(&cluster, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": cluster})
	}
}

func UpdateCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var cluster models.Cluster
		if err := db.First(&cluster, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}

		var body struct {
			Name           *string  `json:"name"`
			Description    *string  `json:"description"`
			Strategy       *string  `json:"strategy"`
			Region         []string `json:"region"`
			ISP            []string `json:"isp"`
			Provider       *string  `json:"provider"`
			Status         *string  `json:"status"`
			BandwidthPrice *float64 `json:"bandwidth_price"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Name != nil {
			if len(*body.Name) == 0 || len(*body.Name) > 64 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name 必须为 1-64 个字符"})
				return
			}
			var count int64
			db.Model(&models.Cluster{}).Where("name = ? AND id != ?", *body.Name, id).Count(&count)
			if count > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "集群名称已存在"})
				return
			}
			cluster.Name = *body.Name
		}
		if body.Description != nil {
			cluster.Description = *body.Description
		}
		if body.Strategy != nil {
			cluster.Strategy = *body.Strategy
		}
		if body.Region != nil {
			cluster.Region = body.Region
		}
		if body.ISP != nil {
			cluster.ISP = body.ISP
		}
		if body.Provider != nil {
			cluster.Provider = *body.Provider
		}
		if body.Status != nil {
			cluster.Status = *body.Status
		}
		if body.BandwidthPrice != nil {
			cluster.BandwidthPrice = *body.BandwidthPrice
		}

		if err := db.Save(&cluster).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": cluster})
	}
}

func DeleteCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var count int64
		db.Model(&models.RuleCluster{}).Where("cluster_id = ?", id).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "集群已被规则引用，请先解除关联"})
			return
		}

		db.Where("cluster_id = ?", id).Delete(&models.NodeCluster{})

		if err := db.Delete(&models.Cluster{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
	}
}

func SetClusterNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var body struct {
			NodeIDs []uint `json:"node_ids"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		cid := uint(id)
		db.Where("cluster_id = ?", cid).Delete(&models.NodeCluster{})
		for _, nid := range body.NodeIDs {
			if nid > 0 {
				db.Create(&models.NodeCluster{NodeID: nid, ClusterID: cid})
			}
		}

		var nodes []models.CdnNode
		db.Where("id IN ?", body.NodeIDs).Find(&nodes)
		c.JSON(http.StatusOK, gin.H{"data": nodes})
	}
}

func ListClusterNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		query := db.Model(&models.CdnNode{}).
			Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
			Where("node_clusters.cluster_id = ?", id)
		status := c.Query("status")
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var nodes []models.CdnNode
		if err := query.Find(&nodes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": nodes, "total": len(nodes)})
	}
}

func ListClusterRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var ruleClusters []models.RuleCluster
		db.Where("cluster_id = ?", id).Find(&ruleClusters)

		var ruleIDs []uint
		for _, rc := range ruleClusters {
			ruleIDs = append(ruleIDs, rc.RuleID)
		}

		var rules []models.RedirectRule
		if len(ruleIDs) > 0 {
			db.Where("id IN ?", ruleIDs).Find(&rules)
		}

		c.JSON(http.StatusOK, gin.H{"data": rules})
	}
}

func GetClusterStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var stats struct {
			Online         int     `json:"online"`
			Total          int     `json:"total"`
			AvgCPU         float64 `json:"avg_cpu"`
			AvgMem         float64 `json:"avg_mem"`
			AvgLatency     float64 `json:"avg_latency"`
			TotalBandwidth int     `json:"total_bandwidth"`
		}
		db.Table("cdn_nodes").
			Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
			Where("node_clusters.cluster_id = ?", id).
			Select("COUNT(*) as total").Scan(&stats)
		db.Table("cdn_nodes").
			Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
			Where("node_clusters.cluster_id = ? AND cdn_nodes.status = 'active'", id).
			Select("COUNT(*) as online, COALESCE(AVG(cdn_nodes.cpu_usage),0) as avg_cpu, COALESCE(AVG(cdn_nodes.mem_usage),0) as avg_mem, COALESCE(AVG(cdn_nodes.latency),0) as avg_latency, COALESCE(SUM(cdn_nodes.bandwidth_mbps),0) as total_bandwidth").
			Scan(&stats)

		c.JSON(http.StatusOK, gin.H{"data": stats})
	}
}
