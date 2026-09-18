# APISIX Ingress Controller 核心能力清单与验证报告

> 验证版本：APISIX Ingress Controller 2.0.1-es + APISIX 3.16.0-es.1（standalone API-driven 模式）。其他版本需重新核验。
> 文档性质：**能力清单 + 实测验证记录**。本文能力项均在上述版本的真实集群执行验证，环境参数一律使用占位符（获取方法见《APISIX 网关用户使用手册》第 1 章）。其中 `<KEEPALIVED_VIP>` 指该实例 **Keepalived 漂移出的网关 VIP**（不是 APISIX 节点 IP）。
> 配置主体：以标准 Ingress 为主，APISIX CRD（ApisixPluginConfig / ApisixConsumer / BackendTrafficPolicy）作为补充。

## 1. 验证结论总览

| 能力域 | 能力项 | 配置方式 | 结论 |
|---|---|---|---|
| 路由 | Host / 前缀 / 精确 / 正则匹配 | Ingress（use-regex 注解） | ✅ 通过 |
| 路由 | 路径重写（前缀替换、正则捕获） | Ingress 注解 | ✅ 通过 |
| 路由 | 重定向（http-redirect） | Ingress 注解 | ✅ 通过 |
| 路由 | 跨命名空间后端 | Ingress 注解 svc-namespace | ✅ 通过 |
| 路由 | 方法 / 请求头 / 查询参数匹配（间接） | Ingress + ApisixPluginConfig（traffic-split match.vars） | ✅ 通过 |
| 路由 | 权重灰度 90/10（间接） | Ingress + ApisixPluginConfig（traffic-split weighted_upstreams） | ✅ 通过 |
| 上游 | 多节点上游 / http 协议 | 标准 Ingress | ✅ 通过 |
| 上游 | https 上游 | 注解 upstream-scheme | ✅ 通过 |
| 上游 | 连接/读/写超时 | 注解 upstream-*-timeout | ✅ 通过（连接与读均实测 504） |
| 上游 | 重试与故障转移 | 注解 upstream-retries | ✅ 通过（连接级失败自动转移） |
| 上游 | 路由级熔断 | Ingress + ApisixPluginConfig（api-breaker） | ✅ 通过 |
| 上游 | Host 头传递与改写 | BackendTrafficPolicy passHost | ✅ 通过 |
| 负载均衡 | roundrobin / chash cookie / chash header / least_conn / ewma | BackendTrafficPolicy | ✅ 配置生效（算法效果见第 4 节） |
| 认证鉴权 | keyAuth / basicAuth | ApisixConsumer + Ingress 注解 | ✅ 通过 |
| 认证鉴权 | jwtAuth | ApisixConsumer + ApisixPluginConfig | ✅ 通过 |
| 认证鉴权 | forward-auth（外部认证） | Ingress 注解 auth-uri | ✅ 通过 |
| 认证鉴权 | IP 黑白名单 | Ingress 注解 | ✅ 通过 |
| 认证鉴权 | HTTP 方法限制 / CSRF / CORS | Ingress 注解 | ✅ 通过 |
| 限流限速 | limit-count（按 IP / 按请求头） | ApisixPluginConfig | ✅ 通过 |
| 限流限速 | limit-req（QPS）、limit-conn（并发） | ApisixPluginConfig | ✅ 通过 |
| 可观测性 | 访问日志 / 错误日志 | APISIX 容器日志 | ✅ 通过 |
| 可观测性 | Admin API 配置查询（X-Digest） | Admin API | ✅ 通过 |
| 可观测性 | 控制器指标 / APISIX Prometheus 指标 | :8080/metrics、节点内 :9091 | ✅ 通过 |
| 可观测性 | 请求 ID | ApisixPluginConfig（request-id） | ✅ 通过 |
| TLS | HTTPS 卸载（含 SNI 证书选择） | Ingress spec.tls + Secret | ✅ 通过 |
| TLS | 双向认证（mTLS） | ApisixTls client.caSecret | ✅ 通过 |

> 说明：本文只覆盖 **Ingress 注解 / Ingress 可引用的补充 CRD** 所提供的能力；由其他资源类型独立定义的能力不在本文范围（见第 9.2 节第 1 条）。

---

## 2. 路由能力

> 通用验证方式（后文不再重复）：
> ```bash
> curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: <域名>' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
> ```

### 2.1 Host 匹配

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: app-host, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
  - host: app.example.com
    http: {paths: [{path: /, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}]}
```

实测：命中域名返回 200（上游收到请求）；未知域名返回 404 `{"error_msg":"404 Route Not Found"}`。

### 2.2 路径前缀 / 精确匹配

```yaml
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
  - host: path.example.com
    http:
      paths:
      - {path: /api,   pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}
  - host: exact.example.com
    http:
      paths:
      - {path: /exact, pathType: Exact,  backend: {service: {name: my-app, port: {number: 80}}}}
```

实测（数据面生成的路由：Prefix → `uris: ["/api","/api/*"]`；Exact → `uris: ["/exact"]`）：

| 请求 | 结果 |
|---|---|
| Prefix `/api` | 200（后端收到 `/api`） |
| Prefix `/api/user` | 200（后端收到 `/api/user`） |
| Prefix `/other` | 404（无匹配路由） |
| Exact `/exact` | 200（后端收到 `/exact`） |
| Exact `/exact/sub` | 404（无匹配路由） |

### 2.3 正则匹配

```yaml
metadata:
  annotations: {k8s.apisix.apache.org/use-regex: "true"}
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
  - host: regex.example.com
    http:
      paths:
      - {path: "/users/[0-9]+", pathType: ImplementationSpecific, backend: {service: {name: my-app, port: {number: 80}}}}
```

实测：

| 请求 | 结果 |
|---|---|
| `/users/123` | 200（后端收到 `/users/123`，说明正则命中并转发） |
| `/users/abc` | 404（无匹配路由） |

### 2.4 方法 / 请求头 / 查询参数路由（间接实现）

标准 Ingress 只提供 host/path 匹配。按 **方法、请求头、查询参数** 做路由，可用 `traffic-split` 插件的 `match.vars` 通过 `ApisixPluginConfig` 挂到 Ingress 上实现，无需其他路由类资源。

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: match-rules, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - name: traffic-split
    enable: true
    config:
      rules:
      - match:                      # 请求头：X-Canary: 1
        - vars: [["http_x_canary", "==", "1"]]
        weighted_upstreams:
        - upstream: {type: roundrobin, nodes: {"<svc-v2>.<业务命名空间>.svc.cluster.local:80": 1}}
          weight: 100
      - match:                      # 方法：POST
        - vars: [["request_method", "==", "POST"]]
        weighted_upstreams:
        - upstream: {type: roundrobin, nodes: {"<svc-v2>.<业务命名空间>.svc.cluster.local:80": 1}}
          weight: 100
      - match:                      # 查询参数：version=v2
        - vars: [["arg_version", "==", "v2"]]
        weighted_upstreams:
        - upstream: {type: roundrobin, nodes: {"<svc-v2>.<业务命名空间>.svc.cluster.local:80": 1}}
          weight: 100
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-match
  namespace: <业务命名空间>
  annotations: {k8s.apisix.apache.org/plugin-config-name: match-rules}
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
  - host: app.example.com
    http: {paths: [{path: /, pathType: Prefix, backend: {service: {name: <svc-v1>, port: {number: 80}}}}]}
```

实测：

| 请求 | 实际命中 |
|---|---|
| 默认（无匹配） | <svc-v1>（Ingress 自身后端） |
| `X-Canary: 1` | <svc-v2> |
| POST | <svc-v2> |
| `?version=v2` | <svc-v2> |
| 上游使用 Service FQDN（`<svc>.<ns>.svc.cluster.local`） | 解析正常，无需写 Pod IP/ClusterIP |

注意事项：

1. 规则不匹配时走 Ingress 自身后端（不是 404）；如需"不匹配即拒绝"，可再叠加 `serverless-pre-function` 插件用 Lua 返回 4xx；
2. 变量命名：请求头 `http_<name>`、查询参数 `arg_<name>`、方法 `request_method`；
3. 规则按顺序匹配、命中即停；权重与灰度共用同一插件。

### 2.5 权重灰度（间接实现）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: canary-split, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - name: traffic-split
    enable: true
    config:
      rules:
      - weighted_upstreams:            # 无 match = 对所有请求生效
        - upstream: {type: roundrobin, nodes: {"<svc-v1>.<业务命名空间>.svc.cluster.local:80": 1}}
          weight: 90
        - upstream: {type: roundrobin, nodes: {"<svc-v2>.<业务命名空间>.svc.cluster.local:80": 1}}
          weight: 10
```

实测：数据面配置为 90/10；200 次请求中 v1/v2 为 **194/6**，两个上游均参与分流。
注意：90/10 是概率权重，不保证小样本得到精确比例；调整比例只需修改权重后重新 apply。

### 2.6 路径重写

```yaml
# 前缀整体替换
metadata:
  annotations: {k8s.apisix.apache.org/rewrite-target: /new-path}
spec:
  rules: [{host: rewrite.example.com, http: {paths: [{path: /api, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}]}}]
---
# 正则捕获重写
metadata:
  annotations:
    k8s.apisix.apache.org/rewrite-target-regex: "^/api/(.*)"
    k8s.apisix.apache.org/rewrite-target-regex-template: "/backend/$1"
```

实测（在后端回显的请求 URI 中确认）：`/api/test` → 上游收到 `/new-path`；`/api/test` → 上游收到 `/backend/test`。

### 2.7 重定向

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/http-redirect: "/new$uri"
    k8s.apisix.apache.org/http-redirect-code: "308"
```

实测：`GET /x/y` 返回 308，`Location: /new/x/y`。

### 2.8 跨命名空间后端

```yaml
metadata:
  annotations: {k8s.apisix.apache.org/svc-namespace: <后端命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
  - host: cross.example.com
    http: {paths: [{path: /, pathType: Prefix, backend: {service: {name: backend-svc, port: {number: 80}}}}]}
```

实测：返回 200，响应来自被引用命名空间的服务。启用 validating webhook 时，apply 可能出现 `Referenced Service '<ingress命名空间>/<服务名>' not found` 的 admission warning；这是 webhook 未识别 `svc-namespace` 产生的误报，不影响控制器按目标命名空间转发。

---

## 3. 上游能力

### 3.1 多节点上游

标准 Ingress 引用 Service，控制器按 EndpointSlice 生成全部后端节点，实测服务下 2 个 Pod 均参与转发。

### 3.2 上游协议

```yaml
# https 上游
metadata:
  annotations: {k8s.apisix.apache.org/upstream-scheme: https}
spec:
  rules: [{host: up-https.example.com, http: {paths: [{path: /, pathType: Prefix, backend: {service: {name: tls-backend, port: {number: 443}}}}]}}]
```

实测：访问 https 后端返回 200（后端响应）

### 3.3 超时控制

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/upstream-connect-timeout: "2s"
    k8s.apisix.apache.org/upstream-read-timeout: "2s"
    k8s.apisix.apache.org/upstream-send-timeout: "2s"
```

数据面配置（Admin API 可见）：

```json
{"timeout":{"connect":2,"send":2,"read":2}}
```

实测：

| 场景 | 构造方式 | 结果 |
|---|---|---|
| 读超时 | 后端接受连接但不返回数据 | **504，约 2s** |
| 连接超时 | ExternalName Service 指向不可达地址 | **504，约 2s** |
| 超时配置 | 通过注解设置 connect/send/read | 三项均正确下发 |

> 说明：控制器读取 EndpointSlice。无 selector 的 Service 只要存在标签、端口匹配的 EndpointSlice，也能生成上游节点；仅创建 legacy Endpoints 不能保证生效。构造不可达节点可使用 ExternalName Service 指向不可达 IP。

### 3.4 重试与故障转移

```yaml
metadata:
  annotations: {k8s.apisix.apache.org/upstream-retries: "2"}
```

实测结论（重要）：

| 失败类型 | 表现 | 结论 |
|---|---|---|
| 连接级失败（Connection refused） | 上游含 1 个好节点 + 1 个不监听端口的节点，20 次请求 **20/20 成功**；错误日志出现 `connect() failed (111: Connection refused)` | ✅ 自动转移到健康节点 |
| 后端返回 HTTP 502 | 上游含 1 个返回 502 的节点，`upstream-retries: 2` 后 20 次请求仍有 10 次返回 502 | ⚠️ 当前场景未触发到其他节点的透明重试 |

`upstream-retries` 设置额外尝试次数，触发条件遵循底层 NGINX upstream 失败语义。当前实测证明连接拒绝会切换节点，但不能据此推断所有 HTTP 5xx、超时阶段和协议。需要 5xx 兜底时应在应用侧或专用容错方案中实现。

### 3.5 上游 Host 头

```yaml
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata: {name: host-policy, namespace: <业务命名空间>}
spec:
  targetRefs: [{group: "", kind: Service, name: my-app}]
  passHost: rewrite            # pass（默认）/ node / rewrite
  upstreamHost: target.internal
```

实测（后端回显 Host）：

| 配置 | 后端收到的 Host |
|---|---|
| 默认（pass） | `app.example.com`（原始域名） |
| rewrite + upstreamHost | `target.internal` |

!注意 `upstreamHost` 只接受主机名（不能带端口）；需要带端口时改用 ApisixPluginConfig 的 `proxy-rewrite.host`。

!注意 BackendTrafficPolicy 作用于整个 Service，同一 Service 被多个 BackendTrafficPolicy 引用时生效顺序不确定，应保证一个 Service 只被一个策略引用。

### 3.6 路由级熔断

上游节点级健康检查未通过 Ingress 注解 / BackendTrafficPolicy 暴露；在 Ingress 路径内可用 `api-breaker` 插件实现"连续失败后快速失败"的熔断效果：

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: breaker, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - name: api-breaker
    enable: true
    config:
      break_response_code: 503
      break_response_body: "circuit-open"
      max_breaker_sec: 30
      unhealthy: {http_statuses: [500, 502, 503], failures: 1}
      healthy:   {http_statuses: [200], successes: 1}
---
# Ingress 通过注解引用：k8s.apisix.apache.org/plugin-config-name: breaker
```

实测（后端持续 502，`max_breaker_sec` 取 3s）：

| 请求 | 结果 |
|---|---|
| 第 1 次 | 502（真实上游失败） |
| 熔断窗口内的后续请求 | 503 + `circuit-open`（快速失败） |
| 等待 4s 后 | 502（熔断窗口结束，重新尝试上游） |

`api-breaker` 按 Host + URI 对后续请求快速失败，不会把当前 5xx 请求重试到另一个上游，也不是节点级健康检查。连接级失败转移与路由级熔断可组合使用，但二者职责不同。

---

## 4. 负载均衡

统一通过 `BackendTrafficPolicy` 绑定 Service 生效：

> `BackendTrafficPolicy` 在当前版本中为 `v1alpha1`，控制器升级前应核对 CRD 兼容性。

```yaml
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata: {name: lb-policy, namespace: <业务命名空间>}
spec:
  targetRefs: [{group: "", kind: Service, name: my-app}]
  loadbalancer:
    type: roundrobin          # roundrobin / chash / ewma / least_conn
    hashOn: cookie            # chash 时生效：vars / header / cookie / consumer / vars_combinations
    key: route                # chash 时的哈希键
```

实测（一个快速后端和一个延迟后端）：

| 算法 | 实测结果 | 结论 |
|---|---|---|
| roundrobin（默认） | fast=11 / slow=9 | ✅ 两个节点均参与轮询 |
| chash + cookie | 两个 Cookie 值分别连续 8 次命中固定后端 | ✅ 会话保持 |
| chash + header | 两个请求头值分别连续 8 次命中固定后端 | ✅ 会话保持 |
| least_conn | 数据面 `type=least_conn`；短连接样本未形成稳定倾斜 | ✅ 配置生效；不要用小样本判断调度质量 |
| ewma | 预热后连续 30 次命中快节点 | ✅ 向低延迟节点倾斜 |

> 注意：`chash` 只负责按哈希键选后端，**Cookie/请求头需要由客户端或应用携带**（需要网关下发 Cookie 时配合 session-cookie-hash 类插件）。

---
## 5. 认证鉴权

### 5.1 Key 认证（keyAuth）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata: {name: app-key, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  authParameter: {keyAuth: {value: {key: <自定义 key>}}}
---
# Ingress 侧
metadata:
  annotations: {k8s.apisix.apache.org/auth-type: keyAuth}
```

实测：

| 请求 | 结果 |
|---|---|
| 不带 apikey | 401 |
| `apikey: <正确 key>` | 200 |
| `apikey: <错误 key>` | 401 |

### 5.2 Basic 认证（basicAuth）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata: {name: app-basic, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  authParameter: {basicAuth: {value: {username: <用户>, password: <口令>}}}
---
metadata:
  annotations: {k8s.apisix.apache.org/auth-type: basicAuth}
```

实测：不带凭据 401；`-u <用户>:<口令>` 200；错误口令 401。

### 5.3 JWT 认证（jwtAuth）

> ⚠️ 注意：Ingress 的 `auth-type` 注解**只支持 keyAuth / basicAuth**；JWT 需要通过 `ApisixPluginConfig` 挂载 `jwt-auth` 插件。

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata: {name: app-jwt, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  authParameter:
    jwtAuth:
      value:
        key: <consumer key，需与 JWT 的 key 声明一致>
        secret: <HS256 密钥>
        algorithm: HS256
        exp: 3600
        private_key: unused        # 当前版本 CRD 校验所需占位值，不是 HS256 算法要求
---
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: jwt-auth-plugin, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - {name: jwt-auth, enable: true, config: {}}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-jwt
  annotations: {k8s.apisix.apache.org/plugin-config-name: jwt-auth-plugin}
```

实测：

| 请求 | 结果 |
|---|---|
| 不带 Authorization | 401 |
| `Authorization: Bearer <有效 JWT>` | 200 |
| `Authorization: Bearer a.b.c` | 401 |

### 5.4 外部认证（forward-auth）

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/auth-uri: "http://<认证服务地址>/verify"
    k8s.apisix.apache.org/auth-request-headers: "Authorization"
    k8s.apisix.apache.org/auth-upstream-headers: "X-User"
```

实测：

| 认证服务返回 | 网关行为 |
|---|---|
| 200 + 响应头 X-User: alice | 请求放行，**后端收到 X-User: alice** |
| 403 | 网关直接返回 403（返回认证服务响应体） |

### 5.5 IP 黑白名单

```yaml
# 白名单：仅允许指定网段
metadata:
  annotations: {k8s.apisix.apache.org/allowlist-source-range: "10.0.0.0/8,192.168.0.0/16"}
---
# 黑名单：拒绝指定网段
metadata:
  annotations: {k8s.apisix.apache.org/blocklist-source-range: "10.0.0.0/8"}
```

实测：白名单内客户端 200；不在白名单 403（`{"message":"Your IP address is not allowed"}`）；黑名单内客户端 403。

### 5.6 HTTP 方法限制

```yaml
metadata:
  annotations: {k8s.apisix.apache.org/http-block-methods: "DELETE,PATCH"}
```

实测：GET 200；DELETE 405。

### 5.7 CSRF 防护

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/enable-csrf: "true"
    k8s.apisix.apache.org/csrf-key: "<签名密钥>"
```

实测：GET 200；POST（无 token）401，响应体 `{"error_msg":"no csrf token in headers"}`。

### 5.8 CORS

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/enable-cors: "true"
    k8s.apisix.apache.org/cors-allow-origin: "https://allowed.example"
    k8s.apisix.apache.org/cors-allow-methods: "GET,POST"
    k8s.apisix.apache.org/cors-allow-headers: "Origin,Authorization"
```

实测（OPTIONS 预检）：

```text
Access-Control-Allow-Origin: https://allowed.example
Access-Control-Allow-Methods: GET,POST
Access-Control-Allow-Headers: Origin,Authorization
Access-Control-Max-Age: 5
```

> 同一插件同时出现在 ApisixPluginConfig 和 Ingress 注解中时，当前版本由注解生成的插件配置完整覆盖 PluginConfig 中的同名插件配置，建议避免重复定义。

---

## 6. 限流限速

### 6.1 limit-count（固定窗口计数）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: rl-count, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - name: limit-count
    enable: true
    config: {count: 3, time_window: 60, rejected_code: 429, key_type: var, key: remote_addr}
---
# Ingress 通过注解引用
metadata:
  annotations: {k8s.apisix.apache.org/plugin-config-name: rl-count}
```

实测（同一客户端连续 5 次）：`200 200 200 429 429`。

窗口重置：`count=1, time_window=10` 时 200 → 429 → 等待 11s 后 200。

### 6.2 按请求头维度限流

```yaml
config: {count: 2, time_window: 60, rejected_code: 429, key_type: var, key: http_x_key}
```

实测：`X-Key: a` 连续请求 200/200/429；`X-Key: b` 独立配额 200/200/429。

### 6.3 limit-req（漏桶 QPS 限速）

```yaml
- name: limit-req
  enable: true
  config: {rate: 1, burst: 0, rejected_code: 429, key_type: var, key: remote_addr}
```

实测：连续 3 次请求 `200 429 429`。

### 6.4 limit-conn（并发连接数限制）

```yaml
- name: limit-conn
  enable: true
  config: {conn: 1, burst: 0, default_conn_delay: 0.1, rejected_code: 429, key_type: var, key: remote_addr}
```

实测（后端为约 5s 的慢服务）：第 1 个请求占用连接期间，第 2 个并发请求快速返回 429；第 1 个请求完成后恢复正常。

> 构造并发场景需要"慢后端"；推荐用 nginx 反代不可达 IP（`proxy_pass http://<不可达IP>` + 长超时）作为稳定的挂起后端。

### 6.5 组合建议

| 目标 | 推荐插件 |
|---|---|
| 按租户/IP 的每日调用量 | limit-count（key_type: var，key: remote_addr 或 http_x_tenant） |
| 接口 QPS 平滑限速 | limit-req（rate + burst） |
| 保护后端并发 | limit-conn（conn + default_conn_delay） |

> 上述示例使用节点本地计数。多 APISIX 节点需要全局统一配额时，应选择适合的共享策略并单独验证。

---

## 7. 可观测性

### 7.1 访问日志

APISIX 访问日志输出到容器 stdout，字段包含上游地址、上游状态、各阶段耗时，可直接用于定位是网关还是后端问题：

```text
$remote_addr - $remote_user [$time_local] $http_host "$request" $status $body_bytes_sent $request_time "$http_referer" "$http_user_agent" $upstream_addr $upstream_status $upstream_response_time "$upstream_scheme://$upstream_host$upstream_uri"
```

实测样例：

```text
<client-ip> - - [...] app.example.com "GET / HTTP/1.1" 200 41 0.004 "-" "curl/8.4.0" <upstream-ip>:80 200 0.005 "http://app.example.com"
```

### 7.2 错误日志

上游连接失败、超时等会写入错误日志，例如：

```text
[error] connect() failed (111: Connection refused) while connecting to upstream, request: "GET / HTTP/1.1", upstream: "http://<node-ip>:80/", host: "app.example.com"
```

查看方式：`kubectl -n <实例命名空间> logs ds/<实例>-apisix --tail=200 | grep -iE 'error|failed'`

### 7.3 Admin API 配置可见性

```bash
curl -s -D - -o /dev/null http://<APISIX节点IP>:<ADMIN_PORT>/apisix/admin/configs \
  -H "X-API-KEY: $ADMIN_API_KEY" | grep -iE 'x-digest|x-last-modified'
# 按 K8s 资源名定位路由
curl -s http://<APISIX节点IP>:<ADMIN_PORT>/apisix/admin/configs -H "X-API-KEY: $ADMIN_API_KEY" \
  | jq '[.routes[] | select(.labels."k8s/name"=="<Ingress名>")]'
```

实测：`X-Digest / X-Last-Modified` 正常返回，可用于判断配置是否已下发。它不代表上游可达或业务请求成功，仍需执行数据面请求；多节点实例应逐节点比较 digest。

### 7.4 控制器指标（Prometheus）

```bash
kubectl -n <实例命名空间> port-forward deploy/<实例>-ingress-controller 18080:8080 &
curl -s http://127.0.0.1:18080/metrics | grep -E 'apisix_ingress_adc'
```

实测存在 `apisix_ingress_adc_sync_duration_seconds_count`、`apisix_ingress_adc_execution_errors_total` 等控制器指标。

### 7.5 APISIX 自身指标（Prometheus）

数据面为 hostNetwork，Prometheus 端口只监听 127.0.0.1，需在 APISIX 所在节点上访问：

```bash
# 在 APISIX Pod 所在节点执行
curl -s http://127.0.0.1:9091/apisix/prometheus/metrics | head
```

实测：HTTP 200，返回 `apisix_http_requests_total`、连接数等实例级指标。

!注意 `kubectl port-forward` 无法访问该端口（仅监听节点回环地址）。

!注意 按路由维度的 `apisix_http_status` / `apisix_bandwidth` / `apisix_http_latency` 需先启用 `prometheus` 插件（可用 ApisixGlobalRule 全局启用，`prefer_name: true` 时标签使用路由名）。

### 7.6 请求 ID（request-id）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: req-id, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
  - {name: request-id, enable: true, config: {include_in_response: true, algorithm: uuid}}
```

实测：响应头返回 UUID 格式的 `X-Request-Id`。如需用该值关联访问日志，必须先在 access log format 中加入对应的 request ID 变量；当前默认日志格式不包含该字段。

---

## 8. TLS

### 8.1 HTTPS 卸载

标准 Ingress `spec.tls` 引用 `kubernetes.io/tls` 类型 Secret 即可，无需额外 CRD：

```yaml
spec:
  ingressClassName: <INSTANCE_CLASS>
  tls:
  - hosts: [app.example.com]
    secretName: app-tls
```

实测：`curl --resolve app.example.com:<HTTPS_PORT>:<APISIX节点IP> https://app.example.com:<HTTPS_PORT>/` 返回 200，证书为 Secret 中的证书；不带 SNI 访问时回落到实例的 fallback 证书。

### 8.2 双向认证（mTLS）

客户端证书校验通过 ApisixTls 配置，`hosts` 与 Ingress 中的域名一致即可对该 Ingress 生效：

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixTls
metadata: {name: app-mtls, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  hosts: [app.example.com]
  secret: {name: app-tls, namespace: <业务命名空间>}
  client:
    caSecret: {name: app-ca, namespace: <业务命名空间>}
    depth: 2
```

实测：不带客户端证书时 TLS 握手失败，带证书时返回 200。

!注意 `caSecret` 对应的 Secret 只能包含 `ca.crt`；同一域名不要同时使用 Ingress `spec.tls`，证书应统一放到 `ApisixTls.spec.secret`。

---

## 9. 验证方法说明与已知限制

### 9.1 通用验证方法

1. 准备一个可回显的后端（响应体/响应头包含版本号与收到的 URI、Host），便于判断网关行为；
2. 每条能力用独立域名，避免用例互相影响；
3. 变更后用 Admin API 确认配置已下发（`X-Digest` 变化 + 按 `k8s/name` 查到的 route/service/upstream 字段）；
4. 用 `curl -H 'Host: <域名>' http://<KEEPALIVED_VIP>:<HTTP_PORT>/` 绕过 DNS 直接验证网关；
5. 通过 APISIX 访问日志核对实际命中的上游节点与状态码。

### 9.2 已知限制与注意事项

| # | 限制 | 说明与规避 |
|---|---|---|
| 1 | 网关内建健康检查不在本文范围 | 上游节点级 health check 未通过 Ingress 注解 / BackendTrafficPolicy 暴露（该 CRD 无 `healthCheck` 字段）；api-breaker 可做路由级快速失败，但不能替代节点健康检查 |
| 2 | `auth-type` 注解范围有限 | 只支持 keyAuth / basicAuth；jwtAuth 需通过 ApisixPluginConfig 挂载 |
| 3 | 重试触发条件有限 | 当前实测中连接拒绝会重试其他节点，后端返回 502 未透明重试；其他状态码和失败阶段需专项验证 |
| 4 | CRD 字段名大小写敏感 | 例如 JWT 使用 `private_key`（snake_case）；建议先 `kubectl apply --dry-run=server` 预检 |
| 5 | Controller 使用 EndpointSlice | selectorless Service 需要标签、端口匹配的 EndpointSlice；仅创建 legacy Endpoints 不能保证生成节点 |
| 6 | 数据面配置在内存 | Pod 重启后依赖控制器重新下发；恢复时间受事件、周期同步、ADC 可用性和重试状态影响 |
| 7 | 同名插件配置覆盖 | ApisixPluginConfig 与 Ingress 注解重复定义同名插件时，当前版本由注解配置覆盖 |
| 8 | 配置整体下发 | 标准模式下全量配置一次下发，任一插件配置校验失败会导致本批次所有 Ingress 都不生效（数据面日志可见 `invalid routes at index N`），变更后应确认 Admin API 配置已更新 |
| 9 | Admin API 属于高权限控制面 | 应限制网络访问并使用 Secret 管理 AdminKey，不应把 key 明文写入普通 ConfigMap 或文档 |

---

## 10. 附录：复测清单

按顺序执行即可逐项复测（占位符按《APISIX 网关用户使用手册》第 1 章获取）：

```bash
VIP=<KEEPALIVED_VIP>; PORT=<HTTP_PORT>
c() { curl -s -o /dev/null -w '%{http_code}\n' -H "Host: $1" "${@:2}"; }

# 路由
c app.example.com            http://$VIP:$PORT/            # 期望 200
c nope.example.com           http://$VIP:$PORT/            # 期望 404
c regex.example.com          http://$VIP:$PORT/users/123   # 期望 200
c method.example.com         -X POST http://$VIP:$PORT/    # 期望 200，并从响应确认命中 v2

# 限流
for i in 1 2 3 4 5; do c rl.example.com http://$VIP:$PORT/; done   # 期望 200×3 + 429×2

# 认证
c key.example.com            http://$VIP:$PORT/            # 期望 401
c key.example.com            -H 'apikey: <key>' http://$VIP:$PORT/  # 期望 200

# 可观测性
curl -s http://<APISIX节点IP>:<ADMIN_PORT>/apisix/admin/configs -H "X-API-KEY: $ADMIN_API_KEY" \
  | jq -c '{routes:(.routes|length), upstreams:(.upstreams|length)}'
```

---

**维护说明**：本清单随 APISIX / Ingress Controller 版本升级需要重新复测；Ingress 相关注解或 CRD 能力发生变化时同步更新。
