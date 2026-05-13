# 302CDN 管理系统 - 增量 PRD v2

## 1. 产品目标

本次迭代聚焦于解决 302CDN 管理系统在生产环境中暴露的**四大核心缺陷**，同时为系统提供必要的**生产级能力**，确保系统可安全、可扩展地对外提供服务。

**核心目标：**
- **多域名适配**：支持不同域名使用同一 ruleKey，实现租户/场景隔离
- **路径完整透传**：修复 302 跳转时路径丢失问题，确保 `/r/img/photo.jpg` 正确跳转到 `cdn.xxx.com/photo.jpg`
- **安全管控**：引入 JWT 认证，防止未授权访问管理后台
- **输入安全**：加强输入校验，防止 XSS、SQL 注入等安全风险

---

## 2. 用户故事

### P0 - 核心缺陷（Must Have）

| # | 用户角色 | 用户故事 |
|---|----------|----------|
| US-01 | 运维工程师 | 作为运维工程师，我希望系统支持多域名绑定，这样可以为不同客户/场景使用相同的 ruleKey 而不产生冲突 |
| US-02 | 前端开发 | 作为前端开发，我希望访问 `/r/img/photo.jpg` 时能跳转到 `cdn.xxx.com/photo.jpg`（而非 `cdn.xxx.com/`），这样才能正确加载静态资源 |
| US-03 | 系统管理员 | 作为系统管理员，我希望管理后台有登录认证，这样未经授权的人无法修改 CDN 配置 |
| US-04 | 安全工程师 | 作为安全工程师，我希望所有用户输入都有严格校验，这样系统的输入安全风险可以被消除 |

### P1 - 生产就绪（Should Have）

| # | 用户角色 | 用户故事 |
|---|----------|----------|
| US-05 | 运维工程师 | 作为运维工程师，我希望节点有自动健康检测和故障剔除，这样故障节点不会被用于服务请求 |
| US-06 | 运维工程师 | 作为运维工程师，我希望配置可以通过 config.yaml 外置，这样修改配置无需重新编译代码 |
| US-07 | 运维工程师 | 作为运维工程师，我希望有 Docker 部署支持，这样我可以快速部署和扩容系统 |
| US-08 | 运维工程师 | 作为运维工程师，我希望 302 响应有 Cache-Control 头，这样下游 CDN 可以缓存跳转减少回源 |
| US-09 | 安全工程师 | 作为安全工程师，我希望有 IP 限流能力，这样我可以防止恶意用户滥用服务 |

### P2 - 体验提升（Nice to Have）

| # | 用户角色 | 用户故事 |
|---|----------|----------|
| US-10 | 运维工程师 | 作为运维工程师，我希望表格支持排序、搜索和批量操作，这样我可以高效管理大量节点和规则 |
| US-11 | 运维工程师 | 作为运维工程师，我希望可以导出数据为 CSV，这样我可以进行离线分析 |
| US-12 | 开发人员 | 作为开发人员，我希望系统支持暗色主题，这样可以减少视觉疲劳 |
| US-13 | 运维工程师 | 作为运维工程师，我希望前后端打包为单一二进制，这样部署更简单 |

---

## 3. 需求池

### P0 - 核心缺陷

#### P0-001: 多域名支持

**详细描述：**
- 在 `RedirectRule` 表新增 `Domain` 字段（VARCHAR(255)，可为空代表泛匹配）
- `GET /r/:ruleKey` 路由改为 `GET /r/:ruleKey`，通过 `X-Forwarded-Host` 或 `Host` Header 获取请求域名
- 匹配逻辑：`Domain` 精确匹配当前请求 Host，或 `Domain` 为空时匹配所有域名
- 若同一 `ruleKey` + `Domain` 组合存在多条规则，按现有 Strategy 执行

**验收标准：**
1. `curl -H "Host: cdn.example.com" http://localhost/r/test` 匹配 `Domain=cdn.example.com` 的规则
2. `curl http://localhost/r/test` (无 Host) 匹配 `Domain=NULL` 的规则
3. 返回的 302 跳转目标正确
4. 数据库迁移后旧数据 `Domain` 字段默认为空字符串
5. 同一 ruleKey 不同 Domain 可独立配置不同 NodeIDs

---

#### P0-002: 请求路径透传

**详细描述：**
- 修改 `GET /r/:ruleKey` 路由，捕获剩余路径：`/r/:ruleKey/*path`
- 跳转逻辑变更：`TargetURL = NodeURL + "/" + path`
- 处理 `/r/img/` 与 `/r/img` 的兼容性问题（统一添加 `/` 分隔符）
- 若 path 为空，则保持原有行为（跳转到 NodeURL）

**验收标准：**
1. `GET /r/img/photo.jpg` 跳转至 `cdn.xxx.com/photo.jpg`
2. `GET /r/img/subfolder/logo.png` 跳转至 `cdn.xxx.com/subfolder/logo.png`
3. `GET /r/img` (无路径) 跳转至 `cdn.xxx.com/` 或 `cdn.xxx.com`（按原 NodeURL 末尾是否有 `/`）
4. 透传的路径不包含 ruleKey 部分
5. URL 编码特殊字符（如空格 `%20`）正确处理

---

#### P0-003: 管理后台登录认证

**详细描述：**
- 新增用户表 `AdminUser`：ID, Username(unique), PasswordHash, CreatedAt
- 新增登录接口 `POST /api/auth/login`，参数 `{username, password}`，返回 JWT Token
- 新增 Token 验证中间件，所有 `/api/nodes/*`、`/api/rules/*` 接口需要 `Authorization: Bearer <token>` Header
- JWT Secret 从配置文件读取（默认使用 config.yaml）
- Token 有效期建议 24 小时（可配置）
- 新增 `POST /api/auth/logout`（可选，标记 Token 失效）

**验收标准：**
1. 正确的用户名密码返回 JWT Token
2. 错误的用户名密码返回 401 Unauthorized
3. 无 Token 或 Token 无效的请求返回 401
4. 已认证请求可正常访问管理接口
5. Token 包含用户 ID 和过期时间信息

---

#### P0-004: 输入校验加强

**详细描述：**
- **规则管理**：
  - `Key`: 必填，3-64 字符，小写字母/数字/连字符，正则 `^[a-z0-9-]{3,64}$`
  - `Description`: 可选，最大 500 字符
  - `Strategy`: 必填，枚举 `round-robin|weighted|random`
  - `Domain`: 可选，最大 255 字符，格式校验为有效域名或为空
- **节点管理**：
  - `Name`: 必填，2-50 字符
  - `URL`: 必填，有效 URL 格式（http/https），最大 2048 字符
  - `Weight`: 必填，1-100 整数
  - `Region`: 可选，最大 50 字符
- **通用**：所有文本输入转义 HTML 特殊字符

**验收标准：**
1. 不符合格式的输入返回 400 Bad Request，包含具体错误信息
2. SQL 注入尝试被拒绝（参数化查询已实现）
3. XSS 注入尝试被转义存储和展示
4. 边界值测试通过（空值、超长、超短、特殊字符）
5. API 文档更新反映新的校验规则

---

### P1 - 生产就绪

#### P1-001: 302 缓存控制头

**详细描述：**
- 302 响应添加 `Cache-Control: private, max-age=300`（5 分钟，可配置）
- 添加 `Vary: Host` Header（确保不同域名缓存隔离）
- 可选支持 `Cache-Control: no-cache` 或 `no-store`（通过配置开关）

**验收标准：**
1. 302 响应包含 `Cache-Control` Header
2. `Vary: Host` Header 存在
3. 可通过配置修改缓存时间
4. 不同域名请求返回正确的 Vary Header

---

#### P1-002: 节点自动健康检测 + 故障剔除

**详细描述：**
- 新增后台任务（Goroutine），每 N 秒（可配置，默认 30s）对 active 节点执行 HTTP HEAD 请求
- 连续失败 M 次（可配置，默认 3 次）的节点自动变更为 `inactive` 状态
- 恢复检测：inactive 节点每 N 秒检测一次，连续成功 M 次后恢复为 `active`
- 检测超时时间可配置（默认 5 秒）
- 节点列表 API `/api/nodes` 支持按 `status` 筛选

**验收标准：**
1. inactive 节点在规则匹配时不会被选中
2. 节点连续失败 3 次后状态变为 inactive
3. inactive 节点恢复成功后状态变回 active
4. 健康检测日志可查询
5. 检测间隔和失败阈值可配置

---

#### P1-003: IP 限流 / 防滥用

**详细描述：**
- 基于 IP 的请求限流，使用滑动窗口算法
- 限流配置：每分钟最多 N 次请求（可配置，默认 60 次/分钟）
- 超限返回 429 Too Many Requests
- 限流豁免：配置白名单 IP 列表
- 记录限流日志（IP、请求路径、时间戳）

**验收标准：**
1. 单 IP 超限返回 429 状态码
2. 正常请求不受限流影响
3. 白名单 IP 不受限制
4. 限流计数器随时间重置
5. 限流事件可查询日志

---

#### P1-004: 配置外置 (config.yaml)

**详细描述：**
- 将硬编码配置迁移至 `config.yaml`
- 配置项：
  ```yaml
  server:
    host: "0.0.0.0"
    port: 8080
  database:
    path: "./data/veer.db"
  jwt:
    secret: "your-secret-key"
    expire_hours: 24
  health_check:
    interval_seconds: 30
    timeout_seconds: 5
    failure_threshold: 3
    recovery_threshold: 3
  rate_limit:
    requests_per_minute: 60
    whitelist:
      - "127.0.0.1"
      - "10.0.0.0/8"
  cache:
    max_age_seconds: 300
  ```
- 支持环境变量覆盖配置（`302CDN_JWT_SECRET` 等）
- 配置缺失时使用默认值

**验收标准：**
1. 系统可通过 config.yaml 启动
2. 环境变量可覆盖 config.yaml 配置
3. 配置错误或缺失时给出友好提示
4. 所有可配置项生效

---

#### P1-005: Docker 部署

**详细描述：**
- 提供 `Dockerfile`（多阶段构建）
  - Build stage: 编译前端 + 后端
  - Runtime stage: 运行最终产物
- 提供 `docker-compose.yml`
  - 包含应用容器 + SQLite 数据持久化 volume
  - 暴露 8080 端口
  - 支持通过环境变量覆盖 config
- 提供 `.dockerignore`

**验收标准：**
1. `docker build` 成功构建镜像
2. `docker-compose up` 启动服务
3. 服务可正常访问（/r/:ruleKey, /api/*）
4. 数据持久化正确
5. 配置文件可通过 volume 挂载覆盖

---

### P2 - 体验提升

#### P2-001: 表格排序/搜索/批量操作

**详细描述：**
- 节点列表：支持按 Name/Region/Status/Weight/CreatedAt 排序
- 规则列表：支持按 Key/Domain/Strategy/HitCount/CreatedAt 排序
- 支持关键词搜索（模糊匹配 Name/Key/URL）
- 节点批量操作：勾选后支持批量删除、批量启用/停用
- 规则批量操作：勾选后支持批量删除
- 分页：每页 10/20/50 条可配置

**验收标准：**
1. 点击列头可切换升序/降序
2. 搜索框输入后 300ms 防抖触发查询
3. 批量选择后显示操作栏
4. 批量删除有确认对话框
5. 分页切换正确

---

#### P2-002: 数据导出 CSV

**详细描述：**
- 节点列表页面添加"导出 CSV"按钮
- 规则列表页面添加"导出 CSV"按钮
- 访问日志页面添加"导出 CSV"按钮
- 导出内容为当前筛选条件下的所有数据
- CSV 编码为 UTF-8 with BOM（兼容 Excel）

**验收标准：**
1. 点击导出按钮下载 CSV 文件
2. CSV 文件可用 Excel 正确打开
3. 导出数据包含所有可见列
4. 导出数据行数与页面显示一致

---

#### P2-003: 暗色主题

**详细描述：**
- MUI ThemeProvider 支持暗色模式切换
- Header 或 Settings 页面添加主题切换按钮
- 主题偏好持久化到 localStorage
- 主题切换无页面闪烁

**验收标准：**
1. 界面可切换为暗色主题
2. 切换后所有组件正确显示
3. 刷新页面后保持主题选择
4. 暗色主题符合色彩对比度要求

---

#### P2-004: 前端构建嵌入后端

**详细描述：**
- 后端增加静态文件服务（`/static/*`）
- 前端 `npm run build` 产物复制到 `embed.FS`
- 单 `go build` 生成单一二进制
- 访问根路径 `/` 返回前端 SPA
- Docker 构建整合此步骤

**验收标准：**
1. 单一二进制可独立运行
2. 访问 `/` 返回前端页面
3. `/api/*` 和 `/r/*` 路由正常
4. 无需单独部署前端服务

---

## 4. UI 设计要求

### 4.1 登录页面

- 新增独立登录页面 `/login`
- 包含 Username、Password 输入框和登录按钮
- 登录失败显示错误提示
- 登录成功后跳转至仪表盘

### 4.2 全局 Header

- 添加用户名显示和退出登录按钮
- 添加暗色主题切换按钮

### 4.3 节点管理页面

- 列表顶部添加搜索框
- 列头可点击排序
- 添加"导出 CSV"按钮
- 列表左侧添加复选框，支持批量选择
- 批量选择后底部出现操作栏（批量删除、批量启用/停用）

### 4.4 规则管理页面

- 列表顶部添加搜索框
- 列头可点击排序
- 添加"导出 CSV"按钮
- 添加 Domain 列显示
- 列表左侧添加复选框，支持批量选择
- 批量选择后底部出现操作栏（批量删除）

### 4.5 访问日志页面

- 列表顶部添加时间范围筛选器
- 添加"导出 CSV"按钮
- 添加分页控件

### 4.6 输入表单校验

- 实时校验输入格式，错误时显示红色边框和错误提示
- 提交时二次校验，阻止无效提交

---

## 5. 待确认问题

| # | 问题 | 背景/原因 |
|---|------|----------|
| Q1 | **JWT Token 存储方式** | 前端应存储在 localStorage 还是 HttpOnly Cookie？HttpOnly 更安全但需要后端配合设置 Cookie |
| Q2 | **初始管理员账户** | 系统首次启动时如何创建第一个管理员？是通过启动脚本、环境变量还是配置文件指定？ |
| Q3 | **限流粒度** | 限流是按单 IP 总请求数，还是按 ruleKey 分别限流？ |
| Q4 | **健康检测失败通知** | 节点变为 inactive 时是否需要发送通知（邮件/钉钉/Webhook）？ |
| Q5 | **Domain 字段默认值** | 历史 ruleKey 的 Domain 字段迁移后默认值是空字符串还是 NULL？是否需要清理脚本？ |
| Q6 | **Docker 镜像仓库** | 镜像发布到哪个仓库（Docker Hub / 私有仓库）？ |
| Q7 | **配置加密** | JWT Secret 等敏感配置是否需要加密存储？还是依赖环境变量覆盖？ |
| Q8 | **灰度发布策略** | 是否需要支持 ruleKey 的灰度流量（百分比分流）？这会影响 P0-002 路径透传逻辑 |

---

## 6. 附录：数据模型变更

### RedirectRule 表变更

```sql
ALTER TABLE redirect_rules ADD COLUMN domain VARCHAR(255) DEFAULT '';
```

### 新增 AdminUser 表

```sql
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 新增限流记录表（可选）

```sql
CREATE TABLE rate_limit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ip VARCHAR(45) NOT NULL,
    request_path VARCHAR(500) NOT NULL,
    blocked BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```
