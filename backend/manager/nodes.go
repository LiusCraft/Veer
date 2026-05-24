package manager

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var validStatuses = []string{"active", "inactive"}

func validateName(name string) (bool, string) {
	if name == "" {
		return false, "name 不能为空"
	}
	if len(name) < 1 || len(name) > 64 {
		return false, "name 必须为 1-64 个字符"
	}
	return true, ""
}

func validateURL(url string) (bool, string) {
	if url == "" {
		return false, "url 不能为空"
	}
	if len(url) > 512 {
		return false, "url 最大 512 个字符"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false, "url 必须以 http:// 或 https:// 开头"
	}
	return true, ""
}

func validateWeight(weight int) (bool, string) {
	if weight < 1 || weight > 100 {
		return false, "weight 必须为 1-100 之间的整数"
	}
	return true, ""
}

func validateNodeStatus(status string) (bool, string) {
	if status == "" {
		return true, ""
	}
	for _, valid := range validStatuses {
		if status == valid {
			return true, ""
		}
	}
	return false, "status 必须是 active 或 inactive"
}

func ListNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.CdnNode{})

		if clusterID := c.Query("cluster_id"); clusterID != "" {
			if cid, err := strconv.ParseUint(clusterID, 10, 64); err == nil {
				query = query.Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
					Where("node_clusters.cluster_id = ?", cid)
			}
		}
		if region := c.Query("region"); region != "" {
			query = query.Where("region = ?", region)
		}
		if isp := c.Query("isp"); isp != "" {
			query = query.Where("isp = ?", isp)
		}
		if provider := c.Query("provider"); provider != "" {
			query = query.Where("provider = ?", provider)
		}
		if nodeType := c.Query("node_type"); nodeType != "" {
			query = query.Where("node_type = ?", nodeType)
		}
		if status := c.Query("status"); status != "" {
			query = query.Where("status = ?", status)
		}

		var nodes []models.CdnNode
		if err := query.Order("created_at desc").Find(&nodes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		nodeIDs := make([]uint, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.ID
		}
		var nodeClusters []models.NodeCluster
		if len(nodeIDs) > 0 {
			db.Where("node_id IN ?", nodeIDs).Find(&nodeClusters)
		}
		clusterMap := make(map[uint][]uint)
		for _, nc := range nodeClusters {
			clusterMap[nc.NodeID] = append(clusterMap[nc.NodeID], nc.ClusterID)
		}

		type nodeWithClusters struct {
			models.CdnNode
			ClusterIDs []uint `json:"cluster_ids"`
		}
		result := make([]nodeWithClusters, len(nodes))
		for i, n := range nodes {
			ids := clusterMap[n.ID]
			if ids == nil {
				ids = []uint{}
			}
			result[i] = nodeWithClusters{CdnNode: n, ClusterIDs: ids}
		}

		c.JSON(http.StatusOK, gin.H{"data": result, "total": len(nodes)})
	}
}

func CreateNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name           string   `json:"name"`
			URL            string   `json:"url"`
			Weight         int      `json:"weight"`
			Region         string   `json:"region"`
			Status         string   `json:"status"`
			ClusterIDs     []uint   `json:"cluster_ids"`
			IP             string   `json:"ip"`
			ISP            string   `json:"isp"`
			Provider       string   `json:"provider"`
			NodeType       string   `json:"node_type"`
			BandwidthMbps  int      `json:"bandwidth_mbps"`
			MaxConnections int      `json:"max_connections"`
			Province       string   `json:"province"`
			City           string   `json:"city"`
			ISPList        []string `json:"isp_list"`
			CPUCores       int      `json:"cpu_cores"`
			MemoryMB       int64    `json:"memory_mb"`
			DiskSizeMB     int64    `json:"disk_size_mb"`
			UplinkMbps     int      `json:"uplink_mbps"`
			DownlinkMbps   int      `json:"downlink_mbps"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if ok, errMsg := validateName(body.Name); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if ok, errMsg := validateURL(body.URL); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		weight := body.Weight
		if weight == 0 {
			weight = 1
		}
		if ok, errMsg := validateWeight(weight); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		status := body.Status
		if status == "" {
			status = "active"
		}
		if ok, errMsg := validateNodeStatus(status); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		ispList := body.ISPList
		if len(ispList) == 0 && body.ISP != "" {
			ispList = []string{body.ISP}
		}

		node := models.CdnNode{
			Name:           body.Name,
			URL:            body.URL,
			Weight:         weight,
			Region:         body.Region,
			Status:         status,
			IP:             body.IP,
			ISP:            body.ISP,
			Provider:       body.Provider,
			NodeType:       body.NodeType,
			BandwidthMbps:  body.BandwidthMbps,
			MaxConnections: body.MaxConnections,
			Province:       body.Province,
			City:           body.City,
			ISPList:        ispList,
			CPUCores:       body.CPUCores,
			MemoryMB:       body.MemoryMB,
			DiskSizeMB:     body.DiskSizeMB,
			UplinkMbps:     body.UplinkMbps,
			DownlinkMbps:   body.DownlinkMbps,
		}

		if err := db.Create(&node).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for _, cid := range body.ClusterIDs {
			if cid > 0 {
				db.Create(&models.NodeCluster{NodeID: node.ID, ClusterID: cid})
			}
		}

		c.JSON(http.StatusCreated, gin.H{"data": node})
	}
}

func UpdateNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var node models.CdnNode
		if err := db.First(&node, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}

		var body struct {
			Name           string   `json:"name"`
			URL            string   `json:"url"`
			Weight         int      `json:"weight"`
			Region         *string  `json:"region"`
			Status         string   `json:"status"`
			ClusterIDs     []uint   `json:"cluster_ids"`
			IP             *string  `json:"ip"`
			ISP            *string  `json:"isp"`
			Provider       *string  `json:"provider"`
			NodeType       *string  `json:"node_type"`
			BandwidthMbps  *int     `json:"bandwidth_mbps"`
			MaxConnections *int     `json:"max_connections"`
			Province       *string  `json:"province"`
			City           *string  `json:"city"`
			ISPList        []string `json:"isp_list"`
			CPUCores       *int     `json:"cpu_cores"`
			MemoryMB       *int64   `json:"memory_mb"`
			DiskSizeMB     *int64   `json:"disk_size_mb"`
			UplinkMbps     *int     `json:"uplink_mbps"`
			DownlinkMbps   *int     `json:"downlink_mbps"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Name != "" {
			if ok, errMsg := validateName(body.Name); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Name = body.Name
		}

		if body.URL != "" {
			if ok, errMsg := validateURL(body.URL); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.URL = body.URL
		}

		if body.Weight != 0 {
			if ok, errMsg := validateWeight(body.Weight); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Weight = body.Weight
		}

		if body.Status != "" {
			if ok, errMsg := validateNodeStatus(body.Status); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Status = body.Status
		}

		if body.Region != nil {
			node.Region = *body.Region
		}
		if body.IP != nil {
			node.IP = *body.IP
		}
		if body.ISP != nil {
			node.ISP = *body.ISP
		}
		if body.Provider != nil {
			node.Provider = *body.Provider
		}
		if body.NodeType != nil {
			node.NodeType = *body.NodeType
		}
		if body.BandwidthMbps != nil {
			node.BandwidthMbps = *body.BandwidthMbps
		}
		if body.MaxConnections != nil {
			node.MaxConnections = *body.MaxConnections
		}
		if body.Province != nil {
			node.Province = *body.Province
		}
		if body.City != nil {
			node.City = *body.City
		}
		if body.ISPList != nil {
			node.ISPList = body.ISPList
		}
		if body.CPUCores != nil {
			node.CPUCores = *body.CPUCores
		}
		if body.MemoryMB != nil {
			node.MemoryMB = *body.MemoryMB
		}
		if body.DiskSizeMB != nil {
			node.DiskSizeMB = *body.DiskSizeMB
		}
		if body.UplinkMbps != nil {
			node.UplinkMbps = *body.UplinkMbps
		}
		if body.DownlinkMbps != nil {
			node.DownlinkMbps = *body.DownlinkMbps
		}

		if err := db.Save(&node).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if body.ClusterIDs != nil {
			db.Where("node_id = ?", id).Delete(&models.NodeCluster{})
			for _, cid := range body.ClusterIDs {
				if cid > 0 {
					db.Create(&models.NodeCluster{NodeID: node.ID, ClusterID: cid})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"data": node})
	}
}

func DeleteNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		db.Where("node_id = ?", id).Delete(&models.NodeCluster{})
		if err := db.Delete(&models.CdnNode{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
	}
}

func BatchDeleteNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IDs []uint `json:"ids" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if len(body.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
			return
		}

		db.Where("node_id IN ?", body.IDs).Delete(&models.NodeCluster{})
		if err := db.Delete(&models.CdnNode{}, body.IDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "batch delete successful", "deleted": len(body.IDs)})
	}
}

func BatchUpdateNodeStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IDs    []uint `json:"ids" binding:"required"`
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if len(body.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
			return
		}
		if ok, errMsg := validateNodeStatus(body.Status); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if err := db.Model(&models.CdnNode{}).Where("id IN ?", body.IDs).Update("status", body.Status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "batch status update successful", "updated": len(body.IDs)})
	}
}

func TestNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var node models.CdnNode
		if err := db.First(&node, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}

		latencyMs := measureLatency(node.URL)
		if node.InternalURL != "" {
			latencyMs = measureLatency(node.InternalURL)
		}

		db.Model(&node).UpdateColumn("latency", latencyMs)
		node.Latency = latencyMs

		c.JSON(http.StatusOK, gin.H{
			"data":    node,
			"latency": latencyMs,
			"message": "health check completed",
		})
	}
}

func measureLatency(url string) int {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return -1
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	latencyMs := int(time.Since(start).Milliseconds())
	if latencyMs < 1 {
		latencyMs = 1
	}
	return latencyMs
}

func GetNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var node models.CdnNode
		if err := db.First(&node, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": node})
	}
}
