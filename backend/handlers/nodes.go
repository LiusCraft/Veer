// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// validStatuses 有效的节点状态列表
var validStatuses = []string{"active", "inactive"}

// validateName 验证节点名称
//
// 参数:
//   - name: 待验证的名称
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateName(name string) (bool, string) {
	if name == "" {
		return false, "name 不能为空"
	}
	if len(name) < 1 || len(name) > 64 {
		return false, "name 必须为 1-64 个字符"
	}
	return true, ""
}

// validateURL 验证节点 URL 格式
//
// 参数:
//   - url: 待验证的 URL
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
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

// validateWeight 验证节点权重
//
// 参数:
//   - weight: 待验证的权重值
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateWeight(weight int) (bool, string) {
	if weight < 1 || weight > 100 {
		return false, "weight 必须为 1-100 之间的整数"
	}
	return true, ""
}

// validateNodeStatus 验证节点状态
//
// 参数:
//   - status: 待验证的状态值
//
// 返回:
//   - bool: 是否有效
//   - string: 错误信息（如果无效）
func validateNodeStatus(status string) (bool, string) {
	if status == "" {
		return true, "" // 允许为空，使用默认值
	}
	for _, valid := range validStatuses {
		if status == valid {
			return true, ""
		}
	}
	return false, "status 必须是 active 或 inactive"
}

// ListNodes 处理 GET /api/nodes — 列出所有 CDN 节点
func ListNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodes []models.CdnNode
		if err := db.Order("created_at desc").Find(&nodes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": nodes, "total": len(nodes)})
	}
}

// CreateNode 处理 POST /api/nodes — 创建新的 CDN 节点
func CreateNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name   string `json:"name"`
			URL    string `json:"url"`
			Weight int    `json:"weight"`
			Region string `json:"region"`
			Status string `json:"status"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 手动校验 name
		if ok, errMsg := validateName(body.Name); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 手动校验 url
		if ok, errMsg := validateURL(body.URL); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 手动校验 weight
		weight := body.Weight
		if weight == 0 {
			weight = 1 // 默认权重为 1
		}
		if ok, errMsg := validateWeight(weight); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		// 校验 status
		status := body.Status
		if status == "" {
			status = "active" // 默认状态为 active
		}
		if ok, errMsg := validateNodeStatus(status); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		node := models.CdnNode{
			Name:   body.Name,
			URL:    body.URL,
			Weight: weight,
			Region: body.Region,
			Status: status,
		}

		if err := db.Create(&node).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": node})
	}
}

// UpdateNode 处理 PUT /api/nodes/:id — 更新 CDN 节点
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
			Name   string `json:"name"`
			URL    string `json:"url"`
			Weight int    `json:"weight"`
			Region string `json:"region"`
			Status string `json:"status"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 校验 name（如果提供了）
		if body.Name != "" {
			if ok, errMsg := validateName(body.Name); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Name = body.Name
		}

		// 校验 url（如果提供了）
		if body.URL != "" {
			if ok, errMsg := validateURL(body.URL); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.URL = body.URL
		}

		// 校验 weight（如果提供了）
		if body.Weight != 0 {
			if ok, errMsg := validateWeight(body.Weight); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Weight = body.Weight
		}

		// 校验 status（如果提供了）
		if body.Status != "" {
			if ok, errMsg := validateNodeStatus(body.Status); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}
			node.Status = body.Status
		}

		node.Region = body.Region

		if err := db.Save(&node).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": node})
	}
}

// DeleteNode 处理 DELETE /api/nodes/:id — 删除 CDN 节点
func DeleteNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		if err := db.Delete(&models.CdnNode{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
	}
}

// BatchDeleteNodes 处理 DELETE /api/nodes/batch — 批量删除 CDN 节点
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

		if err := db.Delete(&models.CdnNode{}, body.IDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "batch delete successful", "deleted": len(body.IDs)})
	}
}

// BatchUpdateNodeStatus 处理 PUT /api/nodes/batch/status — 批量更新节点状态
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

// TestNode 处理 POST /api/nodes/:id/test — 对节点执行 HTTP 健康检查
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

		// 使用 httptrace 测量 HTTP 延迟
		latencyMs := measureLatency(node.URL)

		// 更新数据库中的延迟值
		db.Model(&node).UpdateColumn("latency", latencyMs)
		node.Latency = latencyMs

		c.JSON(http.StatusOK, gin.H{
			"data":    node,
			"latency": latencyMs,
			"message": "health check completed",
		})
	}
}

// measureLatency 对指定 URL 执行 HTTP GET 并返回延迟（毫秒）
func measureLatency(url string) int {
	var start time.Time
	var latency time.Duration

	req, err := newRequestWithTrace(url, &start, &latency)
	if err != nil {
		return -1
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	if latency == 0 {
		latency = time.Since(start)
	}

	return int(latency.Milliseconds())
}

// newRequestWithTrace 创建带有连接时间追踪的 HTTP 请求
func newRequestWithTrace(url string, start *time.Time, latency *time.Duration) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			if !start.IsZero() {
				*latency = time.Since(*start)
			}
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	return req, nil
}
