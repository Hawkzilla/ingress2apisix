# hxk8s1 集群 Ingress 注解迁移分析报告

## 1. 数据概览

| 指标 | 数值 |
|------|------|
| Ingress 总数 | 421 |
| 有注解的 Ingress | 345 |
| 无注解的 Ingress | 76 |
| 命名空间数 | 75 |
| 唯一注解类型数 | 58 |

**命名空间列表（按首字母）：**
abs-sit-2, abs-t-2, accs-dev, accs-sit2, agent-manager, agentplatform, agentplatform-t4, buds-t2, buds-t4, caas-system, cbcbp-dev, cbcbp-hf, cbcbp-t2, cbfb-t2, company-credit, company-credit-t2dg, cop-cors, default, dev, dmtyw-dev, ecif-app, emla-monitoring-system, emp-sit2, emp-t2, fgncs-jspt-app-ft, fgncs-jspt-app-hf, ft, ft-t3, gisplat-sit1, gscf-app, harbor, huawei-agent, iars-dev, iars-sit, ibios-sit2, ibios-t2, ibios-t4, ingress, iotp-t2, iotp-t4, istio-system, khxxpt-t2, kube-system, ljtest-t2, monitoring-vm, mportal-im-t2, natp-dev, natp-et, natp-sit2, natp-t1, natp-t3, ntgls-app, personal-credit, pms-dev, pms-t4, rcsp, rhtx, rjcsgl-t4, servicemesh, stellaris-system, t2-rtc, ucms-dev, ucts-t2, vela-system, xcptcs, xhx-cpcd-app-t4, xhx-jspt-app, xhx-jspt-app-ft, xwjr, xxts-2, xyd-xyk-sitb, xyd-xyk-t2, xyd-xyk-t4, xykxs, ydsq-dev

---

## 2. 注解分类总览

| 分类 | 含义 | 数量 | 占比 |
|------|------|------|------|
| **A. 直接转换** | ingress2apisix 已能自动完成转换（输出注解/PluginConfig/CRD） | 38 | 65.5% |
| **B. 需开发AIC注解** | APISIX 插件或 CRD 能力已有，但 AIC（APISIX Ingress Controller）缺注解入口 | 12 | 20.7% |
| **C. 需插件适配** | APISIX 无等价插件，需开发新 Lua 插件或做功能增强 | 2 | 3.4% |
| **D. 需全局配置** | 对应 NGINX http/server 级配置，无法 per-route 控制 | 3 | 5.2% |
| **E. 拼写错误** | 注解 key 拼写有误，需先修正 | 2 | 3.4% |
| **F. 未使用（标记）** | NGINX Ingress 官方支持但 hxk8s1 集群未使用 | ~76 | — |

> **结论：** 65.5% 可直接批量转换，20.7% 需在 AIC 上新增注解支持（APISIX 插件已就绪），3.4% 需新插件开发。整体迁移难度 **中等偏低**。

---

## 3. 完整注解清单

### A. 直接转换（ingress2apisix 已实现）

这些注解 ingress2apisix converter.go 已经能够自动处理，转换后直接输出可用的 APISIX Ingress YAML（含原生注解）+ ApisixPluginConfig + BackendTrafficPolicy CRD。

#### A1 → APISIX 原生注解（k8s.apisix.apache.org/*）

| # | NGINX 注解 | 出现次数 | 典型值 | APISIX 等价注解 |
|---|-----------|----------|--------|-----------------|
| 1 | `use-regex` | 254 | `true` | `k8s.apisix.apache.org/use-regex` |
| 2 | `backend-protocol` | 233 | `http` | `k8s.apisix.apache.org/upstream-scheme` |
| 3 | `rewrite-target` | 41 | `/$2`, `/$1$2` 等 | `k8s.apisix.apache.org/rewrite-target` 或 `rewrite-target-regex` |
| 4 | `proxy-read-timeout` | 35 | `90` | `k8s.apisix.apache.org/upstream-read-timeout` (自动加 `s`) |
| 5 | `ssl-redirect` | 33 | `false` | `k8s.apisix.apache.org/http-to-https` |
| 6 | `proxy-send-timeout` | 22 | `90` | `k8s.apisix.apache.org/upstream-send-timeout` |
| 7 | `proxy-connect-timeout` | 14 | `90` | `k8s.apisix.apache.org/upstream-connect-timeout` |
| 8 | `enable-access-log` | 3 | `true` | `k8s.apisix.apache.org/enable-access-log` |
| 9 | `enable-cors` | 1 | `true` | `k8s.apisix.apache.org/enable-cors` |
| 10 | `cors-allow-origin` | 1 | `$http_origin` | `k8s.apisix.apache.org/cors-allow-origin` |
| 11 | `cors-allow-methods` | 1 | `GET,POST,...` | `k8s.apisix.apache.org/cors-allow-methods` |
| 12 | `cors-allow-headers` | 1 | `Authorization,...` | `k8s.apisix.apache.org/cors-allow-headers` |
| 13 | `whitelist-source-range` | 1 | `20.199...` | `k8s.apisix.apache.org/allowlist-source-range` |
| 14 | `auth-type` | 1 | `basic` | `k8s.apisix.apache.org/auth-type` (basic→basicAuth) |
| 15 | `auth-secret` | 1 | `ecms-basic-auth` | `k8s.apisix.apache.org/auth-secret` (需创建 ApisixConsumer) |
| 16 | `auth-realm` | 1 | `401: Authentication Required` | `k8s.apisix.apache.org/auth-realm` (需 forward-auth 插件 realm 字段，已有 patch) |
| 17 | `ingress.kubernetes.io/ssl-redirect` | 1 | `true` | `k8s.apisix.apache.org/http-to-https` |

#### A2 → ApisixPluginConfig（插件配置 CRD）

| # | NGINX 注解 | 出现次数 | 典型值 | APISIX 插件 | 插件配置说明 |
|---|-----------|----------|--------|------------|-------------|
| 1 | `limit-rps` | 229 | `10` | `limit-req` | rate=10, key=remote_addr |
| 2 | `limit-rpm` | 229 | `500` | `limit-req` | rate=500/min |
| 3 | `limit-multiplier` | 229 | `5` | `limit-req` | 乘算因子，10×5=50 req/s |
| 4 | `proxy-body-size` | 49 | `30m` | `client-control` | max_body_size 转换 (k/m/g) |
| 5 | `enable-real-ip` | 5 | `true` | `real-ip` | source=http_x_forwarded_for |
| 6 | `configuration-snippet` | 3 | 多条 rewrite | `proxy-rewrite` | 单条 rewrite 用原生注解，多条用 PluginConfig |
| 7 | `session-cookie-hash` | 2 | `sha1` | 自定义 `session-cookie-hash` | 支持 sha1/md5/sha256 |
| 8 | `upstream-vhost` | 2 | `minio1-service:9000` | `proxy-rewrite` | host 覆盖 |
| 9 | `ingress.kubernetes.io/proxy-body-size` | 2 | `0` | `client-control` | 同 proxy-body-size |
| 10 | `cors-allow-credentials` | 1 | `true` | `cors` | allow_credential=true |
| 11 | `cors-max-age` | 1 | `86400` | `cors` | max_age=86400 |
| 12 | `compute-full-forwarded-for` | 1 | `true` | `real-ip` | append 模式 |
| 13 | `forwarded-for-header` | 1 | `X-Forwarded-For` | `real-ip` | source 配置 |
| 14 | `use-forwarded-headers` | 1 | `true` | `real-ip` | recursive=true |
| 15 | `ssl-verify` | 1 | `false` | `proxy-ssl` / ApisixUpstream | 含警告提示 |
| 16 | `session-cookie-path` | 1 | `/system/tools` | 自定义 `session-cookie-hash` | cookie_path 字段 |
| 17 | `session-cookie-expires` | 1 | `172800` | 自定义 `session-cookie-hash` | max_age 字段 |
| 18 | `session-cookie-max-age` | 1 | `172800` | 自定义 `session-cookie-hash` | max_age 字段 |
| 19 | `session-cookie-conditional-samesite-none` | 1 | `true` | 自定义 `session-cookie-hash` | 生成警告（APISIX 无此参数） |

#### A3 → BackendTrafficPolicy CRD（流量策略 CRD）

| # | NGINX 注解 | 出现次数 | 典型值 | CRD 字段映射 |
|---|-----------|----------|--------|-------------|
| 1 | `upstream-hash-by` | 4 | `$remote_addr` | `loadBalancer.type=chash`, hashOn=header/query_arg |
| 2 | `health-check-interval` | 1 | `10s` | `healthCheck.active.interval` |
| 3 | `health-check-path` | 1 | `/v1/health` | `healthCheck.active.httpPath` |
| 4 | `health-check-retries` | 1 | `3` | `healthCheck.active.healthy.successes` |
| 5 | `health-check-timeout` | 1 | `5s` | `healthCheck.active.timeout` |

---

### B. 需开发AIC注解（APISIX 插件/CRD 已有，AIC 缺注解入口）

这些注解在 APISIX 中都有对应的插件或 CRD 能力，但 AIC（APISIX Ingress Controller）尚未提供原生注解映射。**不需要开发新插件，只需在 AIC 中新增注解定义和 handler。**

| # | NGINX 注解 | 出现次数 | 典型值 | APISIX 等价物 | AIC 缺口 | 开发工作量 |
|---|-----------|----------|--------|---------------|---------|-----------|
| 1 | `affinity` | 4 | `cookie` | BackendTrafficPolicy `loadBalancer.type=chash`, `hashOn=cookie` | 无 `affinity` 注解 | 低 — 字段直接映射 |
| 2 | `session-cookie-name` | 3 | `eureka` | BackendTrafficPolicy `loadBalancer.key` | 无 `session-cookie-name` 注解 | 低 — 字段直接映射 |
| 3 | `affinity-mode` | 2 | `persistent` | BackendTrafficPolicy `loadBalancer.type` 选择 | 无 `affinity-mode` 注解 | 低 |
| 4 | `upstream-keepalive-connections` | 4 | `600` | BackendTrafficPolicy `keepalive.connections` | 无 keepalive 注解 | 低 — 字段直接映射 |
| 5 | `upstream-keepalive-requests` | 7 | `4000` | BackendTrafficPolicy `keepalive.requests` | 无 keepalive 注解 | 低 — 字段直接映射 |
| 6 | `upstream-keepalive-timeout` | 6 | `25` | BackendTrafficPolicy `keepalive.timeout` | 无 keepalive 注解 | 低 — 字段直接映射 |
| 7 | `ssl-passthrough` | 5 | `true` | ApisixTls CRD + `stream_proxy` 全局配置 | 无 ssl-passthrough 注解 | 中 — 需同时配置 APISIX 全局 |
| 8 | `canary` | 1 | `true` | ApisixRoute + `vars` 条件路由 | 无 canary 注解 | 中 — 需拆分主/金丝雀路由 |
| 9 | `canary-by-header` | 1 | `X-Agent-Style` | ApisixRoute `vars` 匹配 | 无 canary 注解 | 中 — 配合 canary |
| 10 | `canary-by-header-value` | 1 | `normal` | ApisixRoute `vars` 匹配 | 无 canary 注解 | 中 — 配合 canary |
| 11 | `proxy-set-headers` | 1 | `Upgrade $http_upgrade...` | `proxy-rewrite` 插件 `headers.set` | 注解值引用 ConfigMap，AIC 需支持读取 ConfigMap | 中 — 需跨资源引用 |
| 12 | `service-upstream` | 5 | `true` | APISIX `upstream` 的 ClusterIP 模式 | 无 service-upstream 注解 | 中 — 需改变上游解析方式 |

#### B 类实现方案

**affinity + session-cookie-name + affinity-mode：**
```yaml
# 转换为 BackendTrafficPolicy CRD
apiVersion: apisix.apache.org/v1
kind: BackendTrafficPolicy
metadata:
  name: <ingress>-session-affinity
spec:
  loadBalancer:
    type: chash              # affinity=cookie → chash
    hashOn: cookie
    key: eureka              # session-cookie-name 的值
```

**upstream-keepalive-*：**
```yaml
# 转换为 BackendTrafficPolicy CRD
apiVersion: apisix.apache.org/v1
kind: BackendTrafficPolicy
metadata:
  name: <ingress>-keepalive
spec:
  keepalive:
    connections: 600
    requests: 4000
    timeout: 25
```

**canary：**
```yaml
# 拆分为 主路由 + canary 子路由 (ApisixRoute)
apiVersion: apisix.apache.org/v1
kind: ApisixRoute
spec:
  http:
  - name: canary-route
    priority: 100    # 高于主路由
    match:
      hosts: [example.com]
      paths: [/*]
      exprs:
      - subject: { name: X-Agent-Style, scope: Header }
        op: Equal
        value: normal
    backends:
    - serviceName: canary-service
      servicePort: 80
```

**ssl-passthrough：**
```yaml
# 1. 生成 ApisixTls CRD
apiVersion: apisix.apache.org/v1
kind: ApisixTls
spec:
  hosts: [my-domain.com]
  tcp:
  - host: my-domain.com
    service:
    - name: my-backend
      port: 443
# 2. 需在 APISIX config.yaml 启用 stream_proxy
# apisix:
#   stream_proxy:
#     tcp: ["443"]
```

---

### C. 需插件适配（APISIX 无等价插件或功能不完整）

| # | NGINX 注解 | 出现次数 | 典型值 | APISIX 现状 | 适配方案 | 工作量 |
|---|-----------|----------|--------|------------|---------|--------|
| 1 | `server-snippet` | 1 | 自定义 upstream 块 | APISIX 无 server-level snippet | 内容为权重 upstream，可转为 ApisixUpstream CRD；复杂场景（if 逻辑等）需开发 snippet 解析插件 | 低~中 |
| 2 | `proxy-request-buffering` | 2 | `off` | APISIX 无 per-route buffering 控制 | 需 APISIX 核心支持或 nginx_config snippet；可全局配置 | 低 |

#### server-snippet 实际内容分析
```nginx
upstream weighted_backend {
  server 20.240.52.123:9080 weight=100 max_fails=3 fail_timeout=30s;
  server 20.195.171.117:9080 weight=0 max_fails=3 fail_timeout=30s;
}
```
→ 可转为 ApisixUpstream CRD 的 `loadBalancer.type=roundrobin` + `nodes` 权重配置

---

### D. 需全局配置（APISIX config.yaml）

| # | NGINX 注解 | 出现次数 | 典型值 | APISIX 等价配置 | 可自动化 |
|---|-----------|----------|--------|-----------------|---------|
| 1 | `proxy-request-buffering` | 2 | `off` | `nginx_config.http_configuration_snippet: proxy_request_buffering off;` | ⚠️ 可生成配置提示 |
| 2 | `proxy-buffering` | 1 | `off` | `nginx_config.http_configuration_snippet: proxy_buffering off;` | ⚠️ 可生成配置提示 |

**APISIX config.yaml 等价配置：**
```yaml
nginx_config:
  http_configuration_snippet: |
    proxy_buffering off;
    proxy_request_buffering off;
```

---

### E. 拼写错误

| # | 错误注解 | 出现次数 | 正确注解 | 影响 |
|---|----------|----------|----------|------|
| 1 | `configuration-snipper` | 1 | `configuration-snippet` | ingress2apisix 无法识别，生成"未被识别"警告 |
| 2 | `proxy-http-verson` | 1 | `proxy-http-version` | ingress2apisix 无法识别，需先修正 |

**处理建议：** 在原始 Ingress YAML 中修正拼写后重新转换

---

### F. 未使用注解（标记）

以下是 NGINX Ingress Controller 官方支持的注解，但 **hxk8s1 集群中未使用**。无需改动，仅作记录。

| 分类 | 注解列表 |
|------|----------|
| 认证增强 | `auth-url`, `auth-signin`, `auth-method`, `auth-request-headers`, `auth-response-headers`, `auth-cache-key`, `auth-cache-duration`, `auth-keepalive`, `auth-snippet`, `auth-proxy-set-headers`, `auth-request-redirect`, `enable-global-auth`, `auth-always-set-cookie`, `auth-signin-redirect-param`, `auth-secret-type` |
| mTLS 客户端认证 | `auth-tls-secret`, `auth-tls-verify-client`, `auth-tls-verify-depth`, `auth-tls-error-page`, `auth-tls-pass-certificate-to-upstream`, `auth-tls-match-cn` |
| 后端证书 | `proxy-ssl-secret`, `proxy-ssl-verify`, `proxy-ssl-verify-depth`, `proxy-ssl-ciphers`, `proxy-ssl-name`, `proxy-ssl-protocols`, `proxy-ssl-server-name` |
| 速率限制增强 | `limit-burst-multiplier`, `limit-rate-after`, `limit-rate`, `limit-whitelist` |
| 金丝雀扩展 | `canary-by-cookie`, `canary-by-header-pattern`, `canary-weight`, `canary-weight-total`, `affinity-canary-behavior` |
| Session Cookie 扩展 | `session-cookie-change-on-failure`, `session-cookie-domain`, `session-cookie-samesite`, `session-cookie-secure` |
| 重定向扩展 | `permanent-redirect-code`, `temporal-redirect-code`, `from-to-www-redirect` |
| 代理扩展 | `proxy-next-upstream`, `proxy-next-upstream-timeout`, `proxy-next-upstream-tries`, `proxy-cookie-domain`, `proxy-busy-buffers-size`, `proxy-max-temp-file-size` |
| SSL/TLS | `ssl-protocols`, `ssl-ciphers`, `ssl-prefer-server-ciphers`, `ssl-passthrough` (已在 B 类) |
| 镜像 | `mirror-target`, `mirror-request-body`, `mirror-host` |
| WAF | `enable-modsecurity`, `enable-owasp-core-rules`, `modsecurity-transaction-id`, `modsecurity-snippet` |
| 可观测 | `enable-opentelemetry`, `opentelemetry-trust-incoming-span`, `enable-rewrite-log` |
| 高级功能 | `custom-http-errors`, `custom-headers`, `default-backend`, `server-alias`, `client-body-buffer-size`, `http2-push-preload`, `satisfy`, `stream-snippet`, `preserve-trailing-slash`, `connection-proxy-header`, `x-forwarded-prefix`, `load-balance`, `upstream-hash-by-subset`, `upstream-hash-by-subset-size` |

> **说明：** 以上注解在 hxk8s1 集群中未使用，如果未来集群需要这些功能：
> - 认证增强类（auth-url 等）→ ingress2apisix 已支持自动转换
> - 镜像类 → AIC 需开发 mirror 注解
> - WAF 类 → 需部署 APISIX waf 插件
> - 其他 → 参照各注解的 APISIX 等价物

---

## 4. 转换可行性总结

| 指标 | 数值 | 说明 |
|------|------|------|
| **A. 直接转换** | 38 / 58 | 65.5% — ingress2apisix 已完全支持 |
| **B. 需开发AIC注解** | 12 / 58 | 20.7% — APISIX 插件已就绪，AIC 缺注解入口 |
| **C. 需插件适配** | 2 / 58 | 3.4% — server-snippet 可部分覆盖 |
| **D. 需全局配置** | 2 / 58 | 3.4% — 修改 APISIX config.yaml |
| **E. 拼写错误** | 2 / 58 | 3.4% — 需先修正 |
| **未使用注解** | ~76 | — 仅标记，无需改动 |
| **综合自动化率** | **38/58** | **65.5% 直接转换** |
| **含AIC注解开发后** | **50/58** | **86.2%** |

---

## 5. 行动计划

### P0 — 预处理
- 修正 2 处拼写错误（`configuration-snipper` → `configuration-snippet`, `proxy-http-verson` → `proxy-http-version`）

### P1 — 直接转换（已完成）
- 38 种注解直接运行 ingress2apisix 即可批量转换
- 覆盖 345 个有注解 Ingress 中的绝大部分（top 5 注解占 229~254 个）

### P2 — AIC 注解开发（预计 3~5 天）
按优先级排序：
1. **affinity + session-cookie-name + affinity-mode** → BackendTrafficPolicy 注解（影响 4+3+2=9 个 Ingress）
2. **upstream-keepalive-*** → BackendTrafficPolicy keepalive 注解（影响 4+7+6=17 个 Ingress）
3. **ssl-passthrough** → ApisixTls 注解 + stream_proxy（影响 5 个 Ingress）
4. **canary-*** → ApisixRoute 条件路由注解（影响 1 个 Ingress）
5. **service-upstream** → 上游解析模式注解（影响 5 个 Ingress）
6. **proxy-set-headers** → ConfigMap 引用注解（影响 1 个 Ingress）

### P3 — 全局配置（预计 0.5 天）
- 在 APISIX config.yaml 中添加 `proxy_buffering off; proxy_request_buffering off;`

### P4 — server-snippet 处理（预计 0.5 天）
- hxk8s1 中仅 1 个 server-snippet，内容为权重 upstream，手动转为 ApisixUpstream CRD

---

## 6. 附录：ngnix-ingress-nginx 官方完整注解列表 vs APISIX 覆盖

> 参考：https://github.com/kubernetes/ingress-nginx/blob/main/docs/user-guide/nginx-configuration/annotations.md

NGINX Ingress Controller 官方共定义 **134 种注解**。hxk8s1 集群使用了其中 **58 种**（含 2 种拼写错误变体）。

| 覆盖状态 | 数量 | 说明 |
|----------|------|------|
| hxk8s1 使用 + ingress2apisix 已支持 | 38 | 可直接转换 |
| hxk8s1 使用 + APISIX 有插件但 AIC 缺注解 | 12 | 需 AIC 开发 |
| hxk8s1 使用 + 需插件/适配 | 2 | server-snippet + proxy-request-buffering |
| hxk8s1 使用 + 需全局配置 | 2 | proxy-buffering + proxy-request-buffering |
| hxk8s1 使用 + 拼写错误 | 2 | configuration-snipper + proxy-http-verson |
| hxk8s1 未使用 | ~76 | 无需改动 |
