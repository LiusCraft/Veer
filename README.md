# Veer 管理系统

一个基于 302 跳转原理的 CDN 管理系统，支持多节点负载均衡、路由策略配置、HTTP 缓存代理和访问日志统计。

## 系统架构

```
客户端请求 → GET /r/:ruleKey → Scheduler(8081) 匹配规则 → 选择最优 CDN 节点 → 302 重定向
                        ↕ 规则同步
Manager(8080) ←→ Edge(8082) HTTP 缓存代理 → 回源
```

系统由三个独立二进制组成：

| 组件 | 端口 | 职责 |
|------|------|------|
| **Manager** (`veer`) | 8080 | 管理后台 API（节点、规则、统计、JWT 认证） |
| **Scheduler** (`scheduler`) | 8081 | 数据面 302 跳转，规则匹配与负载均衡 |
| **Edge** (`edge`) | 8082 | HTTP 缓存代理节点，支持本地缓存 + 磁盘缓存 |

## 功能特性

- **CDN 节点管理**：增删改查，健康检测（后台 Goroutine 自动检测）
- **跳转规则管理**：灵活配置路由策略（轮询 / 权重 / 随机），支持域名匹配
- **全局负载均衡**：原子计数器 Round-Robin，权重随机选择
- **HTTP 缓存代理**（Edge 节点）：内存 + 磁盘缓存，L1/L2 分层，Bloom 过滤器，自动压缩
- **JWT 认证**：登录保护所有管理 API
- **IP 限流**：滑动窗口 60 次/分钟，白名单豁免
- **自动健康检测**：30 秒间隔，3 次连续失败自动下线节点
- **访问日志**：记录每次跳转，支持分页查询和规则过滤
- **统计仪表盘**：实时概览、7 日流量趋势折线图
- **暗色主题**：支持亮/暗主题切换

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21 + Gin v1.9.1 + GORM v1.25.7 + SQLite |
| 后端依赖 | JWT v5、Viper、bcrypt、滑动窗口限流 |
| 前端 | Vite 5 + React 18 + MUI v5 + Tailwind CSS + Recharts |
| 部署 | Docker Compose（manager + scheduler + edge × 2） |

## 快速启动

### 方式一：本地开发

需要 CGO_ENABLED=1（SQLite 依赖）。

**1. 启动 Manager（管理 API）**

```bash
cd backend
cp config-manager.yaml config.yaml
go run ./cmd/manager
# 启动在 http://localhost:8080
```

**2. 启动 Scheduler（302 跳转）**

```bash
cd backend
cp config-scheduler.yaml config.yaml
go run ./cmd/scheduler
# 启动在 http://localhost:8081
```

**3. 启动 Edge 节点（HTTP 缓存代理）**

```bash
cd backend
cp config-edge.yaml config.yaml
go run ./cmd/edge
# 启动在 http://localhost:8082
```

**4. 启动前端**

```bash
cd frontend
npm install
npm run dev
# 启动在 http://localhost:5173（自动代理 /api 到 :8080）
```

### 方式二：Docker Compose

```bash
cd backend
docker compose up -d
# Manager:  http://localhost:8080
# Scheduler: http://localhost:8081
# Edge-1:    http://localhost:8082
# Edge-2:    http://localhost:8083
```

### 3. 访问

打开浏览器访问 [http://localhost:5173](http://localhost:5173)

默认管理员：`admin` / `admin123`

## 配置

配置优先级：**环境变量 > config.yaml > 代码默认值**

环境变量前缀：`CDNC_`（例如 `CDNC_SERVER_PORT=9000`）

各组件配置文件：
- Manager: `backend/config-manager.yaml`
- Scheduler: `backend/config-scheduler.yaml`
- Edge: `backend/config-edge.yaml`

## API 文档

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 管理员登录（无需 JWT） |
| POST | `/api/auth/logout` | 登出 |

### 302 跳转（Scheduler，公开访问）

```
GET /r/:ruleKey
GET /r/:ruleKey/*path   # 路径透传
```

例：`GET /r/video/images/logo.png` → 302 到最优 CDN 节点 + 路径拼接

### CDN 节点 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes` | 节点列表 |
| POST | `/api/nodes` | 新增节点 |
| PUT | `/api/nodes/:id` | 更新节点 |
| DELETE | `/api/nodes/:id` | 删除节点 |
| POST | `/api/nodes/:id/test` | 单次健康检测 |

### 跳转规则 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/rules` | 规则列表 |
| POST | `/api/rules` | 新增规则 |
| PUT | `/api/rules/:id` | 更新规则 |
| DELETE | `/api/rules/:id` | 删除规则 |

### 统计 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/stats/overview` | 总览统计 |
| GET | `/api/stats/logs` | 访问日志（分页） |
| GET | `/api/stats/traffic` | 7 日流量趋势 |

### 健康检测

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/config/health-check` | 获取健康检测状态 |
| POST | `/api/config/health-check/toggle` | 手动触发健康检测 |

## 路由策略

- **round-robin**：轮询，原子计数器均匀分发
- **weighted**：权重随机，节点权重越高命中概率越大
- **random**：完全随机选择节点

## Edge 节点

Edge 节点启动时向 Manager 注册，每 60 秒同步规则。作为 HTTP 缓存代理运行：
- L1 内存缓存 + L2 磁盘缓存（Ristretto + Badger）
- Bloom 过滤器减少缓存穿透
- 缓存淘汰：LRU + 分片 compaction
- 支持 Cache-Control 响应头
- Manager 不可达时使用本地配置运行

## 数据持久化

后端使用 SQLite，数据库文件为 `backend/veer.db`，首次启动自动创建并插入示例数据：
- 3 个示例 CDN 节点（阿里云华东、腾讯云华南、AWS 美国西部）
- 2 条示例跳转规则（video: 权重策略，static: 轮询策略）

## 构建

```bash
./build.sh                           # 构建 Manager 二进制
cd backend && go build ./cmd/manager # 构建 Manager
go build -o scheduler ./cmd/scheduler
go build -o edge ./cmd/edge
```
