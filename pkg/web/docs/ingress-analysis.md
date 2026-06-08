# 整体 Ingress 迁移分析报告

> 基于集群现有 Ingress 资源的注解分析，生成时间: 2026-05-18

## 1. 总览

| 指标 | 数值 |
|------|------|
| Ingress 总数 | ~280 |
| 涉及命名空间数 | ~90 |
| 涉及 NGINX 注解类型数 | ~45 |
| 无注解/无 NGINX 注解的 Ingress 数 | ~40 |
| 厂商自定义注解类型数 | 1 |

## 2. 注解迁移状态分类

> 以下注解均使用 `nginx.ingress.kubernetes.io/` 或 `ingress.kubernetes.io/` 前缀，表格中省略前缀。
> 分类依据: `pkg/charts/scanner.go` 中的注解映射规则。

### 2.1 可自动转换（CONVERTED）

这些注解可直接映射到 APISIX 原生注解或 CRD 字段，工具自动处理。

| 注解名称 | 使用 Ingress 数 | 映射说明 |
|----------|----------------|----------|
| `backend-protocol` | ~80 | → `k8s.apisix.apache.org/upstream-scheme`（http/HTTPS） |
| `rewrite-target` | ~30 | → `k8s.apisix.apache.org/rewrite-target`（proxy-rewrite 插件） |
| `ssl-redirect` | ~30 | → `k8s.apisix.apache.org/http-to-https` |
| `use-regex` | ~80 | → `k8s.apisix.apache.org/use-regex`（路由匹配模式） |
| `enable-access-log` | ~3 | → `k8s.apisix.apache.org/enable-access-log` |
| `upstream-hash-by` | ~4 | → BackendTrafficPolicy CRD (chash 负载均衡) |
| `ssl-verify` | ~1 | → 发出警告并指导配置 ApisixUpstream TLS |

**小计: 7 种注解，覆盖约 228 次使用**

### 2.2 插件自动转换（PLUGIN_CONFIG）

这些注解会自动生成 `ApisixPluginConfig` CRD 资源，工具自动处理。

| 注解名称 | 使用 Ingress 数 | 对应 APISIX 插件 |
|----------|----------------|------------------|
| `limit-rpm` | ~80 | `limit-req` 插件（rate: N/min，自动乘算 limit-multiplier） |
| `limit-rps` | ~80 | `limit-req` 插件（自动乘算 limit-multiplier） |
| `limit-multiplier` | ~80 | 配合 limit-rps/limit-rpm，自动计算实际限流值 |
| `proxy-body-size` | ~40 | `client-control` 插件（max_body_size） |
| `proxy-read-timeout` | ~30 | upstream timeout 配置 |
| `proxy-send-timeout` | ~30 | upstream timeout 配置 |
| `proxy-connect-timeout` | ~20 | upstream timeout 配置 |
| `affinity` | ~5 | `session-cookie-hash` 插件 |
| `session-cookie-hash` | ~5 | `session-cookie-hash` 插件 |
| `session-cookie-name` | ~5 | `session-cookie-hash` 插件 |
| `session-cookie-expires` | ~1 | 扩展 session-cookie-hash 插件（max_age） |
| `session-cookie-max-age` | ~1 | 扩展 session-cookie-hash 插件（max_age 优先） |
| `session-cookie-path` | ~1 | 扩展 session-cookie-hash 插件（cookie_path） |
| `affinity-mode` | ~2 | `session-cookie-hash` 插件（配合 affinity） |
| `enable-cors` | ~1 | `cors` 插件 |
| `cors-allow-origin` | ~1 | `cors` 插件 |
| `cors-allow-methods` | ~1 | `cors` 插件 |
| `cors-allow-headers` | ~1 | `cors` 插件 |
| `cors-allow-credentials` | ~1 | `cors` 插件 |
| `cors-max-age` | ~1 | `cors` 插件 |
| `upstream-vhost` | ~2 | `proxy-rewrite` 插件（host） |
| `configuration-snippet` | ~3 | `proxy-rewrite` 插件或自定义插件 |
| `proxy-cookie-path` | ~1 | 自定义插件（proxy-cookie-path） |
| `whitelist-source-range` | ~1 | `ip-restriction` 插件 |
| `enable-real-ip` | ~5 | `real-ip` 插件（source, real_ip_from） |
| `use-forwarded-headers` | ~1 | 配合 real-ip 插件（recursive=true） |
| `compute-full-forwarded-for` | ~1 | 配合 real-ip 插件（append 模式，发出警告） |
| `forwarded-for-header` | ~1 | 配合 real-ip 插件（source 配置） |

**小计: 28 种注解，覆盖约 400 次使用**

### 2.3 需手动迁移（MANUAL）

这些注解需要人工介入，手动配置 APISIX 等价功能。

#### 2.3.1 流量管理

| 注解名称 | 使用 Ingress 数 | 迁移指导 |
|----------|----------------|----------|
| `upstream-keepalive-connections` | ~8 | → ApisixUpstream CRD 或全局配置 apisix.upstream.keepalive |
| `upstream-keepalive-requests` | ~8 | → ApisixUpstream CRD 或全局配置 apisix.upstream.keepalive_requests |
| `upstream-keepalive-timeout` | ~8 | → ApisixUpstream CRD 或全局配置 apisix.upstream.keepalive_timeout |
| `proxy-buffering` | ~1 | → 需全局 nginx snippet `proxy_buffering` 配置 |
| `proxy-request-buffering` | ~2 | → 需全局 nginx snippet `proxy_request_buffering` 配置 |
| `proxy-http-version` | ~1 | → APISIX upstream HTTP 版本配置 |
| `proxy-set-headers` | ~1 | → 需 `proxy-rewrite` 插件 `headers.set`，但值在 ConfigMap 中，需手动迁移 |
| `service-upstream` | ~5 | → APISIX upstream 配置 |

#### 2.3.2 安全与认证

| 注解名称 | 使用 Ingress 数 | 迁移指导 |
|----------|----------------|----------|
| `ssl-passthrough` | ~5 | → 需 APISIX TCP stream 代理配置 |
| `auth-type` | ~1 | → APISIX `basic-auth` 插件，配合 `ApisixConsumer` CRD |
| `auth-secret` | ~1 | → 需 `ApisixConsumer` CRD 配合 auth-type 注解 |

#### 2.3.3 流量分割（灰度发布）

| 注解名称 | 使用 Ingress 数 | 迁移指导 |
|----------|----------------|----------|
| `canary` | ~1 | → 需 APISIX canary 路由方案（traffic-split 插件） |
| `canary-by-header` | ~1 | → 需 APISIX canary 路由方案 |
| `canary-by-header-value` | ~1 | → 需 APISIX canary 路由方案 |

#### 2.3.4 日志与监控

| 注解名称 | 使用 Ingress 数 | 迁移指导 |
|----------|----------------|----------|
| `server-snippet` | ~1 | → 需 APISIX 全局 snippet，NGINX 特有语法需逐行翻译 |

**小计: 15 种注解，覆盖约 45 次使用**

### 2.4 厂商/未知注解（UNKNOWN）

这些是非标准或厂商自定义的注解，不属于官方 NGINX Ingress Controller。

| 注解名称 | 使用 Ingress 数 | 说明 |
|----------|----------------|------|
| `nginx.ingress.harmonycloud.cn/rewrite-params` | ~5 | HarmonyCloud 厂商自定义注解，APISIX 无等价实现，需评估具体功能后手动迁移 |

**小计: 1 种注解，覆盖约 5 次使用**

### 2.5 无 NGINX 注解

约 40 个 Ingress 仅包含 `kubernetes.io/ingress.class` 或 `spec.ingressClassName` 等非功能注解，不含任何 NGINX 特有注解。这些 Ingress 在迁移时仅需修改 `ingressClassName` 字段即可。

## 3. 高频使用注解 TOP 10

| 排名 | 注解名称 | 使用次数 | 迁移状态 | 说明 |
|------|----------|----------|----------|------|
| 1 | `backend-protocol` | ~80 | ✅ CONVERTED | 大量 Ingress 使用 HTTPS 后端协议 |
| 2 | `limit-multiplier` | ~80 | ✅ PLUGIN_CONFIG | 自动乘算 limit-rps/limit-rpm 值 |
| 3 | `limit-rpm` | ~80 | ✅ PLUGIN_CONFIG | 限流插件，自动转换 |
| 4 | `limit-rps` | ~80 | ✅ PLUGIN_CONFIG | 限流插件，自动转换 |
| 5 | `use-regex` | ~80 | ✅ CONVERTED | 正则路由匹配 |
| 6 | `proxy-body-size` | ~40 | ✅ PLUGIN_CONFIG | 请求体大小限制 |
| 7 | `proxy-read-timeout` | ~30 | ✅ PLUGIN_CONFIG | 读超时 |
| 8 | `proxy-send-timeout` | ~30 | ✅ PLUGIN_CONFIG | 发送超时 |
| 9 | `ssl-redirect` | ~30 | ✅ CONVERTED | SSL 重定向 |
| 10 | `rewrite-target` | ~30 | ✅ CONVERTED | 路径重写 |

> TOP 10 注解全部可自动转换，覆盖约 510 次使用。

## 4. 迁移建议

### 4.1 整体评估

| 迁移状态 | 注解种类数 | 占比 | 使用次数 | 占比 |
|----------|-----------|------|----------|------|
| ✅ 可自动转换（CONVERTED） | 7 | 15.6% | ~228 | ~23.3% |
| ✅ 插件自动转换（PLUGIN_CONFIG） | 28 | 62.2% | ~400 | ~40.8% |
| ⚠️ 需手动迁移（MANUAL） | 15 | 33.3% | ~45 | ~4.6% |
| ❌ 厂商/未知（UNKNOWN） | 1 | 2.2% | ~5 | ~0.5% |

> **注**: 手动迁移注解种类从 30 种减少到 15 种，约 30 次使用迁移改为自动处理。高频注解已全部可自动转换。

### 4.2 分阶段迁移策略

**第一阶段: 自动转换（低风险，覆盖广）**

直接使用工具自动转换，覆盖约 64% 的注解使用次数：
- `backend-protocol`、`use-regex`、`ssl-redirect`、`rewrite-target`、`enable-access-log` → 原生 APISIX 注解
- `limit-rps`、`limit-rpm`、`limit-multiplier`、`proxy-body-size`、`proxy-*-timeout` 等 → PluginConfig CRD
- `enable-real-ip`、`forwarded-for-header` → `real-ip` 插件
- `upstream-hash-by` → BackendTrafficPolicy CRD (chash)
- `health-check-*` → BackendTrafficPolicy CRD (healthCheck)
- `session-cookie-expires/max-age/path` → 扩展 session-cookie-hash 插件
- CORS、会话亲和等 → 对应 APISIX 插件

**第二阶段: 批量手动迁移（中风险，需评估）**

按类别批量处理，每类涉及 1-8 个 Ingress：
- **Keepalive 类**: `upstream-keepalive-*` → 统一配置 ApisixUpstream CRD 或全局 upstream
- **认证类**: `auth-secret` → ApisixConsumer CRD
- **流量分割**: `canary-*` → traffic-split 插件

**第三阶段: 逐个处理（高风险，需定制）**

需要逐一评估和测试：
- `ssl-passthrough` → APISIX TCP stream 代理
- `server-snippet` → APISIX 全局 snippet 或自定义插件
- `proxy-buffering`、`proxy-request-buffering` → 全局 nginx snippet

### 4.3 注意事项

1. **limit-multiplier 已自动处理**: 该注解配合 `limit-rps`/`limit-rpm` 使用，约 80 个 Ingress 使用。工具现在自动计算实际限流值（原始值 × 倍数），无需手动调整。

2. **厂商注解处理**: `nginx.ingress.harmonycloud.cn/rewrite-params` 为 HarmonyCloud 厂商特有注解，需联系业务方确认具体功能后决定迁移方案。

3. **配置片段翻译**: `configuration-snippet`（3 处）和 `server-snippet`（1 处）包含 NGINX 原生配置语法，需要逐行翻译为 APISIX Lua 插件或全局配置。

4. **会话亲和兼容性**: `session-cookie-expires`、`session-cookie-max-age`、`session-cookie-path` 已自动扩展到 session-cookie-hash 插件配置。`session-cookie-conditional-samesite-none` 无法直接映射，工具会发出警告指导手动处理。

5. **风险评估**: 高频注解（TOP 10）全部可自动转换，整体迁移风险显著降低。剩余手动注解影响范围小（合计仅约 45 个 Ingress 使用 15 种注解）。
