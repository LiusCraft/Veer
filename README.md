# Veer 管理系统

一个基于 302 跳转原理的 CDN 管理系统，支持多节点负载均衡、路由策略配置和访问日志统计。

## 系统架构

```
客户端请求 → GET /r/:ruleKey → 后端匹配规则 → 选择最优 CDN 节点 → 302 重定向
```

## 功能特性

- **CDN 节点管理**：支持增删改查，健康检测（HTTP 延迟测量）
- **跳转规则管理**：灵活配置路由策略（轮询 / 权重 / 随机）
- **负载均衡**：原子计数器实现 Round-Robin，支持权重随机
- **访问日志**：记录每次跳转，支持分页查询和规则过滤
- **统计仪表盘**：实时概览、7天流量趋势折线图

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21 + Gin v1.9.1 + GORM + SQLite |
| 前端 | Vite + React 18 + MUI v5 + Tailwind CSS + Recharts |

## 快速启动

### 1. 启动后端

```bash
cd backend

# 首次运行：下载依赖
go mod tidy

# 启动服务（端口 8080）
go run .
```

### 2. 启动前端

```bash
cd frontend

# 安装依赖
npm install

# 启动开发服务器（端口 5173）
npm run dev
```

### 3. 访问

打开浏览器访问 [http://localhost:5173](http://localhost:5173)

## API 文档

### 302 跳转

```
GET /r/:ruleKey
```

例：`GET /r/video` → 302 重定向到最优 CDN 节点

### CDN 节点 API

| 方法   | 路径                    | 说明         |
|--------|-------------------------|--------------|
| GET    | /api/nodes              | 节点列表     |
| POST   | /api/nodes              | 新增节点     |
| PUT    | /api/nodes/:id          | 更新节点     |
| DELETE | /api/nodes/:id          | 删除节点     |
| POST   | /api/nodes/:id/test     | 健康检测     |

### 跳转规则 API

| 方法   | 路径            | 说明         |
|--------|-----------------|--------------|
| GET    | /api/rules      | 规则列表     |
| POST   | /api/rules      | 新增规则     |
| PUT    | /api/rules/:id  | 更新规则     |
| DELETE | /api/rules/:id  | 删除规则     |

### 统计 API

| 方法 | 路径                  | 说明               |
|------|-----------------------|--------------------|
| GET  | /api/stats/overview   | 总览统计           |
| GET  | /api/stats/logs       | 访问日志（分页）   |
| GET  | /api/stats/traffic    | 7日流量趋势        |

## 路由策略说明

- **round-robin**：轮询，使用原子计数器均匀分发
- **weighted**：权重随机，节点权重越高命中概率越大
- **random**：完全随机选择节点

## 数据持久化

后端使用 SQLite，数据库文件为 `backend/veer.db`，首次启动自动创建并插入示例数据：
- 3 个示例 CDN 节点（阿里云华东、腾讯云华南、AWS 美国西部）
- 2 条示例跳转规则（video: 权重策略，static: 轮询策略）
