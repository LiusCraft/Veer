# 302CDN 资源管理系统 v3 架构设计

**版本**: v3.0
**日期**: 2026-05-14
**状态**: 待评审

---

## 1. 概述

### 1.1 目标

解决 v2 系统中资源管理的三个核心缺陷：

1. **节点是孤岛**：规则直接引用节点 ID（JSON 字符串反模式），增删节点必须改规则，运维风险高
2. **缺乏集群抽象**：没有"节点组"概念，无法按区域/运营商/业务维度批量管理节点
3. **运维观测空白**：集群健康状态、节点实时负载、流量分布无可视化，故障发现靠人工登录服务器

### 1.2 范围

本文档覆盖三个功能模块：

| 模块 | 类型 | 说明 |
|------|------|------|
| 集群管理 (Cluster) | 新增 | 节点分组抽象，作为调度的基本单元 |
| 节点管理 (Node) | 增强 | 增加集群归属、运维字段、Agent 上报数据 |
| 调度视图 (Topology) | 新增 | 只读拓扑面板：集群健康矩阵 + 节点热力图 + 流量分布 |

### 1.3 非目标

- 不修改调度器 302 重定向核心逻辑
- 不修改边缘节点缓存代理核心逻辑
- 不引入消息队列或外部存储（仍使用 SQLite）
- 不做多租户隔离（留给 v4）

---

## 2. 数据模型

### 2.1 新增：Cluster（集群）

集群是调度的基本单元。一条规则关联一个或多个集群（主备/流量拆分），一个集群包含多个节点。

```go
type Cluster struct {
    ID          uint      `json:"id" gorm:"primarykey"`
    Name        string    `json:"name" gorm:"size:64;not null;uniqueIndex"`      // 集群名称，如"华北-电信-A"
    Description string    `json:"description" gorm:"size:256"`                   // 备注
    Strategy    string    `json:"strategy" gorm:"size:16;default:'round-robin'"` // 调度策略
    Region      string    `json:"region" gorm:"size:32;index"`                   // 区域：华东/华北/华南/海外
    ISP         string    `json:"isp" gorm:"size:32;index"`                      // 运营商：电信/联通/移动/aws/azure
    Provider    string    `json:"provider" gorm:"size:32"`                       // 云厂商：aliyun/aws/azure/self
    Status      string    `json:"status" gorm:"size:16;default:'active'"`        // active / degraded / inactive
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**设计决策**：
- Region + ISP 作为集群的"路由标签"，将来调度器可按地域/运营商做 DNS 级别分流
- Provider 独立于 ISP：一个 AWS 集群可能跨多 ISP，但都归 aws 管理
- Status 新增 `degraded` 状态：集群内部分节点故障但未完全宕机

### 2.2 变更：CdnNode（节点）

```go
type CdnNode struct {
    ID               uint      `json:"id" gorm:"primarykey"`
    ClusterID        uint      `json:"cluster_id" gorm:"index;not null;default:0"`  // 所属集群

    Name             string    `json:"name" gorm:"size:64;not null"`
    URL              string    `json:"url" gorm:"size:512;not null"`                // 公网 endpoint

    // 网络信息
    IP               string    `json:"ip" gorm:"size:45"`                           // 管理 IP
    Region           string    `json:"region" gorm:"size:32"`                       // 物理区域
    ISP              string    `json:"isp" gorm:"size:32"`                          // 物理运营商
    Provider         string    `json:"provider" gorm:"size:32"`                     // 云厂商
    NodeType         string    `json:"node_type" gorm:"size:16;default:'edge'"`     // edge / scheduler

    // 调度权重 & 容量
    Weight           int       `json:"weight" gorm:"default:1"`
    BandwidthMbps    int       `json:"bandwidth_mbps" gorm:"default:1000"`          // 总带宽上限
    MaxConnections   int       `json:"max_connections" gorm:"default:10000"`        // 最大并发连接数

    // Agent 上报的运行时指标（由边缘节点 Agent 定时上报）
    CPUUsage         float64   `json:"cpu_usage" gorm:"default:0"`                  // CPU 使用率 0-100
    MemUsage         float64   `json:"mem_usage" gorm:"default:0"`                  // 内存使用率 0-100
    DiskUsage        float64   `json:"disk_usage" gorm:"default:0"`                 // 磁盘使用率 0-100
    LoadAvg          float64   `json:"load_avg" gorm:"default:0"`                   // 系统负载
    AgentVersion     string    `json:"agent_version" gorm:"size:32"`                // Agent 版本号
    LastHeartbeat    time.Time `json:"last_heartbeat"`                              // 最后一次心跳时间

    // 健康检测
    Status           string    `json:"status" gorm:"size:16;default:'active'"`      // active / inactive
    Latency          int       `json:"latency"`                                     // ms
    ConsecutiveFails int       `json:"consecutive_fails" gorm:"default:0"`

    // 边缘节点配置（下发到边缘）
    OriginBaseURL    string    `json:"origin_base_url" gorm:"size:512;default:''"`
    CacheTTL         int       `json:"cache_ttl" gorm:"default:300"`

    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}
```

**新增字段说明**：

| 字段 | 来源 | 用途 |
|------|------|------|
| `ClusterID` | 管理后台配置 | 节点归属集群 |
| `IP` | 边缘注册上报 | 运维管理：SSH/Ping 地址 |
| `ISP` | 管理后台配置 | 运营商感知调度（将来）|
| `Provider` | 管理后台配置 | 云厂商统计、成本分析 |
| `NodeType` | 管理后台配置 | 区分调度节点和边缘缓存节点 |
| `BandwidthMbps` | 管理后台配置 | 容量管理：过载告警阈值 |
| `MaxConnections` | 管理后台配置 | 容量管理：过载告警阈值 |
| `CPUUsage/MemUsage/DiskUsage/LoadAvg` | Agent 定时上报 | 实时负载展示、过载调度 |
| `LastHeartbeat` | Agent 定时上报 | 判断节点 Agent 是否存活 |
| `UpdatedAt` | 自动更新 | 追踪节点配置变更时间 |

### 2.3 新增：RuleCluster（规则-集群关联）

替代 `RedirectRule.NodeIDs`，建立规则与集群的多对多关系。

```go
type RuleCluster struct {
    ID        uint `json:"id" gorm:"primarykey"`
    RuleID    uint `json:"rule_id" gorm:"index;not null;uniqueIndex:idx_rule_cluster"`
    ClusterID uint `json:"cluster_id" gorm:"index;not null;uniqueIndex:idx_rule_cluster"`
    Weight    int  `json:"weight" gorm:"default:1"`   // 集群间流量权重
    Priority  int  `json:"priority" gorm:"default:0"` // 主备优先级（0=主, 1=备, 2=备...）
}
```

一条规则可以关联多个集群，用于：
- **流量拆分**：80% 流量到华东电信，20% 到华北联通（Weight 控制）
- **主备容灾**：主集群故障时自动切换到备集群（Priority 控制）
- **灰度发布**：先切 5% 到新集群验证，逐步放量

### 2.4 变更：RedirectRule（规则）

```go
type RedirectRule struct {
    ID          uint      `json:"id" gorm:"primarykey"`
    Name        string    `json:"name" gorm:"size:128"`
    Description string    `json:"description"`
    Enabled     bool      `json:"enabled" gorm:"default:true"`
    Priority    int       `json:"priority" gorm:"default:0"`
    RuleType    string    `json:"rule_type" gorm:"size:32;default:'domain_routing'"`
    Domain      string    `json:"domain" gorm:"size:253"`
    Strategy    string    `json:"strategy" gorm:"size:16;default:'round-robin'"`

    // NodeIDs 已废弃，改为通过 RuleCluster 关联集群
    // NodeIDs string `json:"node_ids"`

    OriginBaseURL string `json:"origin_base_url" gorm:"size:512;default:''"`
    HitCount      int64  `json:"hit_count"`

    // url_redirect 类型字段（不变）
    MatchType    string `json:"match_type" gorm:"size:16;default:'prefix'"`
    SourcePath   string `json:"source_path" gorm:"size:512;default:'/'"`
    TargetHost   string `json:"target_host" gorm:"size:253"`
    TargetPath   string `json:"target_path" gorm:"size:512"`
    RedirectCode int    `json:"redirect_code" gorm:"default:302"`

    CreatedAt    time.Time `json:"created_at"`
}
```

**变更点**：
- 移除 `NodeIDs` 字段（保留列但不再使用，迁移完成后删除）
- `Strategy` 字段保留，但实际调度策略优先使用集群自身的 `Cluster.Strategy`
- 规则级别的 Strategy 仅作为"推荐的集群间调度策略"

### 2.5 新增：ClusterMetric（集群指标快照，可选）

用于调度视图的流量分布数据。

```go
type ClusterMetric struct {
    ID              uint      `json:"id" gorm:"primarykey"`
    ClusterID       uint      `json:"cluster_id" gorm:"index"`
    RequestCount    int64     `json:"request_count"`     // 统计周期内请求数
    BandwidthBytes  int64     `json:"bandwidth_bytes"`   // 统计周期内流量
    AvgLatencyMs    float64   `json:"avg_latency_ms"`
    PeriodMinutes   int       `json:"period_minutes"`    // 统计周期
    RecordedAt      time.Time `json:"recorded_at"`
}
```

### 2.6 ER 关系

```
Cluster (1) ──── (N) CdnNode
Cluster (N) ──── (N) RedirectRule   (via RuleCluster)
RedirectRule (1) ──── (N) RuleCluster ──── (1) Cluster
```

### 2.7 数据迁移策略

| 步骤 | 操作 | 可回滚 |
|------|------|--------|
| 1 | 新建 `clusters` 表、`rule_clusters` 表、`cluster_metrics` 表 | 是 |
| 2 | CdnNode 表新增字段（ClusterID + 运维字段），均为 nullable | 是 |
| 3 | 创建默认集群 `default`，将现有节点全部归入 | 是 |
| 4 | 将现有规则的 `NodeIDs` 迁移到 `rule_clusters` | 是 |
| 5 | 代码中逐模块切换为使用新关联 | 逐步 |
| 6 | 确认稳定后，废弃 `NodeIDs` 列 | 否（可保留） |

---

## 3. API 设计

### 3.1 集群管理 API

| 方法 | 路径 | 说明 | 请求体/参数 | 响应 |
|------|------|------|-------------|------|
| GET | `/api/clusters` | 集群列表 | `?region=&isp=&status=` | `{data: [Cluster], total: N}` |
| POST | `/api/clusters` | 创建集群 | Cluster body | `{data: Cluster}` |
| GET | `/api/clusters/:id` | 集群详情 | - | `{data: Cluster}` |
| PUT | `/api/clusters/:id` | 更新集群 | Cluster body 部分字段 | `{data: Cluster}` |
| DELETE | `/api/clusters/:id` | 删除集群 | - | `{message: ...}` |

> **删除行为**：删除集群时，该集群下的所有节点的 `cluster_id` 被置为 0（即成为"未归属"节点），不级联删除节点。如果有关联规则引用了该集群，则请求被拒绝（400），要求先解除规则与集群的关联后再删除。

| GET | `/api/clusters/:id/nodes` | 集群内节点列表 | `?status=` | `{data: [CdnNode], total: N}` |
| PUT | `/api/clusters/:id/nodes` | 批量设置集群成员 | `{node_ids: [1,2,3]}` | `{data: [CdnNode]}` |
| GET | `/api/clusters/:id/rules` | 引用此集群的规则列表 | - | `{data: [RedirectRule]}` |
| GET | `/api/clusters/:id/stats` | 集群聚合指标 | - | `{online, total, avg_cpu, avg_mem, avg_latency, total_bandwidth}` |

### 3.2 节点管理 API（增强）

| 方法 | 路径 | 变更 |
|------|------|------|
| GET | `/api/nodes` | 新增筛选 `?cluster_id=&region=&isp=&provider=&node_type=&status=`；新增响应字段 |
| POST | `/api/nodes` | 新增请求字段 `cluster_id`, `ip`, `isp`, `provider`, `node_type`, `bandwidth_mbps`, `max_connections` |
| PUT | `/api/nodes/:id` | 同上 |
| GET | `/api/nodes/:id` | **新增**：单个节点详情 |
| POST | `/api/nodes/:id/heartbeat` | **新增**：Agent 上报心跳和负载数据 |

#### Agent 心跳端点

```http
POST /api/nodes/:id/heartbeat
X-Edge-Secret: <manager_edge_secret>

{
  "cpu_usage": 45.2,
  "mem_usage": 62.1,
  "disk_usage": 33.5,
  "load_avg": 2.5,
  "request_count_1m": 1200,
  "bandwidth_bytes_1m": 524288000
}
```

**认证说明**：心跳端点使用 `X-Edge-Secret` 请求头进行认证（与已有的边缘注册 `POST /api/edge/register`、规则同步 `GET /api/edge/rules` 使用相同的共享密钥模式）。密钥对比 `config.Edge.Manager.Secret`（默认 `veer-edge-secret`，通过配置文件 `edge.manager.secret` 或环境变量 `CDNC_EDGE_MANAGER_SECRET` 配置）。

此端点**不经过** JWT 中间件，在路由注册时跳过 `/api` 组的 JWT 认证，单独绑定到 `/api/nodes/:id/heartbeat`。

### 3.3 规则管理 API（变更）

| 方法 | 路径 | 变更 |
|------|------|------|
| POST | `/api/rules` | 移除 `node_ids`，新增 `clusters: [{cluster_id, weight, priority}]` |
| PUT | `/api/rules/:id` | 同上 |
| GET | `/api/rules` | 响应中规则附带关联的集群信息 |
| GET | `/api/rules/:id` | **新增**：单条规则详情，含完整集群拓扑 |

### 3.4 调度视图 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/views/topology` | 完整调度拓扑：集群 → 节点树 |
| GET | `/api/views/health-matrix` | 集群健康矩阵（在线率/平均负载/延迟/带宽）|
| GET | `/api/views/traffic-distribution` | 流量分布数据 |

#### GET /api/views/topology 响应

```json
{
  "data": {
    "clusters": [
      {
        "id": 1,
        "name": "华北-电信-A",
        "region": "华北",
        "isp": "电信",
        "strategy": "round-robin",
        "status": "active",
        "stats": {
          "online": 5,
          "total": 5,
          "avg_cpu": 32.5,
          "avg_mem": 45.2,
          "avg_latency": 12,
          "total_bandwidth_mbps": 5000
        },
        "nodes": [
          {
            "id": 1,
            "name": "hb-telecom-01",
            "ip": "10.0.1.1",
            "status": "active",
            "latency": 8,
            "cpu_usage": 28.3,
            "mem_usage": 42.1,
            "last_heartbeat": "2026-05-14T10:00:00Z"
          }
        ]
      }
    ]
  }
}
```

#### GET /api/views/health-matrix 响应

```json
{
  "data": [
    {
      "cluster_id": 1,
      "cluster_name": "华北-电信-A",
      "region": "华北",
      "isp": "电信",
      "online_nodes": 5,
      "total_nodes": 5,
      "online_rate": 1.0,
      "avg_cpu": 32.5,
      "avg_mem": 45.2,
      "avg_disk": 60.1,
      "avg_latency": 12,
      "total_bandwidth_mbps": 5000,
      "health_status": "healthy"        // healthy / overload / degraded / down
    }
  ]
}
```

#### GET /api/views/traffic-distribution 响应

```json
{
  "data": {
    "period_minutes": 5,
    "clusters": [
      { "cluster_id": 1, "cluster_name": "华北-电信-A", "requests": 150000, "bandwidth_gbps": 5.0 },
      { "cluster_id": 2, "cluster_name": "华东-电信-A", "requests": 120000, "bandwidth_gbps": 4.2 },
      { "cluster_id": 3, "cluster_name": "华北-联通-A", "requests": 60000,  "bandwidth_gbps": 2.0 }
    ],
    "total_requests": 330000,
    "total_bandwidth_gbps": 11.2
  }
}
```

---

## 4. 前端设计

### 4.1 导航结构

```
概览仪表盘      ← 不变
调度视图        ← 新增，顶层菜单
资源管理        ← 原"资源管理"，收起展开
  ├── 集群管理  ← 原"业务组"重命名并激活
  └── 节点管理  ← 已有页面增强
域名管理        ← 不变
访问日志        ← 不变
```

### 4.2 调度视图页面 `/views`

#### 布局

```
┌──────────────────────────────────────────────────────────┐
│  调度视图                              [自动刷新 30s] 🔄  │
├──────────────────────────────────────────────────────────┤
│  筛选: 区域[▼] 运营商[▼] 状态[▼]                         │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─── 集群健康矩阵 ─────────────────────────────────┐   │
│  │  集群        在/总  CPU  延迟   带宽     状态     │   │
│  │  华北-电信-A  5/5   32%  12ms  5.0Gbps  ● 健康   │   │
│  │  华北-联通-A  2/3   78%  45ms  2.0Gbps  ● 过载   │   │
│  │  华东-电信-A  8/8   28%   8ms  8.0Gbps  ● 健康   │   │
│  │  华东-移动-A  1/4   92%  120ms 1.0Gbps  ● 故障   │   │
│  └────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─── 节点热力图 ─────────────────────────────────┐     │
│  │                                                  │   │
│  │  华北-电信-A  ■ ■ ■ ■ ■   5/5 全绿             │   │
│  │  华北-联通-A  ■ ■ ■       3节点, 1红1黄1绿     │   │
│  │  华东-移动-A  ■ ■ ■ ■     4节点, 3灰1红        │   │
│  │                                                  │   │
│  │  图例: ● 健康 ● 高负载 ● 故障 ● 离线            │   │
│  └────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─── 流量分布 ───────────────────────────────────┐     │
│  │  华北-电信-A  ██████████████████████  5.0Gbps  │   │
│  │  华东-电信-A  ████████████████████   4.2Gbps   │   │
│  │  华北-联通-A  ██████████           2.0Gbps     │   │
│  │  华东-移动-A  ████                 1.0Gbps     │   │
│  └────────────────────────────────────────────────┘   │
│                                                          │
├──────────────────────────────────────────────────────────┤
│  最后更新: 2026-05-14T10:00:00Z                          │
└──────────────────────────────────────────────────────────┘
```

#### 交互

| 操作 | 行为 |
|------|------|
| 点击集群行 | 跳转集群管理页对应集群详情 |
| 点击节点圆点 | 跳转节点管理页对应节点 |
| 筛选 | 按区域/运营商/状态过滤集群 |
| 自动刷新 | 默认 30s 拉取一次，可手动关闭 |
| 手动刷新 | 右上角刷新按钮立即拉取 |

### 4.3 集群管理页面 `/clusters`

**列表视图**：

```
┌──────────────────────────────────────────────────────────┐
│  集群管理                             [+ 新建集群] [导出] │
├──────────────────────────────────────────────────────────┤
│  搜索框 [                                                   ]
├──────────────────────────────────────────────────────────┤
│  ☐  集群名称     │ 区域/运营商 │ 节点 │ 健康率 │ 策略 │ 状态 │
│  ☐  华北-电信-A  │ 华北/电信   │  5   │ 100%  │ rr  │ ●   │
│  ☐  华北-联通-A  │ 华北/联通   │  3   │  67%  │ w   │ ●   │
│  ☐  华东-移动-A  │ 华东/移动   │  4   │  25%  │ rr  │ ●   │
├──────────────────────────────────────────────────────────┤
│  已选 N 个  [批量删除]  [批量启用]  [批量停用]            │
└──────────────────────────────────────────────────────────┘
```

**详情抽屉**（点击集群行展开）：

```
┌─── 华北-电信-A ──────────────────────────────────────┐
│  基础信息: 区域=华北  ISP=电信  策略=round-robin      │
│  状态: ● active  创建于 2026-01-15                   │
│                                                      │
│  ┌── 成员节点 (5) ─────────────────────────────┐     │
│  │ 节点名         IP           状态   延迟  CPU │     │
│  │ hb-tele-01  10.0.1.1    ● active  8ms  28% │     │
│  │ hb-tele-02  10.0.1.2    ● active 12ms  35% │     │
│  │ hb-tele-03  10.0.1.3    ● active 10ms  30% │     │
│  │ hb-tele-04  10.0.1.4   ○ inactive  -    -  │     │
│  │ hb-tele-05  10.0.1.5    ● active  9ms  32% │     │
│  │                                [编辑成员]    │     │
│  └────────────────────────────────────────────┘     │
│                                                      │
│  ┌── 关联规则 (3) ─────────────────────────────┐     │
│  │ 规则名     域名            策略    命中数    │     │
│  │ static   cdn.veer.local  weighted  1.2M     │     │
│  │ video    video.veer.local weighted  890K    │     │
│  │ image    img.veer.local   rr        456K    │     │
│  └────────────────────────────────────────────┘     │
│                                                      │
│  ┌── 集群指标 ─────────────────────────────────┐     │
│  │  在线率 100%  │ 平均CPU 31% │ 平均延迟 10ms │     │
│  │  总带宽 5Gbps │ 总连接 4.5K                  │     │
│  └────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────┘
```

**新建/编辑集群表单**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 名称 | text | 是 | 唯一，如"华北-电信-A" |
| 区域 | select | 是 | 华东/华北/华南/海外/其他 |
| 运营商 | select | 是 | 电信/联通/移动/aws/azure/其他 |
| 云厂商 | select | 否 | aliyun/aws/azure/self/其他 |
| 调度策略 | select | 否 | round-robin / weighted / random，默认 round-robin |
| 备注 | text | 否 | 256 字以内 |

### 4.4 节点管理页面 `/nodes`（增强）

在现有表格基础上：

**新增列**：
- 所属集群（Chip，可点击跳转）
- IP 地址
- 云厂商
- CPU 使用率（紧凑进度条）
- 最后心跳（相对时间）
- 节点类型（edge/scheduler）

**新增筛选器**：
- 集群下拉筛选
- 云厂商筛选
- 节点类型筛选

**新增详情抽屉**：点击节点行展开，展示实时指标趋势图（预留图表接口）。

**新增/编辑对话框**增加字段：
- 所属集群（下拉选择）
- IP 地址
- 运营商（自动从集群继承，可覆盖）
- 云厂商
- 节点类型
- 带宽上限 (Mbps)
- 最大连接数

---

## 5. 后端变更清单

### 5.1 新增文件

| 文件 | 说明 |
|------|------|
| `backend/models/cluster.go` | Cluster 模型 |
| `backend/models/rule_cluster.go` | RuleCluster 关联模型 |
| `backend/models/cluster_metric.go` | 集群指标快照模型（可选） |
| `backend/handlers/clusters.go` | 集群 CRUD + 成员管理 + 统计 |
| `backend/handlers/views.go` | 调度视图三个面板 API |
| `backend/handlers/node_heartbeat.go` | Agent 心跳上报端点 |

### 5.2 修改文件

| 文件 | 变更 |
|------|------|
| `backend/models/cdn_node.go` | 新增 ClusterID + 运维字段 |
| `backend/models/redirect_rule.go` | 标记 NodeIDs 废弃（注释） |
| `backend/handlers/nodes.go` | 新增字段处理、详情端点、新增筛选参数 |
| `backend/handlers/rules.go` | 新增 clusters 参数、不再使用 node_ids |
| `backend/handlers/redirect.go` | RuleCache 改为通过 RuleCluster 加载集群→节点 |
| `backend/router/manager.go` | 注册新路由 |
| `backend/config/config.go` | SeedData 适配新模型 |
| `backend/services/health_check.go` | 健康检测结果写入节点新增字段 |
| `frontend/src/pages/Nodes.jsx` | 新增列、筛选器、详情抽屉 |
| `frontend/src/pages/Rules.jsx` | 规则编辑改为选集群 |
| `frontend/src/components/Layout.jsx` | 导航结构调整 |
| `frontend/src/App.jsx` | 新增路由 `/views`、`/clusters` |

### 5.3 不修改的文件

| 文件 | 原因 |
|------|------|
| `backend/edge/server.go` | 边缘代理逻辑不涉及资源模型 |
| `backend/edge/client.go` | 注册/同步逻辑不变；新增字段作为补充 |
| `backend/middleware/*` | 中间件不涉及业务模型 |
| `backend/cmd/*` | 二进制入口不涉及资源模型 |

---

## 6. 调度器适配

### 6.1 当前调度器加载规则的方式

```
refresh():
  1. SELECT * FROM redirect_rules WHERE enabled = true
  2. 解析 rule.NodeIDs JSON → []uint
  3. SELECT * FROM cdn_nodes WHERE id IN (?) AND status = 'active'
  4. 按 strategy 选择节点
```

### 6.2 改为集群模式后的调度逻辑

```
refresh():
  1. SELECT * FROM redirect_rules WHERE enabled = true
  2. SELECT * FROM rule_clusters WHERE rule_id IN (rule_ids)
  3. 加载集群及其活跃节点
     clusters := db.Preload("Nodes", "status = ?", "active").Where("id IN ?", clusterIDs).Find()
  4. 按主备/权重选择集群 → 按集群策略选择节点
```

**实现说明**：第 3 步使用 GORM 的 `Preload` 而非原生 JOIN 查询。`Preload` 会执行两条 SQL（先查集群、再查节点），自动将节点按 `ClusterID` 归组到对应集群的 `Nodes` 字段中，避免手动处理 JOIN 返回的扁平行。如果后续需优化查询性能，可改为单条 JOIN + 手动归组。

### 6.3 关键修改点

| 组件 | 修改 |
|------|------|
| `handlers/redirect.go RuleCache` | refresh() 改为查 rule_clusters 关联表 |
| `handlers/redirect.go selectNode()` | 增加层级：先选集群再选节点 |
| `handlers/redirect.go` | 集群间支持 weighted 和 priority 两种选择模式 |

---

## 7. 调度视图的三个数据面板

### 7.1 集群健康矩阵

**数据来源**：

| 指标 | 来源 | 计算方式 |
|------|------|----------|
| 在线节点数 | CdnNode 表 | `COUNT(*) WHERE cluster_id=? AND status='active'` |
| 总节点数 | CdnNode 表 | `COUNT(*) WHERE cluster_id=?` |
| 平均 CPU | CdnNode 表 | `AVG(cpu_usage) WHERE cluster_id=? AND status='active'` |
| 平均延迟 | CdnNode 表 | `AVG(latency) WHERE cluster_id=? AND status='active'` |
| 总带宽 | CdnNode 表 | `SUM(bandwidth_mbps) WHERE cluster_id=?` |
| 健康状态 | 计算得出 | 规则见下方 |

**健康状态判定规则**（按优先级从上到下匹配，首条匹配即为最终状态）：

| 优先级 | 条件 | 状态 |
|--------|------|------|
| 1 | 在线节点 = 0 或 在线率 <= 30% | down（大面积故障） |
| 2 | 在线率 < 70% 且 在线率 > 30% | degraded（部分故障） |
| 3 | 平均 CPU >= 85% | overload（严重过载） |
| 4 | 平均 CPU >= 70% | overload（高负载） |
| 5 | 在线率 = 100% 且 平均 CPU < 70% | healthy（健康） |
| 6 | 其余（在线率 >= 70% 且 平均 CPU < 70%） | healthy（健康） |

> **说明**：第 3、4 行覆盖了 CPU 高但在线率正常的场景（如 DDoS/突发流量），第 6 行作为兜底规则保证无遗漏。

> **`Cluster.Status` 自动更新**：健康检查 Goroutine（`services/health_check.go`）每次完成一轮全量节点探测后，应自动更新每个集群的 `Status` 字段，规则为：集群下所有节点均为 `active` 且健康状态为 `healthy` → `active`；部分节点故障（`degraded`）→ `degraded`；全部不可用（`down`）→ `inactive`。运维也可在管理后台手动覆盖此字段。

### 7.2 节点热力图

每个集群一行，用圆点代表节点：

| 节点状态 | 颜色 | 条件 |
|----------|------|------|
| 健康 | 绿色 | status=active, cpu<70%, latency<100ms |
| 高负载 | 黄色 | status=active, cpu>=70% 或 latency>=300ms |
| 故障 | 红色 | status=inactive |
| 离线 | 灰色 | last_heartbeat > 5min 前 |

### 7.3 流量分布

数据来自 `ClusterMetric` 表（由调度器异步写入），展示过去 5 分钟各集群处理请求量和带宽。

---

## 8. 实施计划

### 阶段一：数据层 + 集群 CRUD（预计 3 天）

| 任务 | 产出 |
|------|------|
| 新增 Cluster、RuleCluster、CdnNode 字段变更 | 数据模型迁移脚本 |
| 集群 CRUD 后端 API | `handlers/clusters.go` |
| 集群管理前端页面（列表 + 详情 + 表单） | `frontend/src/pages/Clusters.jsx` |
| SeedData 适配 | 启动时创建默认集群 |

### 阶段二：规则适配 + 节点增强（预计 2 天）

| 任务 | 产出 |
|------|------|
| 规则 API 改为关联集群 | `handlers/rules.go` 修改 |
| 规则编辑 UI 改选集群 | `Rules.jsx` 修改 |
| 节点 API 增强（新字段 + 详情端点） | `handlers/nodes.go` 修改 |
| 节点前端增强（新列 + 筛选 + 详情抽屉） | `Nodes.jsx` 修改 |

### 阶段三：调度视图 + Agent 心跳（预计 2 天）

| 任务 | 产出 |
|------|------|
| 调度视图后端 API | `handlers/views.go` |
| 调度视图前端页面 | `frontend/src/pages/Views.jsx` |
| Agent 心跳端点 | `handlers/node_heartbeat.go` |
| 调度器适配集群模式 | `handlers/redirect.go` 修改 |

---

## 9. 设计决策记录

| 编号 | 决策 | 选项 | 选择理由 |
|------|------|------|----------|
| ADR-01 | 集群代替业务组 | 业务组 / 集群 / 标签 | "集群"是 CDN 行业的通用术语，减少认知成本 |
| ADR-02 | 规则多对多集群 | 一对一 / 多对多 | 支持流量拆分和主备容灾 |
| ADR-03 | 节点保留 region/isp 可覆盖集群 | 继承集群 / 独立设置 | 混合部署场景（如集群跨机房）需要独立覆盖 |
| ADR-04 | 节点新增运维字段一次性到位 | 最小字段 / 完整字段 | 避免反复改表，Agent 上报字段可空 |
| ADR-05 | NodeIDs 保留列不立刻删除 | 立刻删除 / 保留废弃 | 避免原地大表迁移风险，确认零引用后清理 |
| ADR-06 | 调度视图不展示域名 | 展示 / 不展示 | 运维关注的是基础设施健康，域名配置在"域名管理" |
| ADR-07 | 健康状态分四级 | 二级/三级/四级 | 二级(健康/故障)不够精确；四级(degraded)覆盖部分故障场景 |
