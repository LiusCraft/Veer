# Edge 节点内容改写能力设计

**版本**: v1.0
**日期**: 2026-05-24
**状态**: 实现完成

---

## 1. 概述

### 1.1 目标

为 Veer Edge 节点提供**响应内容改写**能力，使用户无需修改源站即可在边缘侧对 HTTP 响应进行定制化处理，包括：

- 修改/添加/删除响应头
- 替换响应体内容
- 修改响应状态码
- 基于请求上下文（域名、路径、请求头）做条件化改写

### 1.2 两大改写机制

| 机制 | 类型 | 适用场景 | 性能 |
|------|------|----------|------|
| **声明式响应头规则** | 声明式配置 | 安全头（CSP、X-Frame-Options）、缓存控制、CORS | 零开销（纯 map 操作） |
| **Lua 脚本引擎** | 编程式编程 | 响应体替换、条件改写、状态码修改、复杂逻辑 | ~0.5ms/请求（沙箱执行） |

### 1.3 与缓存的关系

改写发生在**缓存命中后**（或回源后）、**响应发送给客户端前**。改写操作会**拷贝 `CacheEntry`**，不修改缓存中的原始数据，因此：

- 缓存 HIT 的响应可以被不同规则（或不同请求参数）改写为不同结果
- 改写后的响应不会被重新缓存

---

## 2. 架构总览

```
       ┌───────────────────────────────────────────────────┐
       │                  proxyHandler                      │
       │  ┌─────────┐  ┌──────────┐  ┌──────────────────┐  │
       │  │ 缓存读取 │  │ transform│  │ 响应头发送      │  │
       │  │ (HIT/   │→│ Response │→│ + 响应体写入     │  │
       │  │  MISS/  │  │ (Lua)    │  │ (io.CopyN)       │  │
       │  │  BYPASS)│  └────┬─────┘  └──────────────────┘  │
       │  └─────────┘       │                              │
       │                    ▼                               │
       │           ┌────────────────┐                      │
       │           │ applyResponse  │                      │
       │           │ Headers        │ ← 声明式头规则       │
       │           └────────────────┘                      │
       └───────────────────────────────────────────────────┘
```

### 2.1 数据流向

```
[Manager API]
    POST/PUT /api/rules (保存 lua_script + response_header_rules 到 DB)
        │
        ▼
[GET /api/edge/rules] (由边缘节点定时同步，默认 60s)
        │
        ▼
[edge/client.go: SyncRules]
    │  ├── 编译 Lua 脚本为 lua.FunctionProto
    │  └── 解析响应头规则 JSON
    │
    ▼
[edge/server.go: proxyHandler]
    │
    ├── 1. s.transformResponse(rule, r, entry)   ← Lua 脚本执行
    │       ├── 解压 body (gzip/deflate)
    │       ├── 执行 transform(req, resp)
    │       ├── 读取修改后的 status_code/headers/body
    │       └── 重新压缩 body (gzip/deflate)
    │
    ├── 2. 写入 entry.Headers 到 w.Header()
    │
    ├── 3. applyResponseHeaders(rule, w)          ← 声明式头规则
    │
    ├── 4. 写入 X-Cache / X-Edge 元数据头
    │
    └── 5. w.WriteHeader + w.Write(body)
```

---

## 3. 数据模型

### 3.1 数据库模型 (`models/redirect_rule.go`)

```go
type RedirectRule struct {
    // ... 通用字段 ...

    // Per-domain cache & rewrite configuration
    CacheTTLSeconds      *int   `json:"cache_ttl_seconds"`
    CacheControlOverride string `json:"cache_control_override"`
    BypassCache          bool   `json:"bypass_cache"`

    // ResponseHeaderRules: JSON 数组，e.g.
    // [{"action":"set","name":"X-Frame-Options","value":"DENY"}]
    ResponseHeaderRules string `json:"response_header_rules" gorm:"type:text"`

    // LuaScript: Lua 脚本源码明文，需定义 transform(req, resp) 函数
    LuaScript string `json:"lua_script" gorm:"type:text"`
}
```

### 3.2 API 传输结构 (`manager/edge_reg.go`)

```go
type EdgeRule struct {
    Domain               string                `json:"domain"`
    OriginBaseURL        string                `json:"origin_base_url"`
    CacheTTLSeconds      *int                  `json:"cache_ttl_seconds"`
    CacheControlOverride string                `json:"cache_control_override"`
    BypassCache          bool                  `json:"bypass_cache"`
    ResponseHeaders      []edgeResponseHeaders `json:"response_headers"`
    LuaScript            string                `json:"lua_script"`
}

type edgeResponseHeaders struct {
    Action string `json:"action"` // "set" | "add" | "remove"
    Name   string `json:"name"`
    Value  string `json:"value"`
}
```

### 3.3 边缘节点运行时结构 (`edge/server.go`)

```go
type domainRule struct {
    originBaseURL        string
    cacheTTLSeconds      *int
    cacheControlOverride string
    bypassCache          bool
    responseHeaders      []responseHeaderRule  // 声明式响应头规则
    luaScript            string
    luaProto             *lua.FunctionProto    // 预编译的 Lua 函数原型
}

type responseHeaderRule struct {
    Action string `json:"action"` // "set" | "add" | "remove"
    Name   string `json:"name"`
    Value  string `json:"value"`
}
```

---

## 4. 声明式响应头规则

### 4.1 实现（`server.go:applyResponseHeaders`）

```go
func applyResponseHeaders(rules []responseHeaderRule, w http.ResponseWriter) {
    for _, h := range rules {
        switch h.Action {
        case "set":
            w.Header().Set(h.Name, h.Value)
        case "add":
            w.Header().Add(h.Name, h.Value)
        case "remove":
            w.Header().Del(h.Name)
        }
    }
}
```

### 4.2 支持的操作

| Action | 语义 | 对应 Go 方法 |
|--------|------|-------------|
| `set` | 设置（覆盖已有值） | `Header.Set()` |
| `add` | 追加（允许同名多值） | `Header.Add()` |
| `remove` | 删除 | `Header.Del()` |

### 4.3 执行时机

在 `proxyHandler` 中**所有**响应路径的最后阶段执行：

1. Lua 脚本先执行（可能修改 headers）
2. Lua 修改后的 headers 写入 `w.Header()`
3. **声明式规则后执行**，覆盖同名字段 — 因此声明式规则优先级高于 Lua 脚本的 header 修改

### 4.4 Cache-Control 覆盖

`CacheControlOverride` 字段是独立机制，在 `fetchFromOrigin` 之后、`transformResponse` 之前执行：

```go
if rule.cacheControlOverride != "" {
    entry.Headers.Set("Cache-Control", rule.cacheControlOverride)
}
```

这意味着 Lua 脚本可以再次修改 `Cache-Control`。最终值取决于上述执行顺序。

---

## 5. Lua 脚本引擎

### 5.1 生命周期

```
规则同步时:
  LuaScript (string) ──编译──→ lua.FunctionProto (预编译字节码)
                                  │
                                  ▼
                            存放在 domainRule.luaProto 中

每个请求处理时:
  luaProto ──加载到 LState ──→ 执行 transform(req, resp) ──→ 读取修改结果
                     │
               从 lStatePool 获取
               执行后归还到 pool
```

### 5.2 编译阶段（`script_engine.go:compileLuaScript`）

```
输入: Lua 源码
过程:
  1. 创建临时 LState（仅加载 base/table/string/math 安全库）
  2. parse.Parse() 解析源码 → AST
  3. lua.Compile() 编译 AST → FunctionProto
  4. 执行 chunk 注册 transform() 函数
  5. 验证 LState 中存在 transform 函数
输出: *lua.FunctionProto（预编译字节码）
```

编译失败时**日志记录但不阻止规则生效**（`scriptEngine.proto` 设为 nil，后续请求跳过脚本执行）。

### 5.3 执行阶段（`script_engine.go:applyLuaScript`）

```
输入: scriptEngine（含 proto）, lStatePool, scriptReq, scriptResp, timeout

过程:
  1. 检查 Content-Encoding
     ├── gzip → gunzip → 执行 → gzip → 覆盖 body
     ├── deflate → inflate → 执行 → deflate → 覆盖 body
     └── br → 跳过脚本执行（不支持 brotli）
  
  2. 从 pool 获取 LState
  
  3. 加载预编译的 transform 函数
  
  4. 构建 Lua 数据结构:
     req = { method="GET", path="/index.html", query="foo=bar", headers={} }
     resp = { status_code=200, headers={}, body="..." }
  
  5. 设置超时上下文（默认 500ms）
  
  6. 调用 transform(req, resp) → 返回 resp table
  
  7. 读取修改:
     ├── status_code (number)
     ├── headers (table string→string)
     └── body (string)
  
  8. 如需重新压缩: gzip/inflate
  
  9. 删除 content-length（让 Go 自动设置）

  10. 归还 LState 到 pool

容错: 任何错误 → 恢复原始 body → 日志记录 → 不中断请求（fail-open）
```

### 5.4 Lua 接口定义

```lua
-- transform 函数签名
function transform(req, resp) -> table

-- req 只读
req = {
    method:  string,  -- "GET" / "HEAD"
    path:    string,  -- e.g. "/index.html"
    query:   string,  -- e.g. "foo=bar&baz=1"
    headers: table,   -- key → value (string→string), 小写化
}

-- resp 可读写
resp = {
    status_code: number,  -- HTTP 状态码
    headers:     table,   -- key → value (string→string), 可修改
    body:        string,  -- 响应体字符串, 可替换
}

-- 必须返回 resp table（通过 return 或直接修改参数均可）
```

### 5.5 Lua 沙箱安全

| 维度 | 措施 |
|------|------|
| **库限制** | 仅加载 `base`、`table`、`string`、`math` |
| **禁用功能** | `io`、`os`、`debug`、`loadlib` 不可用 |
| **超时** | `context.WithTimeout` 500ms 硬超时 |
| **资源** | `lStatePool` 池化管理，防止 OOM |
| **容错** | 抛错时恢复原始响应，请求不中断 |

### 5.6 内容编码处理

| 原始编码 | 行为 |
|----------|------|
| 无（identity） | 直接处理 body |
| gzip | 解压 → 处理 → 重新 gzip 压缩 |
| deflate | 解压 → 处理 → 重新 deflate 压缩 |
| br (brotli) | **跳过脚本执行**（不支持），响应原样返回 |
| 其他 | 直接处理 body |

---

## 6. 响应管线执行顺序

```go
// proxyHandler 中每个路径的执行顺序（以 HIT 为例）:
func (s *EdgeServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 缓存查找或回源
    entry, ok := s.cache.Get(cacheKey)

    // 2. 【Lua 脚本改写】- 修改 status_code / headers / body
    entry = s.transformResponse(rule, r, entry)

    // 3. 【写入 Lua 修改后的 headers】到 ResponseWriter
    for k, v := range entry.Headers {
        w.Header()[k] = v
    }

    // 4. 【声明式头规则】- set/add/remove 响应头
    //    （优先级高于 Lua 的 header 修改）
    applyResponseHeaders(rule.responseHeaders, w)

    // 5. 元数据头
    w.Header().Set("X-Cache", "HIT")
    w.Header().Set("X-Edge", s.cfg.Name)

    // 6. 发送响应
    w.WriteHeader(entry.StatusCode)
    w.Write(entry.Body)
}
```

**重要结论**: 声明式头规则的优先级高于 Lua 脚本对同一 header 的修改。

---

## 7. 前端集成

### 7.1 响应头规则编辑

在 `Rules.jsx` 中以动态表单存在：

```
┌─────────────────────────────────────────┐
│ 响应头改写                              │
│ ┌─────────┬─────────────┬────────────┐  │
│ │ set     │ X-Frame-Options │ DENY   │  │
│ ├─────────┼─────────────┼────────────┤  │
│ │ remove  │ X-Powered-By│            │  │
│ └─────────┴─────────────┴────────────┘  │
│ [+ 添加响应头规则]                      │
└─────────────────────────────────────────┘
```

保存时序列化为 JSON 字符串存入 `response_header_rules` 字段。

### 7.2 Lua 脚本编辑

```
┌──────────────────────────────────────────────┐
│ Lua 脚本 (可选)                               │
│ ┌──────────────────────────────────────────┐  │
│ │ function transform(req, resp)            │  │
│ │     -- 在此编写改写逻辑                  │  │
│ │     -- req.method, req.path, req.headers │  │
│ │     -- resp.status_code, resp.headers    │  │
│ │     -- resp.body 可读写                  │  │
│ │     return resp                          │  │
│ └──────────────────────────────────────────┘  │
└──────────────────────────────────────────────┘
```

---

## 8. 典型场景示例

### 8.1 添加安全头

```json
// 声明式规则
[
  {"action": "set", "name": "X-Frame-Options", "value": "DENY"},
  {"action": "set", "name": "X-Content-Type-Options", "value": "nosniff"},
  {"action": "set", "name": "Referrer-Policy", "value": "strict-origin-when-cross-origin"}
]
```

### 8.2 响应体文本替换

```lua
function transform(req, resp)
    resp.body = string.gsub(resp.body, "http://", "https://")
    resp.headers["x-rewrite"] = "http-to-https"
    return resp
end
```

### 8.3 条件化改写（基于 User-Agent）

```lua
function transform(req, resp)
    local ua = req.headers["user-agent"]
    if ua and string.find(ua, "Mobile") then
        resp.body = string.gsub(resp.body, "/desktop/", "/mobile/")
        resp.headers["x-variant"] = "mobile"
    end
    return resp
end
```

### 8.4 自定义响应（缓存降级）

```lua
function transform(req, resp)
    if resp.status_code >= 500 then
        resp.status_code = 200
        resp.body = "<!-- service degraded --> cached version"
        resp.headers["x-cache-degraded"] = "true"
    end
    return resp
end
```

---

## 9. 与现有设计文档的差异

| 差异项 | 设计文档描述（`edge-disk-cache-v1.md`） | 实际实现 |
|--------|----------------------------------------|----------|
| 改写位置 | 设计文档未覆盖 | `transformResponse()` 在 `proxyHandler` 中每次请求都执行 |
| 缓存修改 | 设计文档未说明 | 拷贝 `CacheEntry`，不修改缓存中的数据 |
| 执行顺序 | 设计文档未定义 | Lua 先执行 → 写入 headers → 声明式规则 → 写入最终响应 |
| Content-Encoding 处理 | 设计文档未覆盖 | 自动解压→Lua 处理→重新压缩 |
| 声明式规则优先级 | 设计文档未定义 | 声明式规则后执行，优先级高于 Lua 的 header 修改 |
| fail-open 策略 | - | Lua 错误时恢复原始 body，不中断请求 |

---

## 10. 性能特征

| 操作 | 耗时估计 | 说明 |
|------|---------|------|
| 声明式头规则（3 条） | < 1μs | 纯 map 操作，零 GC 压力 |
| Lua 脚本编译（规则同步时） | ~1ms | 仅在规则变更时执行一次 |
| Lua 脚本执行（空脚本） | ~20μs | 函数调用 + 返回 |
| Lua 脚本执行（string.gsub） | ~50-200μs | 取决于 body 大小 |
| gzip 解压 + 重新压缩（1KB body） | ~100μs | 主要开销在压缩 |

---

## 11. 行业标准对标与可选扩展方向

Edge 节点内容改写能力在 CDN 行业有成熟对标。以下列出 CDN 行业常见的边缘计算能力及其在 Veer 当前架构下的可行性分析：

### 11.1 请求路径改写（Originless Rewrite）

| 行业对标 | 实现方式 |
|----------|----------|
| Cloudflare Page Rules | 静态 URL 转发规则 |
| Akamai Edge Redirector | 条件 + 目标路径模板 |
| Varnish vmod_rewrite | 正则替换回源 URL |

**Veer 当前缺失**: `fetchFromOrigin` 中 `targetURL` 构造时不做任何路径修改。Lua 脚本只接收只读的 `req.path`，无法修改。

```go
// server.go:336 — 当前实现
targetURL := strings.TrimRight(originBaseURL, "/") + path
```

**可行性**: 低——此能力与 scheduler 的 `url_redirect` 规则类型功能重叠，应集中到调度层而非边缘层实现。

### 11.2 请求头修改（Request Header Rewrite）

| 行业对标 | 实现方式 |
|----------|----------|
| Cloudflare Transform Rules | `set`/`remove` 请求头 |
| Fastly VCL | `set req.http.X-Custom = "value"` |
| AWS CloudFront | Origin Request Policy |

**Veer 当前缺失**: 回源请求（`fetchFromOrigin` 中的 `req`）不做任何 header 修改。Lua `req.headers` 是只读的。

```go
// server.go:338 — 回源请求构造时没有应用任何 header 规则
req, err := http.NewRequest("GET", targetURL, nil)
```

**行业需求**:
- 回源时附加客户端真实 IP（`X-Forwarded-For`）
- 附加认证令牌（`X-Origin-Auth`）
- 移除隐私相关的请求头（`Referer`、`Cookie`）

**实现路径**:
- `domainRule` 新增 `requestHeaders []requestHeaderRule`
- `fetchFromOrigin` 中 `Set`/`Add`/`Del` 回源请求头
- 前端表单复用 `responseHeaderRule` 的表单组件

### 11.3 Brotli 支持（Content-Encoding 扩展）

| 行业对标 | 支持情况 |
|----------|----------|
| Cloudflare | 自动解压 + Brotli 压缩 |
| Fastly | 仅转码，不自动解压 |
| Varnish + libbrotli | 通过 vmod 支持 |

**Veer 当前行为**: 遇到 `br` 编码时跳过脚本执行。

```go
// script_engine.go:218-221
case "br":
    log.Printf("[edge] lua: brotli not supported, skipping script")
    return
```

**行业趋势**: Brotli 在 2025 年已占 Web 流量的 ~75%，且 `compress/gzip` 是 Go 标准库，而 brotli 需要 `andybalholm/brotli` 外部依赖。

**实现路径**:
- 引入 `github.com/andybalholm/brotli`
- `script_engine.go` 添加 `br` case 的解压/压缩
- 注意 Go 标准库不包含 brotli，需评审依赖引入

### 11.4 基于路径的多脚本路由（Per-Path Script Dispatch）

| 行业对标 | 实现方式 |
|----------|----------|
| Cloudflare Workers | 单一 Worker 内路由分发 |
| Fastly Compute@Edge | 单一二进制内路由 |
| OpenResty | `location` 块绑定不同 Lua 代码 |

**Veer 当前**: 每个域名绑定一个 Lua 脚本，无法按路径分发。

**行业模式**: CDN 边缘计算最核心的设计模式是"单一入口 + 内部路由"，而非"每路由绑定不同代码"。

**建议**: 通过 Lua 脚本内部路由实现，无需改动引擎：

```lua
function transform(req, resp)
    if string.match(req.path, "^/api/") then
        -- API 响应改写
    elseif string.match(req.path, "^/static/") then
        -- 静态资源改写
    end
    return resp
end
```

### 11.5 Lua 运行时扩展：缓存 API

| 行业对标 | KV 存储 |
|----------|---------|
| Cloudflare Workers KV | 全球 KV 存储 |
| Fastly KV Store | 边缘 KV |
| OpenResty `shared dict` | 本机共享字典 |

**Veer 当前**: Lua 无法读写缓存。K-V 存储是边缘计算的标准能力，用于存储配置、令牌、计数等。

**实现路径**:
- 在 Lua 中注册 `cache.get(key)` / `cache.set(key, value, ttl)` 函数
- 映射到 `RamTier.Get` / `RamTier.Set`
- `value` 限制为 string，最大 1MB
- 注意同步问题：多个 LState 实例同时访问同一 key

### 11.6 可配置超时

**Veer 当前**: 硬编码 500ms 超时。

```go
// script_engine.go:20
const defaultScriptTimeout = 500 * time.Millisecond
```

**行业实践**: CDN 边缘计算的超时通常可配置为 5ms-5s。

**实现路径**:
- `domainRule` 新增 `scriptTimeout *int`
- `EdgeRule` API 新增 `script_timeout_ms` 字段
- `applyLuaScript` 将 timeout 参数从读取规则中获取
- 前端表单新增"脚本超时（毫秒）"输入框

### 11.7 响应体流式改写（Streaming Transform）

| 行业对标 | 实现方式 |
|----------|----------|
| Cloudflare Workers | `TransformStream` API |
| Fastly Compute@Edge | Streaming body |
| Varnish + vmod | 流式 filter |

**Veer 当前**: 全部读入内存后再处理。

```go
// 整个 body 作为一个 string 传入 Lua
resp.Body = string(entry.Body)
```

**行业需求**: 大文件（>100MB 视频/安装包）的实时替换场景，例如 JS/CSS 注入、水印。

**技术挑战**:
- Lua VM 当前不支持流式输入（`gopher-lua` 的限制）
- 需要换用 `go-lua` + 自定义 reader/writer
- 或引入 `luajit` 通过 FFI 操作 buffer

**建议**: 初期不做。Veer 场景下 cache body 已全部在内存中（源站响应已 `io.ReadAll`），流式改写的意义有限。

---

## 12. 行业参考总结

| 能力 | Cloudflare | Fastly | Akamai / OpenResty | Veer 当前 | 优先级 |
|------|-----------|--------|-------------------|-----------|--------|
| 响应头改写 | ✅ Transform Rules | ✅ VCL `set resp.http.*` | ✅ EdgeWorkers | ✅ 声明式 + Lua | - |
| 响应体改写 | ✅ Workers | ✅ Compute@Edge | ✅ Lua | ✅ Lua | - |
| 响应状态码修改 | ✅ Workers | ✅ VCL | ✅ Lua | ✅ Lua | - |
| 请求头改写 | ✅ Transform Rules | ✅ VCL `set req.http.*` | ✅ Lua | ❌ | P2 |
| 请求路径改写 | ✅ Page Rules | ✅ VCL | ✅ Lua | 🔶 scheduler 层有 | P3 |
| Brotli 解压 | ✅ 自动 | ✅ 转码 | ✅ | ❌ 跳过 | P2 |
| 边缘 KV 存储 | ✅ Workers KV | ✅ KV Store | ✅ shared dict | ❌ | P3 |
| 流式改写 | ✅ TransformStream | ✅ Streaming body | ✅ Lua | ❌ | P4 |
| 可配置超时 | ✅ Workers 配置 | ✅ VCL | ✅ | ❌ 硬编码 500ms | P1 |

---

**文档版本**: v1.0
**最后更新**: 2026-05-24
**状态**: 实现完成
