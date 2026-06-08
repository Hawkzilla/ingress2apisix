# NGINX Ingress 注解迁移全局策略

## 1. 概述

本文档从全局视角梳理 **所有已知 NGINX Ingress Controller 注解**，按三种自动化能力进行分类：

| 分类 | 说明 | 典型输出 |
|------|------|----------|
| **A. 直接注解转换** | NGINX 注解 → APISIX 原生注解，由 ingress2apisix converter 直接完成 | Ingress YAML（含 `k8s.apisix.apache.org/*` 注解） |
| **B. 自动转换 CRD** | NGINX 注解 → APISIX CRD 资源，由 converter 自动生成 YAML | ApisixPluginConfig / BackendTrafficPolicy / ApisixUpstream / ApisixRoute / ApisixTls |
| **C. 需要插件开发/功能增强** | APISIX 生态中尚无等价方案，需开发 Lua 插件或扩展 AIC 注解 | 新 Lua 插件 / AIC 新注解 / 全局 config |

> **核心结论**：目前标记为 MANUAL 的注解中，约 **60%** 可通过自动生成 CRD 转为全自动；真正需要插件开发的仅约 **15%**。

---

## 2. 分类一：直接注解转换（→ APISIX 原生注解）

converter.go 的 `convertAnnotations()` 函数直接将 NGINX 注解映射为 `k8s.apisix.apache.org/*` 原生注解，**不需要生成额外 CRD 文件**。

| # | NGINX 注解 | APISIX 原生注解 | 状态 | hxk8s1 使用 |
|---|-----------|----------------|------|-------------|
| 1 | `enable-cors` | `k8s.apisix.apache.org/enable-cors` | ✅ 已实现 | 1 |
| 2 | `cors-allow-origin` | `k8s.apisix.apache.org/cors-allow-origin` | ✅ 已实现 | 1 |
| 3 | `cors-allow-methods` | `k8s.apisix.apache.org/cors-allow-methods` | ✅ 已实现 | 1 |
| 4 | `cors-allow-headers` | `k8s.apisix.apache.org/cors-allow-headers` | ✅ 已实现 | 1 |
| 5 | `ssl-redirect` | `k8s.apisix.apache.org/http-to-https` | ✅ 已实现 | 35 |
| 6 | `force-ssl-redirect` | `k8s.apisix.apache.org/http-to-https` | ✅ 已实现 | 0 |
| 7 | `proxy-redirect-from` + `proxy-redirect-to` | `k8s.apisix.apache.org/http-to-https`（仅 HTTP→HTTPS 场景） | ✅ 已实现 | 0 |
| 8 | `rewrite-target` | `k8s.apisix.apache.org/rewrite-target` 或 `rewrite-target-regex` | ✅ 已实现 | 75 |
| 9 | `proxy-connect-timeout` | `k8s.apisix.apache.org/upstream-connect-timeout` | ✅ 已实现 | 17 |
| 10 | `proxy-send-timeout` | `k8s.apisix.apache.org/upstream-send-timeout` | ✅ 已实现 | 26 |
| 11 | `proxy-read-timeout` | `k8s.apisix.apache.org/upstream-read-timeout` | ✅ 已实现 | 40 |
| 12 | `backend-protocol` | `k8s.apisix.apache.org/upstream-scheme` | ✅ 已实现 | 235 |
| 13 | `whitelist-source-range` | `k8s.apisix.apache.org/allowlist-source-range` | ✅ 已实现 | 1 |
| 14 | `denylist-source-range` | `k8s.apisix.apache.org/blocklist-source-range` | ✅ 已实现 | 0 |
| 15 | `auth-url` | `k8s.apisix.apache.org/auth-uri` | ✅ 已实现 | 0 |
| 16 | `auth-method` | `k8s.apisix.apache.org/auth-method` | ✅ 已实现 | 0 |
| 17 | `auth-request-headers` | `k8s.apisix.apache.org/auth-request-headers` | ✅ 已实现 | 0 |
| 18 | `auth-response-headers` | `k8s.apisix.apache.org/auth-upstream-headers` | ✅ 已实现 | 0 |
| 19 | `auth-signin` | `k8s.apisix.apache.org/auth-signin` | ✅ 已实现 | 0 |
| 20 | `auth-type` | `k8s.apisix.apache.org/auth-type`（basic→basicAuth） | ✅ 已实现 | 2 |
| 21 | `auth-secret` | `k8s.apisix.apache.org/auth-secret` | ✅ 已实现 | 2 |
| 22 | `websocket-services` | `k8s.apisix.apache.org/enable-websocket` | ✅ 已实现 | 0 |
| 23 | `use-regex` | `k8s.apisix.apache.org/use-regex` | ✅ 已实现 | 269 |
| 24 | `enable-access-log` | `k8s.apisix.apache.org/enable-access-log` | ✅ 已实现 | 5 |
| 25 | `permanent-redirect` | `k8s.apisix.apache.org/http-redirect` + `http-redirect-code: 308` | ✅ 已实现 | 0 |
| 26 | `temporal-redirect` | `k8s.apisix.apache.org/http-redirect` + `http-redirect-code: 302` | ✅ 已实现 | 0 |
| 27 | `app-root` | `k8s.apisix.apache.org/http-redirect` | ✅ 已实现 | 0 |
| 28 | `custom-http-errors` | `k8s.apisix.apache.org/custom-error-codes`（需 custom-error-page 插件） | ✅ 已实现 | 0 |
| 29 | `configuration-snippet`（单条 rewrite） | `k8s.apisix.apache.org/rewrite-target-regex` + `rewrite-target-regex-template` | ✅ 已实现 | 3 |

**小计：29 种注解，全部已实现自动转换。**

---

## 3. 分类二：自动转换 CRD（→ APISIX CRD 资源）

这类注解无法仅通过注解映射完成，需要 converter **额外生成一个或多个 CRD YAML 文件**。当前部分已实现，部分需要开发。

### 3.1 ApisixPluginConfig（插件配置 CRD）

这些注解的 APISIX 等价物是某个插件的配置，目前 converter 已经会生成 `ApisixPluginConfig`。

| # | NGINX 注解 | APISIX 插件 | 状态 | hxk8s1 |
|---|-----------|------------|------|--------|
| 1 | `limit-rps` | `limit-req` plugin | ✅ 已实现 | 230 |
| 2 | `limit-rpm` | `limit-req` plugin (rate: N/min) | ✅ 已实现 | 230 |
| 3 | `limit-connections` | `limit-conn` plugin | ✅ 已实现 | 0 |
| 4 | `limit-multiplier` | 乘算 limit-rps/limit-rpm | ✅ 已实现 | 230 |
| 5 | `proxy-body-size` | `client-control` plugin (max_body_size) | ✅ 已实现 | 71 |
| 6 | `cors-allow-credentials` | `cors` plugin (allow_credential) | ✅ 已实现 | 1 |
| 7 | `cors-max-age` | `cors` plugin (max_age) | ✅ 已实现 | 1 |
| 8 | `upstream-vhost` | `proxy-rewrite` plugin (host) | ✅ 已实现 | 2 |
| 9 | `configuration-snippet`（多条 rewrite） | `proxy-rewrite` plugin (多个 rules) | ✅ 已实现 | 3 |
| 10 | `proxy-cookie-path` | 自定义 `proxy-cookie-path` 插件 | ✅ 已实现 | 0 |
| 11 | `enable-real-ip` | `real-ip` plugin (source, real_ip_from) | ✅ 已实现 | 5 |
| 12 | `use-forwarded-headers` | `real-ip` plugin (recursive=true) | ✅ 已实现 | 1 |
| 13 | `compute-full-forwarded-for` | `real-ip` plugin (append) | ✅ 已实现 | 1 |
| 14 | `forwarded-for-header` | `real-ip` plugin (source) | ✅ 已实现 | 1 |
| 15 | `ssl-verify` | ApisixUpstream TLS 或 proxy-ssl | ✅ 已实现（含警告） | 2 |
| 16 | `session-cookie-hash` | 自定义 `session-cookie-hash` 插件 | ✅ 已实现 | 2 |
| 17 | `session-cookie-expires` | 扩展 session-cookie-hash 插件 | ✅ 已实现 | 2 |
| 18 | `session-cookie-max-age` | 扩展 session-cookie-hash 插件 | ✅ 已实现 | 2 |
| 19 | `session-cookie-path` | 扩展 session-cookie-hash 插件 | ✅ 已实现 | 2 |
| 20 | `session-cookie-conditional-samesite-none` | 生成警告 | ✅ 已实现 | 2 |

**小计：20 种注解，全部已实现 ApisixPluginConfig 生成。**

### 3.2 BackendTrafficPolicy（流量策略 CRD）

这些注解需要生成 `BackendTrafficPolicy` CRD 来实现负载均衡、会话亲和、健康检查等。

| # | NGINX 注解 | APISIX 等价物 | 状态 | hxk8s1 | 自动化难度 |
|---|-----------|--------------|------|--------|-----------|
| 1 | `upstream-hash-by` | `loadBalancer.type=chash` | ✅ 已实现 | 4 | — |
| 2 | `health-check-interval` | `healthCheck.active.interval` | ✅ 已实现 | 1 | — |
| 3 | `health-check-path` | `healthCheck.active.httpPath` | ✅ 已实现 | 1 | — |
| 4 | `health-check-retries` | `healthCheck.active.healthy.successes` | ✅ 已实现 | 1 | — |
| 5 | `health-check-timeout` | `healthCheck.active.timeout` | ✅ 已实现 | 1 | — |
| 6 | `affinity` + `session-cookie-name` | `loadBalancer.type=chash` + `hashOn=cookie` | 🔧 需开发 | 6 | 低 — 字段直接映射 |
| 7 | `affinity-mode` | `loadBalancer.type` 选择（persistent→chash, balanced→ewma） | 🔧 需开发 | 4 | 低 |
| 8 | `upstream-keepalive-connections` | `keepalive` 配置 | 🔧 需开发 | 5 | 低 — 字段直接映射 |
| 9 | `upstream-keepalive-requests` | `keepalive` 配置 | 🔧 需开发 | 8 | 低 — 字段直接映射 |
| 10 | `upstream-keepalive-timeout` | `keepalive` 配置 | 🔧 需开发 | 7 | 低 — 字段直接映射 |

**小计：10 种注解，5 种已实现，5 种需开发（难度低）。**

#### affinity + session-cookie-name 实现方案

```
// NGINX Ingress
nginx.ingress.kubernetes.io/affinity: "cookie"
nginx.ingress.kubernetes.io/session-cookie-name: "route"
nginx.ingress.kubernetes.io/affinity-mode: "persistent"

// 转换为 BackendTrafficPolicy CRD
apiVersion: apisix.apache.org/v1
kind: BackendTrafficPolicy
metadata:
  name: <ingress-name>-session-affinity
spec:
  loadBalancer:
    type: chash
    hashOn: cookie
    key: route           # session-cookie-name 的值
```

#### upstream-keepalive 实现方案

```
// NGINX Ingress
nginx.ingress.kubernetes.io/upstream-keepalive-connections: "32"
nginx.ingress.kubernetes.io/upstream-keepalive-requests: "100"
nginx.ingress.kubernetes.io/upstream-keepalive-timeout: "60s"

// 转换为 BackendTrafficPolicy CRD
apiVersion: apisix.apache.org/v1
kind: BackendTrafficPolicy
metadata:
  name: <ingress-name>-keepalive
spec:
  keepalive:
    connections: 32
    requests: 100
    timeout: 60s
```

### 3.3 ApisixUpstream（上游 CRD）

| # | NGINX 注解 | APISIX 等价物 | 状态 | hxk8s1 | 自动化难度 |
|---|-----------|--------------|------|--------|-----------|
| 1 | `upstream-keepalive-*` | ApisixUpstream keepalive 配置 | 与 3.2 共用方案 | 5~8 | 低 |

> **注**：keepalive 既可放在 BackendTrafficPolicy 也可放在 ApisixUpstream，推荐使用 BackendTrafficPolicy 统一管理。

### 3.4 ApisixRoute（路由 CRD）

这些注解在 NGINX 中控制金丝雀发布逻辑，需要生成额外的 ApisixRoute 或修改现有路由。

| # | NGINX 注解 | APISIX 等价物 | 状态 | hxk8s1 | 自动化难度 |
|---|-----------|--------------|------|--------|-----------|
| 1 | `canary` | ApisixRoute 条件路由 | 🔧 需开发 | 1 | 中 |
| 2 | `canary-by-header` | ApisixRoute `vars` 匹配 | 🔧 需开发 | 1 | 中 |
| 3 | `canary-by-header-value` | 配合 canary-by-header | 🔧 需开发 | 1 | 中 |
| 4 | `canary-by-cookie` | ApisixRoute `vars` 匹配 cookie | 🔧 需开发 | 0 | 中 |
| 5 | `canary-weight` | `traffic-split` 插件权重配置 | 🔧 需开发 | 0 | 中 |

#### canary 实现方案

NGINX Ingress 的金丝雀是通过 **两个 Ingress 对象** 实现的：一个正常 Ingress + 一个带 canary 注解的 Ingress。APISIX 需要：

```
// 方案：拆分为主路由 + canary 子路由

// 主 ApisixRoute（正常流量）
apiVersion: apisix.apache.org/v1
kind: ApisixRoute
spec:
  http:
  - name: main-route
    match:
      hosts: [example.com]
      paths: [/api/*]
    backends:
    - serviceName: my-service-stable
      servicePort: 80

// Canary ApisixRoute（条件流量）
apiVersion: apisix.apache.org/v1
kind: ApisixRoute
spec:
  http:
  - name: canary-route
    priority: 100   # 更高优先级
    match:
      hosts: [example.com]
      paths: [/api/*]
      exprs:
      - subject:
          name: X-Agent-Style
          scope: Header
        op: Equal
        value: normal
    backends:
    - serviceName: my-service-canary
      servicePort: 80
```

**自动化挑战**：需要同时识别正常 Ingress 和对应 canary Ingress，合并为一组 ApisixRoute。建议 converter 生成一对 ApisixRoute 文件。

### 3.5 ApisixTls（TLS CRD）

| # | NGINX 注解 | APISIX 等价物 | 状态 | hxk8s1 | 自动化难度 |
|---|-----------|--------------|------|--------|-----------|
| 1 | `ssl-passthrough` | ApisixTls + stream_proxy | 🔧 需开发 | 8 | 中 — 需同时配置 APISIX 全局 |
| 2 | `ssl-protocols` | ApisixTls `spec.ssl.ssl_protocols` | 🔧 需开发 | 0 | 低 — 字段直接映射 |
| 3 | `ssl-ciphers` | ApisixTls `spec.ssl.ssl_ciphers` | 🔧 需开发 | 0 | 低 — 字段直接映射 |
| 4 | `auth-tls-secret` | ApisixTls mTLS 配置 | 🔧 需开发 | 0 | 中 |
| 5 | `auth-tls-verify-client` | ApisixTls client_verify | 🔧 需开发 | 0 | 中 |

#### ssl-passthrough 实现方案

```
// 1. 生成 ApisixTls CRD
apiVersion: apisix.apache.org/v1
kind: ApisixTls
metadata:
  name: my-domain-tls
spec:
  hosts: [my-domain.com]
  ssl:
    client:
      caSecret: ...
      depth: 1
  tcp:
  - host: my-domain.com
    service:
    - name: my-backend
      port: 443

// 2. 警告用户需在 APISIX config.yaml 中启用 stream_proxy
// apisix:
//   stream_proxy:
//     tcp:
//       - "443"
```

---

## 4. 分类三：需要插件开发/功能增强

这类注解在 APISIX 生态中尚无完整等价方案，需要投入开发资源。

### 4.1 需要 AIC 新增注解支持

这些功能 APISIX 已有对应插件/配置能力，但 AIC（APISIX Ingress Controller）尚未提供注解入口。需要在 AIC 中新增原生注解。

| # | NGINX 注解 | APISIX 已有能力 | AIC 缺口 | 开发工作量 | hxk8s1 |
|---|-----------|----------------|---------|-----------|--------|
| 1 | `hsts` | `response-rewrite` 插件设置 `Strict-Transport-Security` 头 | AIC 无 hsts 相关注解 | 小 — 新增注解 + 插件映射 | 0 |
| 2 | `hsts-max-age` | 配合 hsts | 同上 | 小 | 0 |
| 3 | `hsts-include-subdomains` | 配合 hsts | 同上 | 小 | 0 |
| 4 | `hsts-preload` | 配合 hsts | 同上 | 小 | 0 |
| 5 | `connection-proxy-header` | `proxy-rewrite` 插件 headers 配置 | AIC 无 Connection 头注解 | 小 | 0 |
| 6 | `x-forwarded-prefix` | `proxy-rewrite` 插件添加 X-Forwarded-Prefix | AIC 无此注解 | 小 | 0 |
| 7 | `preserve-trailing-slash` | `proxy-rewrite` 插件 uri 配置 | AIC 无此注解 | 小 | 0 |
| 8 | `mirror-target` | `proxy-mirror` 插件 | AIC 无 mirror 注解 | 小 | 0 |
| 9 | `mirror-path` | 配合 proxy-mirror | 同上 | 小 | 0 |
| 10 | `session-cookie-conditional-samesite-none` | APISIX `proxy-cookie-flags` 插件 | AIC 无 SameSite 控制注解 | 小 | 2 |

#### 实现示例（以 hsts 为例）

```go
// AIC internal/adc/translator/annotations/types.go 新增常量
const AnnotationsHSTS = "k8s.apisix.apache.org/hsts"
const AnnotationsHSTSMaxAge = "k8s.apisix.apache.org/hsts-max-age"
const AnnotationsHSTSIncludeSubdomains = "k8s.apisix.apache.org/hsts-include-subdomains"
const AnnotationsHSTSPreload = "k8s.apisix.apache.org/hsts-preload"

// AIC plugins/hsts.go handler
// 将注解值转换为 response-rewrite 插件配置：
// headers.add = ["Strict-Transport-Security: max-age=31536000; includeSubDomains; preload"]

// ingress2apisix converter.go
// hsts: "true" + hsts-max-age: "31536000" →
//   k8s.apisix.apache.org/hsts: "31536000; includeSubDomains; preload"
```

### 4.2 需要 Lua 插件开发

这些功能在 APISIX 核心/插件库中完全没有等价物，需要编写自定义 Lua 插件。

| # | NGINX 注解 | 功能描述 | 实现方案 | 开发工作量 | hxk8s1 |
|---|-----------|---------|---------|-----------|--------|
| 1 | `server-snippet` 中的 `upstream` 块 | 自定义 upstream 定义（权重、主备） | ApisixUpstream CRD 可部分覆盖，但复杂场景需 Lua 插件解析 | 中 | 1 |
| 2 | `proxy-set-headers`（ConfigMap 引用） | 引用 ConfigMap 中的 header 值注入请求 | 需 AIC 能读取 ConfigMap 并传入 proxy-rewrite 插件，或开发独立插件 | 大 | 2 |
| 3 | `enable-owasp-modsecurity-crc` | ModSecurity WAF 集成 | APISIX 需部署 `waf` 插件或对接外部 WAF | 大 — 需独立部署 | 0 |
| 4 | `modsecurity-snippet` | ModSecurity 自定义规则 | 同上 | 大 | 0 |
| 5 | `modsecurity-transaction-id` | ModSecurity 事务 ID 透传 | 同上 | 大 | 0 |
| 6 | `satisfy` | 多认证插件 "any/all" 语义 | APISIX `multi-auth` 插件或自定义逻辑 | 中 | 0 |

> **注**：`server-snippet` 中的 upstream 块虽然技术上可以手动用 ApisixUpstream 表达，但如果用户在 snippet 中写了复杂的 `if` 条件逻辑、`proxy_cookie_flags`、`more_set_headers` 等，仍需要逐条分析，无法完全自动化。hxk8s1 中发现的 `server-snippet` 内容是权重 upstream，可转为 ApisixUpstream CRD，工作量较低。

### 4.3 需要全局 APISIX 配置

这些注解对应的是 NGINX `http/server` 级别配置，无法通过 per-route 注解或 CRD 实现，需要修改 APISIX 全局配置文件（`config.yaml`）。

| # | NGINX 注解 | APISIX config.yaml 等价 | 状态 | hxk8s1 | 是否可自动化 |
|---|-----------|------------------------|------|--------|-------------|
| 1 | `proxy-buffering` | `nginx_config.http_configuration_snippet: proxy_buffering off;` | 需全局配置 | 2 | ⚠️ 可生成配置片段提示 |
| 2 | `proxy-request-buffering` | `nginx_config.http_configuration_snippet: proxy_request_buffering off;` | 需全局配置 | 4 | ⚠️ 可生成配置片段提示 |
| 3 | `proxy-buffer-size` | `nginx_config.http_configuration_snippet: proxy_buffer_size 4k;` | 需全局配置 | 0 | ⚠️ 可生成配置片段提示 |
| 4 | `proxy-buffers-number` | `nginx_config.http_configuration_snippet: proxy_buffers 4 8k;` | 需全局配置 | 0 | ⚠️ 可生成配置片段提示 |
| 5 | `proxy-http-version` | `nginx_config.http_upstream: proxy_http_version 1.1;` | 需全局配置 | 2 | ⚠️ 可生成配置片段提示 |
| 6 | `service-upstream` | `apisix.enable_server: true` 或 DNS 解析模式 | 需全局配置 | 0 | ❌ 不可自动 |
| 7 | `disable-access-log` | `nginx_config.http.access_log off;` | 需全局配置 | 0 | ❌ 不可自动 |
| 8 | `retry-non-idempotent` | APISIX 全局 `proxy_retry` 配置 | 需全局配置 | 0 | ❌ 不可自动 |
| 9 | `enable-opentracing` | APISIX `opentelemetry` 插件全局配置 | 需全局配置 | 0 | ❌ 不可自动 |
| 10 | `enable-influxdb` | APISIX `prometheus` / `node-status` 插件 | 需全局配置 | 0 | ❌ 不可自动 |

> **建议**：对于 `proxy-buffering` 和 `proxy-request-buffering`，ingress2apisix 可以在转换输出中自动生成一个 APISIX config.yaml 配置片段建议文件，提示用户合并到全局配置中。

---

## 5. 现状总结与自动化升级路径

### 5.1 当前自动化率

| 分类 | 数量 | 占比 |
|------|------|------|
| A. 直接注解转换（✅ 已实现） | 29 | 30% |
| B. 自动转换 CRD（✅ 已实现 PluginConfig） | 20 | 21% |
| B. 自动转换 CRD（✅ 已实现 BackendTrafficPolicy） | 5 | 5% |
| **当前已自动化小计** | **54** | **56%** |

### 5.2 可升级为自动化的注解

| 分类 | 数量 | 预计工作量 | 优先级 |
|------|------|-----------|--------|
| B → 自动化 BackendTrafficPolicy（affinity + keepalive） | 5 | 低 | P1 |
| B → 自动化 ApisixTls（ssl-passthrough/protocols/ciphers/mTLS） | 5 | 低~中 | P2 |
| B → 自动化 ApisixRoute（canary 注解） | 5 | 中 | P2 |
| C → AIC 新增注解（hsts、mirror、cookie-flags 等） | 10 | 小（每项 1~2 天） | P3 |
| C → 全局配置自动生成提示文件 | 4 | 小 | P4 |

### 5.3 升级后的自动化率

| 阶段 | 已自动化 | 需插件开发 | 总计 | 自动化率 |
|------|---------|-----------|------|---------|
| **当前** | 54 | 41 | 95 | 56% |
| **+ CRD 自动化（P1）** | 59 | 36 | 95 | 62% |
| **+ CRD 自动化（P2）** | 69 | 26 | 95 | 73% |
| **+ AIC 注解（P3）** | 79 | 16 | 95 | 83% |
| **+ 全局配置提示（P4）** | 83 | 12 | 95 | 87% |

> 剩余 12 项为 ModSecurity（6）、proxy-set-headers ConfigMap（1）、service-upstream（1）、disable-access-log（1）、retry-non-idempotent（1）、satisfy（1）、tracing（1），均为低频注解或需独立基础设施配合。

---

## 6. 行动计划

### P1 — 立即可做（预计 1~2 天）

1. **affinity + session-cookie-name → BackendTrafficPolicy**
   - converter.go `buildBackendTrafficPolicies()` 新增 session affinity 逻辑
   - 输出 BackendTrafficPolicy CRD：`loadBalancer.type=chash`, `hashOn=cookie`, `key=<session-cookie-name>`
   - scanner.go 将 affinity/session-cookie-name 从 MANUAL 升级为 CONVERTED

2. **upstream-keepalive-* → BackendTrafficPolicy**
   - converter.go `buildBackendTrafficPolicies()` 新增 keepalive 逻辑
   - 输出 BackendTrafficPolicy CRD：`keepalive.connections/requests/timeout`
   - scanner.go 将 upstream-keepalive-* 从 MANUAL 升级为 CONVERTED

3. **配置类注解自动生成提示文件**
   - `proxy-buffering`、`proxy-request-buffering`、`proxy-buffer-size`、`proxy-buffers-number` 转换时生成 config.yaml 配置片段建议

### P2 — 短期（预计 3~5 天）

4. **ssl-passthrough/protocols/ciphers → ApisixTls CRD**
   - converter.go 新增 `buildApisixTlses()` 函数
   - scanner.go 从 MANUAL 升级

5. **canary-* → ApisixRoute CRD**
   - converter.go 新增 canary Ingress 识别和 ApisixRoute 拆分逻辑
   - 需要解决"关联 Ingress"问题：同一 Service 的正常 Ingress 和 canary Ingress

6. **affinity-mode: balanced → BackendTrafficPolicy loadBalancer.type=ewma**

### P3 — 中期（预计 5~10 天）

7. **AIC 新增 hsts 注解** — 四个注解映射到 `response-rewrite` 插件
8. **AIC 新增 mirror 注解** — 映射到 `proxy-mirror` 插件
9. **AIC 新增 cookie-flags 注解** — 映射到 `proxy-cookie-flags` 插件
10. **AIC 新增 x-forwarded-prefix 注解** — 映射到 `proxy-rewrite` 插件
11. **AIC 新增 connection-proxy-header 注解** — 映射到 `proxy-rewrite` 插件
12. **AIC 新增 preserve-trailing-slash 注解** — 映射到 `proxy-rewrite` 插件

### P4 — 长期（需评估投入产出比）

13. **ModSecurity WAF 集成** — 需独立部署 WAF 基础设施
14. **proxy-set-headers ConfigMap 读取** — 需 AIC 支持读取任意 ConfigMap
15. **satisfy 多认证语义** — 需 APISIX multi-auth 插件增强
16. **OpenTracing/InfluxDB** — APISIX 已有 opentelemetry 插件，需全局配置自动化

---

## 7. hxk8s1 集群特殊发现

### 7.1 拼写错误

| 错误注解 | 出现次数 | 正确注解 | 处理建议 |
|----------|----------|----------|----------|
| `configuration-snipper` | 2 | `configuration-snippet` | ingress2apisix 增加 typo 容错，或提示用户修正 |
| `proxy-http-verson` | 2 | `proxy-http-version` | 同上 |

### 7.2 auth-realm 特殊性

hxk8s1 中 2 个 Ingress 使用了 `auth-realm: "401: Authentication Required"`。注意当前 AIC 已支持 `k8s.apisix.apache.org/auth-realm` 注解，APISIX `forward-auth.lua` 插件已支持 `realm` 字段注入 `WWW-Authenticate` 头。**但 scanner.go 中仍标记为 MANUAL**，需同步更新。

### 7.3 configuration-snippet 内容

hxk8s1 的 3 个 `configuration-snippet` 包含：
- `set $proxy_upstream_name` — APISIX 原生处理，可忽略
- `add_header Content-Security-Policy` — 需 `response-rewrite` 插件
- `access_log` 指令 — 需全局日志配置

---

## 8. 附录：完整注解清单

### A 类：直接注解转换（29 种）

```
enable-cors, cors-allow-origin, cors-allow-methods, cors-allow-headers,
ssl-redirect, force-ssl-redirect, proxy-redirect-from, proxy-redirect-to,
rewrite-target, proxy-connect-timeout, proxy-send-timeout, proxy-read-timeout,
backend-protocol, whitelist-source-range, denylist-source-range,
auth-url, auth-method, auth-request-headers, auth-response-headers,
auth-signin, auth-type, auth-secret, auth-realm,
websocket-services, use-regex, enable-access-log,
permanent-redirect, temporal-redirect, app-root,
custom-http-errors, configuration-snippet (单条 rewrite)
```

### B 类：自动转换 CRD（30 种）

**ApisixPluginConfig（20 种）：**
```
limit-rps, limit-rpm, limit-connections, limit-multiplier,
proxy-body-size, cors-allow-credentials, cors-max-age,
upstream-vhost, configuration-snippet (多条 rewrite),
proxy-cookie-path, enable-real-ip, use-forwarded-headers,
compute-full-forwarded-for, forwarded-for-header, ssl-verify,
session-cookie-hash, session-cookie-expires, session-cookie-max-age,
session-cookie-path, session-cookie-conditional-samesite-none
```

**BackendTrafficPolicy（10 种）：**
```
upstream-hash-by, health-check-interval, health-check-path,
health-check-retries, health-check-timeout,
affinity, session-cookie-name, affinity-mode,
upstream-keepalive-connections, upstream-keepalive-requests,
upstream-keepalive-timeout
```

**ApisixRoute（5 种）：**
```
canary, canary-weight, canary-by-header, canary-by-header-value, canary-by-cookie
```

**ApisixTls（5 种）：**
```
ssl-passthrough, ssl-protocols, ssl-ciphers, auth-tls-secret, auth-tls-verify-client
```

### C 类：需要插件开发/功能增强（26 种）

**需 AIC 新增注解（10 种）：**
```
hsts, hsts-max-age, hsts-include-subdomains, hsts-preload,
connection-proxy-header, x-forwarded-prefix, preserve-trailing-slash,
mirror-target, mirror-path, session-cookie-conditional-samesite-none (AIC 侧)
```

**需 Lua 插件开发（6 种）：**
```
proxy-set-headers (ConfigMap), satisfy,
enable-owasp-modsecurity-crc, modsecurity-snippet, modsecurity-transaction-id,
server-snippet (复杂场景)
```

**需全局 APISIX 配置（10 种）：**
```
proxy-buffering, proxy-request-buffering, proxy-buffer-size, proxy-buffers-number,
proxy-http-version, service-upstream, disable-access-log,
retry-non-idempotent, enable-opentracing, enable-influxdb
```
