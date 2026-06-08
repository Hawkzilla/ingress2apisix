# hxk8s1-lf2023cses1.json 注解迁移报告

> 421 个 Ingress 资源 | 58 个不同的 nginx/ingress 注解 key
> 基于 ingress2apisix 转换器实际代码 & APISIX/AIC 源码深度审查
> "JSON中" 列：`✓` = JSON 文件中出现过，`✗` = 仅转换器支持但该 JSON 未使用

---

## 一、自动转换 — APISIX 原生注解

直接映射到 `k8s.apisix.apache.org/*`，无需额外 CRD。

| 注解 | JSON中 | 次数 | 样例值 | → APISIX 注解 | 审查结论 |
|---|---|---|---|---|---|
| `use-regex` | ✓ | 254 | `true` / `false` | `k8s.apisix.apache.org/use-regex` | ✅ AIC 有 handler |
| `backend-protocol` | ✓ | 233 | `HTTPS` / `http` | `k8s.apisix.apache.org/upstream-scheme` | ✅ 支持 http/https/grpc/grpcs |
| `ssl-redirect` | ✓ | 33 | `false` / `true` | `k8s.apisix.apache.org/http-to-https` | ✅ 仅 true 生效 |
| `rewrite-target` | ✓ | 41 | `/$2` / `/$1` | `k8s.apisix.apache.org/rewrite-target-regex` + `rewrite-target-regex-template` | ✅ 同时输出 regex 和 template |
| `proxy-connect-timeout` | ✓ | 14 | `90` / `30` / `800` | `k8s.apisix.apache.org/upstream-connect-timeout` | ✅ 自动加 `s` 后缀 |
| `proxy-send-timeout` | ✓ | 22 | `90` / `3600` / `0.5` / `300` | `k8s.apisix.apache.org/upstream-send-timeout` | ✅ 自动加 `s` 后缀 |
| `proxy-read-timeout` | ✓ | 35 | `90` / `3600` / `30` / `86400` | `k8s.apisix.apache.org/upstream-read-timeout` | ✅ 自动加 `s` 后缀 |
| `enable-access-log` | ✓ | 3 | `true` | `k8s.apisix.apache.org/enable-access-log` | ⚠ AIC 未定义此注解 handler，可能被忽略 |
| `auth-url` | ✗ | 0 | — | `k8s.apisix.apache.org/auth-uri` | ✅ |
| `auth-method` | ✗ | 0 | — | `k8s.apisix.apache.org/auth-method` | ✅ 自动大写 |
| `auth-request-headers` | ✗ | 0 | — | `k8s.apisix.apache.org/auth-request-headers` | ✅ |
| `auth-response-headers` | ✗ | 0 | — | `k8s.apisix.apache.org/auth-upstream-headers` | ✅ |
| `auth-signin` | ✗ | 0 | — | `k8s.apisix.apache.org/auth-signin` | ✅ |
| `auth-type` | ✓ | 1 | `basic` | `k8s.apisix.apache.org/auth-type=basicAuth` | ✅ |
| `auth-secret` | ✓ | 1 | `ecms-basic-auth` | ⚠ 无法自动转换 | ❌ AIC 无 handler，需手动创建 ApisixConsumer |
| `auth-realm` | ✓ | 1 | `401: Authentication Required` | `k8s.apisix.apache.org/auth-realm` | ⚠ 注解中此 realm 走 forward-auth，非 basic-auth |
| `custom-http-errors` | ✗ | 0 | — | `k8s.apisix.apache.org/custom-error-codes` | ✅ |
| `websocket-services` | ✗ | 0 | — | `k8s.apisix.apache.org/enable-websocket` | ✅ |
| `permanent-redirect` | ✗ | 0 | — | `k8s.apisix.apache.org/http-redirect` + code=308 | ✅ |
| `temporal-redirect` | ✗ | 0 | — | `k8s.apisix.apache.org/http-redirect` + code=302 | ✅ |
| `app-root` | ✗ | 0 | — | `k8s.apisix.apache.org/http-redirect` | ⚠ 语义差异：nginx 只重定向 `/`，APISIX 重定向所有请求 |
| `denylist-source-range` | ✗ | 0 | — | `k8s.apisix.apache.org/blocklist-source-range` | ✅ |

---

## 二、自动转换 — ApisixPluginConfig 插件

生成 `ApisixPluginConfig` CRD，通过 `plugin-config-name` 注解关联。

| 注解 | JSON中 | 次数 | 样例值 | → APISIX 插件 | 审查结论 |
|---|---|---|---|---|---|
| `limit-rps` | ✓ | 229 | `10` / `2` | `limit-req` {rate, burst, key} | ✅ rate 为数字 |
| `limit-rpm` | ✓ | 229 | `500` / `2` | `limit-req` {rate} (÷60 转 rps) | ✅ 已修正：rpm÷60=rps |
| `limit-multiplier` | ✓ | 229 | `5` | 乘数应用于 limit-rps/rpm | ✅ |
| `proxy-body-size` | ✓ | 49 | `30m` / `500m` / `40m` / `10M` | `client-control` {max_body_size} | ✅ 转为字节数 |
| `proxy-request-buffering` | ✓ | 2 | `off` | `proxy-control` {request_buffering: false} | ✅ |
| `enable-cors` | ✓ | 1 | `true` | `cors` plugin | ✅ |
| `cors-allow-origin` | ✓ | 1 | `$http_origin` | `cors.allow_origins` | ⚠ `$http_origin` 是 nginx 变量，APISIX 用 `*` 替代 |
| `cors-allow-methods` | ✓ | 1 | `GET,POST,PUT,DELETE,OPTIONS,PATCH,HEAD` | `cors.allow_methods` | ✅ |
| `cors-allow-headers` | ✓ | 1 | `Authorization,Content-Type,...` | `cors.allow_headers` | ✅ |
| `cors-allow-credentials` | ✓ | 1 | `true` | `cors.allow_credential` | ⚠ `*` + credentials 有冲突，需确认 |
| `cors-max-age` | ✓ | 1 | `86400` | `cors.max_age` | ✅ |
| `enable-real-ip` | ✓ | 5 | `true` | `real-ip` {source, trusted_addresses} | ✅ 已修正字段名 |
| `use-forwarded-headers` | ✓ | 1 | `true` | `real-ip` {recursive: true} | ✅ |
| `compute-full-forwarded-for` | ✓ | 1 | `true` | ⚠ 产生警告 | ❌ APISIX real-ip 无等价 `append` 参数 |
| `forwarded-for-header` | ✓ | 1 | `X-Forwarded-For` | `real-ip` {source: http_x_forwarded_for} | ✅ |
| `session-cookie-hash` | ✓ | 2 | `sha1` | `session-cookie-hash` (自定义) | ✅ |
| `session-cookie-name` | ✓ | 3 | `eureka` / `route` | cookie_name 参数 | ✅ |
| `session-cookie-expires` | ✓ | 1 | `172800` | max_age 参数 | ✅ |
| `session-cookie-max-age` | ✓ | 1 | `172800` | max_age 参数 | ✅ |
| `session-cookie-path` | ✓ | 1 | `/system/tools` | cookie_path 参数 | ✅ |
| `session-cookie-conditional-samesite-none` | ✓ | 1 | `true` | ⚠ 产生警告，APISIX 无此参数 | ⚠ 无法自动处理 |
| `ssl-verify` | ✓ | 1 | `false` | ⚠ 产生警告 | ⚠ 需 ApisixUpstream TLS 配置 |
| `upstream-vhost` | ✓ | 2 | `minio1-service:9000` | `proxy-rewrite` {host} | ✅ |
| `configuration-snippet` | ✓ | 3 | 含 rewrite / add_header | `proxy-rewrite` {regex_uri} | ✅ |

---

## 三、自动转换 — BackendTrafficPolicy CRD

生成 `BackendTrafficPolicy` 资源，关联后端 Service。

| 注解 | JSON中 | 次数 | 样例值 | → 转换方式 | 审查结论 |
|---|---|---|---|---|---|
| `affinity` | ✓ | 4 | `cookie` | LoadBalancer{type: chash, hashOn: cookie} | ✅ |
| `affinity-mode` | ✓ | 2 | `persistent` | 配合 cookie affinity | ✅ |
| `upstream-hash-by` | ✓ | 4 | `$remote_addr` / `$arg_hashid` | LoadBalancer{type: chash, hashOn: vars} | ✅ 已修正：hashOn=vars |
| `health-check-*` | ✓ | 4 | `/v1/health` / `10s` | ⬆ 已移至 ApisixUpstream CRD | ✅ BTP 无 healthCheck 字段 |

---

## 四、自动转换 — ApisixUpstream CRD

生成 `ApisixUpstream` CRD，用于 upstream 级别配置（健康检查）。

| 注解 | JSON中 | 次数 | 样例值 | → 转换方式 | 审查结论 |
|---|---|---|---|---|---|
| `health-check-path` | ✓ | 1 | `/v1/health` | `spec.healthCheck.active.httpPath` | ✅ |
| `health-check-interval` | ✓ | 1 | `10s` | `spec.healthCheck.active.healthy.interval` | ✅ 自动加 `s` 后缀 |
| `health-check-retries` | ✓ | 1 | `3` | `spec.healthCheck.active.healthy.successes` | ✅ |
| `health-check-timeout` | ✓ | 1 | `5s` | `spec.healthCheck.active.timeout` | ✅ 自动加 `s` 后缀 |

> ⚠ `upstream-keepalive-*` 注解无法生成 ApisixUpstream，因为 AIC 的 ApisixUpstream CRD schema **没有 keepalive 字段**。已产生手动迁移警告。

---

## 五、自动转换 — ApisixTls CRD

从 Ingress `spec.tls` 段自动生成 `ApisixTls` CRD，用于 TLS 终止配置（证书引用）。

> 非注解，而是 Ingress Spec 字段。每个 `spec.tls` 条目（含 secretName + hosts）生成一个 ApisixTls。

| 来源字段 | JSON中 | 样例值 | → ApisixTls 字段 | 审查结论 |
|---|---|---|---|---|
| `spec.tls[].secretName` | ✓ | TLS secret 名称 | `spec.secret.name` + `spec.secret.namespace` | ✅ 结构完全匹配 |
| `spec.tls[].hosts` | ✓ | `["secure.example.com"]` | `spec.hosts` | ✅ |

> ⚠ `ssl-passthrough`（L4 TLS 透传）**不**生成 ApisixTls。APISIX 尚不支持 TLSRoute passthrough，该注解仍需手动处理（见第六节）。

---

## 六、需手动迁移

这些注解在 APISIX/AIC 中无自动转换路径，需人工评估和处理。

| 注解 | JSON中 | 次数 | 样例值 | 迁移建议 |
|---|---|---|---|---|
| `upstream-keepalive-connections` | ✓ | 4 | `600` | **AIC ApisixUpstream CRD 无 keepalive 字段**；需在 APISIX 全局配置中设置 |
| `upstream-keepalive-requests` | ✓ | 7 | `4000` / `1000` / `800` | 同上 |
| `upstream-keepalive-timeout` | ✓ | 6 | `25` / `60` | 同上 |
| `auth-secret` | ✓ | 1 | `ecms-basic-auth` | AIC 无此注解 handler；需手动创建 ApisixConsumer CRD 配置 basic-auth 凭证 |
| `service-upstream` | ✓ | 5 | `true` | APISIX 全局 DNS 解析模式配置 |
| `ssl-passthrough` | ✓ | 5 | `true` | APISIX `stream_proxy` 配置 + `ApisixTls` CRD；需 L4 层面配置，无插件自动转换 |
| `ssl-verify` | ✓ | 1 | `false` | 需 `ApisixUpstream` CRD 配置 TLS 验证 |
| `proxy-buffering` | ✓ | 1 | `off` | APISIX `proxy-control` 插件仅支持 `request_buffering`，**无响应缓冲控制**，需全局 nginx snippet |
| `proxy-set-headers` | ✓ | 1 | `Upgrade $http_upgrade` | `proxy-rewrite` 插件 `headers.set`，但此注解值引用了 nginx 变量 `$http_upgrade`，需手动替换为固定值 |
| `proxy-http-version` | ✓ | 1 | `1.1`（注: 原文拼写错误为 `verson`） | 全局配置上游 HTTP 版本 |
| `server-snippet` | ✓ | 1 | 含 `upstream weighted_backend { ... }` | 需逐条分析：此为 weighted upstream，需用 APISIX `traffic-split` 插件实现 |
| `canary` | ✓ | 1 | `true` | APISIX `ApisixRoute` + `traffic-split` 插件，或 Gateway API `HTTPRoute` weighted backendRefs |
| `canary-by-header` | ✓ | 1 | `X-Agent-Style` | `traffic-split` 插件 `match[].vars` 条件路由 |
| `canary-by-header-value` | ✓ | 1 | `normal` | 配合 canary-by-header 的 match 条件 |
| `whitelist-source-range` | ✓ | 1 | `20.199.194.183,20.202.165.102` | APISIX `ip-restriction` 插件 `whitelist` |

---

## 七、拼写错误 / 无效注解

JSON 中存在以下拼写错误的注解，nginx ingress 本身会忽略它们：

| 注解 | JSON中 | 次数 | 样例值 | 问题 |
|---|---|---|---|---|
| `proxy-http-verson` | ✓ | 1 | `1.1` | 应为 `proxy-http-version`（少了个 `i`） |
| `configuration-snipper` | ✓ | 1 | `proxy_set_header X-Forwarded-Prefix /flink;` | 应为 `configuration-snippet`（多了个 `p`） |

---

## 八、Gateway API 模式

选择 Gateway API 作为目标时，第二、三类注解同样生效，额外生成 `ApisixPluginConfig` 和 `BackendTrafficPolicy` CRD 并通过 `HTTPRoute.ExtensionRef` 引用。

Gateway API 模式下额外自动处理的注解（生成 APISIX 插件）：

| 注解 | JSON中 | → APISIX 插件 | 说明 |
|---|---|---|---|
| `auth-url` / `auth-method` / `auth-request-headers` / `auth-response-headers` / `auth-signin` | ✗ | `forward-auth` | Gateway API 无原生外部认证，通过 ExtensionRef 注入 |
| `proxy-connect/send/read-timeout` | ✓ | `proxy-rewrite` (timeout) | 超时值秒→毫秒转换 |
| `whitelist-source-range` / `denylist-source-range` | ✗ | `ip-restriction` | IP 黑白名单 |
| `custom-http-errors` | ✗ | `custom-error-page` | 自定义错误页 |
| `proxy-request-buffering` | ✓ | `proxy-control` | 请求缓冲控制 |
| `auth-realm` | ✓ | `forward-auth` (realm 相关头) | 认证域 |

---

## 九、汇总

| 转换路径 | 资源类型 | 注解/字段数 | JSON中出现 | 覆盖 Ingress 次数 |
|---|---|---|---|---|
| APISIX 原生注解 | Ingress annotations | 22 | 11 ✓ / 11 ✗ | ~580 |
| PluginConfig 插件 | ApisixPluginConfig | 24 | 24 ✓ / 0 ✗ | ~540 |
| BackendTrafficPolicy | BackendTrafficPolicy | 3 | 3 ✓ / 0 ✗ | ~10 |
| ApisixUpstream CRD | ApisixUpstream | 4 | 4 ✓ / 0 ✗ | ~4 |
| ApisixTls CRD | ApisixTls | spec.tls | ✓ | ~28 |
| 需手动迁移 | — | 15 | 15 ✓ / 0 ✗ | ~55 |
| 拼写错误/无效 | — | 2 | 2 ✓ / 0 ✗ | 2 |

> JSON 文件中实际出现的注解 key：**58 个**（含 2 个拼写错误），全部已被转换器自动处理或归类为手动迁移。
> 转换器另外支持 **11 个** JSON 未使用但常见于其他集群的注解（auth-url、cors-*、custom-http-errors 等）。
> Ingress `spec.tls` 段自动转换为 `ApisixTls` CRD（JSON 中有 ~28 个 Ingress 含 TLS 配置）。

---

## 十、审查发现的已修正问题

| 严重程度 | 问题 | 修正方案 |
|---|---|---|
| **CRITICAL** | 健康检查字段放在 BackendTrafficPolicy，但 AIC BTP CRD 无此字段 | 移至 ApisixUpstream CRD 的 `healthCheck` 字段 |
| **CRITICAL** | limit-req rpm 后缀 `rate: "500/min"` 不合法 | 改为除以 60 转换为 rps 数值 |
| **CRITICAL** | real-ip 使用不存在的 `real_ip_from` 字段 | 改为 `trusted_addresses` |
| **CRITICAL** | `compute-full-forwarded-for` 使用不存在的 `append` 字段 | 移除，改为产生警告 |
| **CRITICAL** | keepalive 字段在 AIC ApisixUpstream CRD 中不存在 | 不再生成，改为产生手动迁移警告 |
| **CRITICAL** | `query_arg` 不是 AIC 合法的 hashOn 值 | 改为 `vars`（APISIX 变量格式） |
| **HIGH** | `rewrite-target` regex 缺少 `rewrite-target-regex-template` | 同时输出两个注解 |
| **HIGH** | `auth-secret` 不被 AIC 支持 | 不再输出，改为产生手动迁移警告 |
| **HIGH** | `auth-type: digest` 误映射为 `keyAuth` | 移除该映射，digest 认证无等价方案 |
| **HIGH** | 健康检查 `httpStatuses` YAML tag 错误 | 改为 `httpCodes` 匹配 AIC |
| **HIGH** | 健康检查 `timeouts` tag（复数）错误 | 改为 `timeout`（单数）匹配 AIC |
| **HIGH** | 健康检查 `Interval` 放在 ActiveHealthCheck 顶层 | 移至 Healthy/Unhealthy 子结构 |
| **HIGH** | 健康检查 timeout/interval 类型为 int | 改为 duration 字符串（`"10s"`） |
