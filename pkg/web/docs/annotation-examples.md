# 注解逐条转换示例

> 本文档为 `ingress2apisix` 支持的每条 NGINX Ingress 注解提供一个最小化转换示例。
> 输出基于转换器实际代码逻辑（`pkg/converter/converter.go`），默认值参照 ingress-nginx 源码验证。

---

## 目录

- [跨域 CORS](#1-跨域-cors)
- [HTTPS 重定向](#2-https-重定向)
- [路径重写](#3-路径重写)
- [代理超时](#4-代理超时)
- [后端协议](#5-后端协议)
- [外部认证（forward-auth）](#6-外部认证forward-auth)
- [基础认证](#7-基础认证basic-auth)
- [IP 访问控制](#8-ip-访问控制)
- [WebSocket](#9-websocket)
- [正则路径](#10-正则路径)
- [请求体大小](#11-请求体大小)
- [请求缓冲](#12-请求缓冲)
- [速率限制](#13-速率限制)
- [连接限制](#14-连接限制)
- [上游主机改写](#15-上游主机改写)
- [真实客户端 IP（real-ip）](#16-真实客户端-ipreal-ip)
- [代理 Cookie Path 改写](#17-代理-cookie-path-改写)
- [代理 Cookie Flags](#18-代理-cookie-flags)
- [会话亲和（cookie affinity）](#19-会话亲和cookie-affinity)
- [upstream-hash-by](#20-upstream-hash-by)
- [健康检查](#21-健康检查)
- [错误页面](#22-错误页面)
- [重定向](#23-重定向)
- [访问日志](#24-访问日志)
- [TLS 配置（ApisixTls）](#25-tls-配置apisixtls)
- [代理重定向（scheme 重写）](#26-代理重定向scheme-重写)
- [Server Snippet / Configuration Snippet](#27-server-snippet--configuration-snippet)
- [上游 keepalive](#28-上游-keepalive)
- [SSL 验证](#29-ssl-验证)
- [限流乘数（limit-multiplier）](#30-限流乘数limit-multiplier)

---

## 1. 跨域 CORS

### 1.1 单独启用 CORS（使用 ingress-nginx 默认值）

> ingress-nginx 默认 `allow_credential=true`、`max_age=1728000`，methods/headers 使用具体列表（非 `*`）。
> APISIX `cors` 插件在 `allow_credential: true` 时不允许 `*`，通配 origin 使用 `allow_origins_by_regex` 替代。

**输入：**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
spec:
  ingressClassName: nginx
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-svc
                port:
                  number: 80
```

**转换输出：**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    k8s.apisix.apache.org/plugin-config-name: my-app-plugins
    ingressClassName: apisix
  # ... (spec unchanged)
---
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: my-app-plugins
  namespace: default
  labels:
    managed-by: ingress2apisix
    ingress-name: my-app
spec:
  ingressClassName: apisix
  plugins:
    # Source: nginx.ingress.kubernetes.io/enable-cors
    - name: cors
      enable: true
      config:
        allow_methods: "GET, PUT, POST, DELETE, PATCH, OPTIONS"      # Default — set nginx.ingress.kubernetes.io/cors-allow-methods to customize
        allow_headers: "DNT,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range,Authorization"  # Default — set nginx.ingress.kubernetes.io/cors-allow-headers to customize
        allow_credential: true                                       # Default — ingress-nginx default is true
        max_age: 1728000                                             # Default — ingress-nginx default is 1728000
        allow_origins_by_regex:                                      # Default — ingress-nginx default origin is *, using regex to allow all with credentials
          - ".*"
```

### 1.2 CORS + 自定义 origin + 关闭 credentials

**输入：**

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/cors-allow-origin: "https://app.example.com"
    nginx.ingress.kubernetes.io/cors-allow-credentials: "false"
    nginx.ingress.kubernetes.io/cors-max-age: "3600"
    nginx.ingress.kubernetes.io/cors-expose-headers: "X-Custom"
```

**转换输出（ApisixPluginConfig 部分）：**

```yaml
plugins:
  - name: cors
    enable: true
    config:
      allow_origins: "https://app.example.com"                       # Source: nginx.ingress.kubernetes.io/cors-allow-origin
      allow_methods: "GET, PUT, POST, DELETE, PATCH, OPTIONS"        # Default
      allow_headers: "DNT,Keep-Alive,User-Agent,..."                 # Default
      allow_credential: false                                        # Source: nginx.ingress.kubernetes.io/cors-allow-credentials
      max_age: 3600                                                  # Source: nginx.ingress.kubernetes.io/cors-max-age
      expose_headers: "X-Custom"                                     # Source: nginx.ingress.kubernetes.io/cors-expose-headers
```

> 注意：`allow_credential: false` 时可以使用 `allow_origins: "*"`，不再需要 `allow_origins_by_regex` 变通方案。

---

## 2. HTTPS 重定向

### 2.1 `ssl-redirect`

```yaml
# 输入
nginx.ingress.kubernetes.io/ssl-redirect: "true"
```

```yaml
# 输出 (Ingress annotation)
k8s.apisix.apache.org/http-to-https: "true"   # Source: nginx.ingress.kubernetes.io/ssl-redirect
```

### 2.2 `force-ssl-redirect`

```yaml
# 输入
ingress.kubernetes.io/force-ssl-redirect: "true"
```

```yaml
# 输出
k8s.apisix.apache.org/http-to-https: "true"   # Source: ingress.kubernetes.io/force-ssl-redirect
```

---

## 3. 路径重写

### 3.1 简单 rewrite-target

> **语义差异警告：** ingress-nginx 的 `rewrite-target` 是 nginx `rewrite` 指令的**正则替换**，只替换匹配部分、保留子路径。APISIX 的 `rewrite-target` 注解映射到 `proxy-rewrite.uri` 字段，是**整体替换**，原始请求路径被完全丢弃。
>
> - ingress-nginx：`path=/api, rewrite-target=/` + 请求 `/api/foo` → 上游收到 `/foo`
> - APISIX：`rewrite-target: /` + 请求 `/api/foo` → 上游收到 `/`
>
> 如需保留子路径，应使用 `configuration-snippet` 中的 `rewrite` 指令（转换器会自动提取为 `rewrite-target-regex` + `rewrite-target-regex-template`）。

```yaml
# 输入
nginx.ingress.kubernetes.io/rewrite-target: /
```

```yaml
# 输出
k8s.apisix.apache.org/rewrite-target: /       # Source: nginx.ingress.kubernetes.io/rewrite-target
```

### 3.2 正则 rewrite-target（含捕获组 `$1`）

> **已知 Bug：** 当前转换器对含 `$N` 的 rewrite-target 同时将 `rewrite-target-regex` 和 `rewrite-target-regex-template` 设为同一值（如 `/$1`），但 AIC 期望前者为**正则匹配模式**（如 `/api/(.*)`），后者为**替换模板**（如 `/$1`）。建议改用 `configuration-snippet` 方式（见 3.3），由转换器自动提取正确的正则模式和模板。

```yaml
# 输入
nginx.ingress.kubernetes.io/rewrite-target: /$1
spec:
  rules:
    - http:
        paths:
          - path: /api/(.*)
            pathType: Prefix
```

```yaml
# 输出（当前转换器行为 — 存在上述 bug）
k8s.apisix.apache.org/rewrite-target-regex: /$1                        # Source: nginx.ingress.kubernetes.io/rewrite-target
k8s.apisix.apache.org/rewrite-target-regex-template: /$1               # Source: nginx.ingress.kubernetes.io/rewrite-target
```

```yaml
# 正确输出（推荐：通过 configuration-snippet 实现）
# ingress.kubernetes.io/configuration-snippet: |
#   rewrite ^/api/(.*) /$1 break;
# → 转换器自动提取：
k8s.apisix.apache.org/rewrite-target-regex: ^/api/(.*)                 # Source: ingress.kubernetes.io/configuration-snippet
k8s.apisix.apache.org/rewrite-target-regex-template: /$1               # Source: ingress.kubernetes.io/configuration-snippet
```

### 3.3 configuration-snippet 中的单条 rewrite

```yaml
# 输入
ingress.kubernetes.io/configuration-snippet: |
  rewrite ^/iam/(.*) /$1 break;
```

```yaml
# 输出（自动提取为原生注解）
k8s.apisix.apache.org/rewrite-target-regex: ^/iam/(.*)                 # Source: ingress.kubernetes.io/configuration-snippet
k8s.apisix.apache.org/rewrite-target-regex-template: /$1               # Source: ingress.kubernetes.io/configuration-snippet
```

### 3.4 多条 rewrite（→ proxy-rewrite 插件）

```yaml
# 输入
ingress.kubernetes.io/configuration-snippet: |
  rewrite ^/shard/0/(.*) /$1 break;
  rewrite ^/am/(.*) /$1 break;
```

```yaml
# 输出（ApisixPluginConfig）
plugins:
  - name: proxy-rewrite
    enable: true
    config:
      regex_uri:
        - "^/shard/0/(.*)"
        - "/$1"
        - "^/am/(.*)"
        - "/$1"
```

### 3.5 rewrite-target + configuration-snippet 混合（→ proxy-rewrite 插件）

```yaml
# 输入
nginx.ingress.kubernetes.io/rewrite-target: /new/$1
ingress.kubernetes.io/configuration-snippet: |
  rewrite ^/old/(.*) /new/$1 break;
```

```yaml
# 输出（ApisixPluginConfig — 合并为多条 regex_uri）
plugins:
  - name: proxy-rewrite
    enable: true
    config:
      regex_uri:
        - "(?i)/api/(.*)"    # 来自 rewrite-target，按 path 规则生成
        - "/new/$1"
        - "^/old/(.*)"       # 来自 configuration-snippet
        - "/new/$1"
```

---

## 4. 代理超时

```yaml
# 输入
nginx.ingress.kubernetes.io/proxy-connect-timeout: "30"
nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
```

```yaml
# 输出（自动添加 's' 后缀）
k8s.apisix.apache.org/upstream-connect-timeout: "30s"    # Source: nginx.ingress.kubernetes.io/proxy-connect-timeout
k8s.apisix.apache.org/upstream-send-timeout: "3600s"     # Source: nginx.ingress.kubernetes.io/proxy-send-timeout
k8s.apisix.apache.org/upstream-read-timeout: "3600s"     # Source: nginx.ingress.kubernetes.io/proxy-read-timeout
```

> 注：原始值如果已带 `s` 后缀则不会重复添加。

---

## 5. 后端协议

```yaml
# 输入
nginx.ingress.kubernetes.io/backend-protocol: GRPC
```

```yaml
# 输出（转小写）
k8s.apisix.apache.org/upstream-scheme: "grpc"            # Source: nginx.ingress.kubernetes.io/backend-protocol
```

> 支持值：`grpc`、`grpcs`、`https`、`http`

---

## 6. 外部认证（forward-auth）

### 6.1 auth-url + auth-response-headers

```yaml
# 输入
nginx.ingress.kubernetes.io/auth-url: http://auth-svc.default.svc.cluster.local/verify
nginx.ingress.kubernetes.io/auth-response-headers: Authorization,X-User-Id
nginx.ingress.kubernetes.io/auth-signin: https://login.example.com
```

```yaml
# 输出
k8s.apisix.apache.org/auth-uri: http://auth-svc.default.svc.cluster.local/verify     # Source: nginx.ingress.kubernetes.io/auth-url
k8s.apisix.apache.org/auth-upstream-headers: Authorization,X-User-Id                   # Source: nginx.ingress.kubernetes.io/auth-response-headers
k8s.apisix.apache.org/auth-signin: https://login.example.com                          # Source: nginx.ingress.kubernetes.io/auth-signin
```

### 6.2 auth-method + auth-request-headers

```yaml
# 输入
nginx.ingress.kubernetes.io/auth-method: POST
nginx.ingress.kubernetes.io/auth-request-headers: User-Agent,Cookie
```

```yaml
# 输出（method 自动转大写）
k8s.apisix.apache.org/auth-method: "POST"                                             # Source: nginx.ingress.kubernetes.io/auth-method
k8s.apisix.apache.org/auth-request-headers: "User-Agent,Cookie"                       # Source: nginx.ingress.kubernetes.io/auth-request-headers
```

> 注意：`auth-method` 和 `auth-request-headers` 为 APISIX Ingress Controller 原生支持的注解，可直接映射。

---

## 7. 基础认证（Basic Auth）

```yaml
# 输入
nginx.ingress.kubernetes.io/auth-type: basic
nginx.ingress.kubernetes.io/auth-realm: "401: Authentication Required"
```

```yaml
# 输出（Ingress 注解映射）
k8s.apisix.apache.org/auth-type: "basicAuth"                                          # Source: nginx.ingress.kubernetes.io/auth-type
k8s.apisix.apache.org/auth-realm: "401: Authentication Required"                      # Source: nginx.ingress.kubernetes.io/auth-realm
```

> 注意：`auth-secret` 不被 AIC 支持，需手动创建 `ApisixConsumer` CRD 配置凭据。

### 7.1 auth-type: digest（不支持自动转换）

> ingress-nginx 源码（`internal/ingress/annotations/auth/main.go:45`）确认 `auth-type` 接受 `basic|digest`。
> `digest` 类型使用 nginx 的 `ngx_http_auth_digest_module`。APISIX 没有原生 HTTP Digest 认证插件，无法等价转换。
> hxk8s1 JSON 中仅出现 `auth-type: basic`（1 次），无 `digest` 实例。

```yaml
# 输入
nginx.ingress.kubernetes.io/auth-type: digest
nginx.ingress.kubernetes.io/auth-secret: my-digest-secret
nginx.ingress.kubernetes.io/auth-realm: "Digest Auth Required"
```

```yaml
# 输出 — 跳过 auth-type 转换，产生告警
# 告警：[ns/name] auth-type="digest" 无 APISIX 等价方案，需手动使用 APISIX consumer + 自定义插件实现
# auth-realm 仍会被转换（用于其他认证场景）
k8s.apisix.apache.org/auth-realm: "Digest Auth Required"               # Source: nginx.ingress.kubernetes.io/auth-realm
```

---

## 8. IP 访问控制

### 8.1 白名单

```yaml
# 输入
ingress.kubernetes.io/whitelist-source-range: 10.20.0.0/24,192.168.10.0/24
```

```yaml
# 输出
k8s.apisix.apache.org/allowlist-source-range: "10.20.0.0/24,192.168.10.0/24"         # Source: ingress.kubernetes.io/whitelist-source-range
```

### 8.2 黑名单

```yaml
# 输入
nginx.ingress.kubernetes.io/denylist-source-range: 172.16.0.0/12
```

```yaml
# 输出
k8s.apisix.apache.org/blocklist-source-range: "172.16.0.0/12"                         # Source: nginx.ingress.kubernetes.io/denylist-source-range
```

---

## 9. WebSocket

### 9.1 概述

> `websocket-services` 是 **F5 NGINX Ingress Controller**（`nginx.org/websocket-services`）的注解，用于指定哪些 Service 启用 WebSocket。社区 ingress-nginx **不需要任何注解**即可支持 WebSocket（默认开启）。
>
> AIC 的 `enable-websocket` 是路由级布尔开关，不支持按 Service 指定。因此当从 F5 NGINX Ingress Controller 迁移时，`websocket-services` 的值（具体服务名）会被丢弃，统一转为路由级 `enable-websocket: "true"`。
>
> 如需保留按 Service 差异化配置，需手动拆分为多条 Ingress 规则。

```yaml
# 输入（F5 NGINX Ingress Controller 格式）
nginx.org/websocket-services: my-ws-svc
```

```yaml
# 输出 — 路由级启用 WebSocket，my-ws-svc 值丢失
k8s.apisix.apache.org/enable-websocket: "true"                          # Source: nginx.org/websocket-services
```

> **注意：** 社区 ingress-nginx 默认支持 WebSocket，无需 `websocket-services` 注解。从社区 ingress-nginx 迁移时无需处理此注解；如需调整 WebSocket 超时，设置 `proxy-read-timeout` 和 `proxy-send-timeout` 即可。

---

## 10. 正则路径

```yaml
# 输入
nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  rules:
    - http:
        paths:
          - path: /api/v[0-9]+/.*
            pathType: ImplementationSpecific
```

```yaml
# 输出
k8s.apisix.apache.org/use-regex: "true"                                               # Source: nginx.ingress.kubernetes.io/use-regex
```

> 转换器还会自动调整 `pathType`：正则路径从 `Prefix` 改为 `ImplementationSpecific`。
> 无正则的 `ImplementationSpecific` 会被改为 `Prefix`。

---

## 11. 请求体大小

```yaml
# 输入
nginx.ingress.kubernetes.io/proxy-body-size: "50m"
```

```yaml
# 输出（ApisixPluginConfig — client-control 插件，自动转为字节）
plugins:
  - name: client-control
    enable: true
    config:
      max_body_size: 52428800                                       # Source: nginx.ingress.kubernetes.io/proxy-body-size
```

> 支持单位：`k`/`K`、`m`/`M`、`g`/`G`，无单位按字节处理。`0` 表示不限制。

---

## 12. 请求缓冲

```yaml
# 输入
nginx.ingress.kubernetes.io/proxy-request-buffering: "off"
```

```yaml
# 输出（ApisixPluginConfig — proxy-control 插件）
plugins:
  - name: proxy-control
    enable: true
    config:
      request_buffering: false                                      # Source: nginx.ingress.kubernetes.io/proxy-request-buffering
```

> 仅当值为 `off` 或 `false` 时才生成插件。

---

## 13. 速率限制

### 13.1 limit-rps（每秒请求数）

```yaml
# 输入
nginx.ingress.kubernetes.io/limit-rps: "1000"
ingress.kubernetes.io/configuration-snippet: |
  limit_req_status 429;
```

```yaml
# 输出（ApisixPluginConfig — limit-req 插件）
plugins:
  - name: limit-req
    enable: true
    config:
      rate: 1000                                                    # Source: nginx.ingress.kubernetes.io/limit-rps
      burst: 0                                                      # Default — hardcoded, same as nginx default
      key: remote_addr                                              # Default — hardcoded, same as nginx default
      rejected_code: 429                                            # Source: ingress.kubernetes.io/configuration-snippet (limit_req_status)
```

### 13.2 limit-rpm（每分钟请求数，自动转为 rps）

```yaml
# 输入
nginx.ingress.kubernetes.io/limit-rpm: "6000"
```

```yaml
# 输出（自动除以 60 转换）
plugins:
  - name: limit-req
    enable: true
    config:
      rate: 100                                                     # 6000 / 60 = 100 rps
      burst: 0
      key: remote_addr
      rejected_code: 429
```

### 13.3 limit-multiplier（限流乘数）

```yaml
# 输入
nginx.ingress.kubernetes.io/limit-rps: "100"
nginx.ingress.kubernetes.io/limit-multiplier: "3"
```

```yaml
# 输出（rate = 100 * 3 = 300）
plugins:
  - name: limit-req
    enable: true
    config:
      rate: 300
      burst: 0
      key: remote_addr
      rejected_code: 429
```

---

## 14. 连接限制

```yaml
# 输入
nginx.ingress.kubernetes.io/limit-connections: "100"
```

```yaml
# 输出（ApisixPluginConfig — limit-conn 插件）
plugins:
  - name: limit-conn
    enable: true
    config:
      conn: 100                                                     # Source: nginx.ingress.kubernetes.io/limit-connections
      burst: 0                                                      # Default — hardcoded, same as nginx default
      key: remote_addr                                              # Default — hardcoded, same as nginx default
      rejected_code: 503                                            # Default — hardcoded, same as nginx default
```

---

## 15. 上游主机改写

```yaml
# 输入
nginx.ingress.kubernetes.io/upstream-vhost: internal.example.com
```

```yaml
# 输出（ApisixPluginConfig — proxy-rewrite 插件）
plugins:
  - name: proxy-rewrite
    enable: true
    config:
      host: internal.example.com                                    # Source: nginx.ingress.kubernetes.io/upstream-vhost
```

---

## 16. 真实客户端 IP（real-ip）

### 16.1 enable-real-ip 单独使用

```yaml
# 输入
nginx.ingress.kubernetes.io/enable-real-ip: "true"
```

```yaml
# 输出（ApisixPluginConfig — real-ip 插件）
plugins:
  - name: real-ip
    enable: true
    config:
      source: http_x_forwarded_for                                  # Default — hardcoded, same as nginx default
      trusted_addresses:                                             # Default — hardcoded to accept all
        - "0.0.0.0/0"
      recursive: false                                               # Default — hardcoded default
```

### 16.2 forwarded-for-header + use-forwarded-headers

```yaml
# 输入
nginx.ingress.kubernetes.io/forwarded-for-header: X-Real-IP
nginx.ingress.kubernetes.io/use-forwarded-headers: "true"
```

```yaml
# 输出（自动启用 real-ip 插件）
plugins:
  - name: real-ip
    enable: true
    config:
      source: http_x_real_ip                                        # 自动将 "X-Real-IP" 转为 nginx 变量格式
      trusted_addresses:
        - "0.0.0.0/0"
      recursive: true                                                # Source: nginx.ingress.kubernetes.io/use-forwarded-headers
```

> 告警：`forwarded-for-header/use-forwarded-headers 未配合 enable-real-ip 使用，已自动启用 real-ip 插件`
> 若同时设置 `compute-full-forwarded-for=true`，会额外输出告警（APISIX real-ip 插件无等价参数）。

---

## 17. 代理 Cookie Path 改写

```yaml
# 输入
ingress.kubernetes.io/proxy-cookie-path: ~^/api/(.*) /$1
```

```yaml
# 输出（ApisixPluginConfig — proxy-cookie-path 自定义插件）
plugins:
  - name: proxy-cookie-path
    enable: true
    config:
      path_pairs:
        - match: "~^/api/(.*)"
          replacement: "/$1"
```

> 支持精确匹配和正则匹配（`~` 前缀）。多条规则用逗号分隔。

---

## 18. 代理 Cookie Flags

```yaml
# 输入
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_cookie_flags sessionid SameSite=None Secure;
  proxy_cookie_flags auth_token HttpOnly Secure;
```

```yaml
# 输出（ApisixPluginConfig — proxy-cookie-flags 自定义插件）
plugins:
  - name: proxy-cookie-flags
    enable: true
    config:
      rules:
        - match: "sessionid"
          flags: ["SameSite=None", "Secure"]
        - match: "auth_token"
          flags: ["HttpOnly", "Secure"]
```

---

## 19. 会话亲和（cookie affinity）

### 19.1 完整配置（affinity + session-cookie-name + session-cookie-hash）

```yaml
# 输入
ingress.kubernetes.io/affinity: cookie
ingress.kubernetes.io/session-cookie-name: MYSESSION
ingress.kubernetes.io/session-cookie-hash: sha256
ingress.kubernetes.io/session-cookie-max-age: "86400"
ingress.kubernetes.io/session-cookie-path: /app
```

```yaml
# 输出

# 1. session-cookie-hash 自定义插件（ApisixPluginConfig）
plugins:
  - name: session-cookie-hash
    enable: true
    config:
      cookie_name: "MYSESSION"                                      # Source: ingress.kubernetes.io/session-cookie-name
      algorithm: "sha256"                                           # Source: ingress.kubernetes.io/session-cookie-hash
      header_name: "X-Session-Hash"                                 # Default — hardcoded default
      fallback: "pass"                                              # Default — hardcoded default
      generate_cookie: true                                         # Default — hardcoded default
      cookie_httponly: false                                         # Default — hardcoded default
      max_age: 86400                                                # Source: ingress.kubernetes.io/session-cookie-max-age
      cookie_path: "/app"                                           # Source: ingress.kubernetes.io/session-cookie-path

# 2. BackendTrafficPolicy（cookie 亲和）
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: my-app-cookie-affinity
  namespace: default
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: my-svc
  loadbalancer:
    type: chash
    hashOn: cookie
    key: MYSESSION
```

### 19.2 仅 affinity: cookie（未配置 session-cookie-name 和 hash）

```yaml
# 输入
ingress.kubernetes.io/affinity: cookie
```

```yaml
# 输出 — 自动生成 sha1 默认算法 + INGRESSCOOKIE 默认名称
# 告警：affinity=cookie 未配置 session-cookie-hash，默认使用 sha1 生成 session-cookie-hash 插件
# 告警：未配置 session-cookie-name，session-cookie-hash 插件将使用默认 cookie_name=INGRESSCOOKIE（首次请求无该 Cookie 时插件会自动生成并返回 Set-Cookie）

plugins:
  - name: session-cookie-hash
    enable: true
    config:
      cookie_name: "INGRESSCOOKIE"                                  # Default — set ingress.kubernetes.io/session-cookie-name to customize
      algorithm: "sha1"                                             # Default — set ingress.kubernetes.io/session-cookie-hash to customize
      header_name: "X-Session-Hash"
      fallback: "pass"
      generate_cookie: true
      cookie_httponly: false

# BackendTrafficPolicy:
#   loadbalancer.key: "INGRESSCOOKIE"
#   告警：affinity=cookie 未配置 session-cookie-name，BackendTrafficPolicy 将使用默认 key=INGRESSCOOKIE（session-cookie-hash 插件会自动生成该 Cookie）
```

### 19.3 session-cookie-conditional-samesite-none（不支持）

```yaml
# 输入
ingress.kubernetes.io/session-cookie-conditional-samesite-none: "true"
```

```yaml
# 输出 — 仅产生告警
# 告警：session-cookie-conditional-samesite-none: APISIX session-cookie-hash 插件无此参数，SameSite=None 需通过代理 Cookie 标志或应用层处理
```

---

## 20. upstream-hash-by

```yaml
# 输入
nginx.ingress.kubernetes.io/upstream-hash-by: $remote_addr
```

```yaml
# 输出（BackendTrafficPolicy — chash 负载均衡）
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: my-app-hash-by
  namespace: default
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: my-svc
  loadbalancer:
    type: chash
    hashOn: vars
    key: remote_addr
```

> 支持变量映射：`$remote_addr` → `vars`/`remote_addr`，`$arg_xxx` → `vars`/`arg_xxx`，`$http_xxx` → `header`/`xxx`，`$request_uri` → `vars`/`request_uri`

---

## 21. 健康检查

```yaml
# 输入
nginx.ingress.kubernetes.io/health-check-path: /healthz
nginx.ingress.kubernetes.io/health-check-interval: "10"
nginx.ingress.kubernetes.io/health-check-timeout: "5"
nginx.ingress.kubernetes.io/health-check-retries: "3"
```

```yaml
# 输出（ApisixUpstream CRD）
apiVersion: apisix.apache.org/v2
kind: ApisixUpstream
metadata:
  name: my-app-upstream
  namespace: default
spec:
  ingressClassName: apisix
  healthCheck:
    active:
      type: http                                                    # Default — hardcoded to http
      httpPath: /healthz                                            # Source: nginx.ingress.kubernetes.io/health-check-path
      timeout: "5s"                                                 # Source: nginx.ingress.kubernetes.io/health-check-timeout
      healthy:
        successes: 3                                                # Source: nginx.ingress.kubernetes.io/health-check-retries
        interval: "10s"                                             # Source: nginx.ingress.kubernetes.io/health-check-interval
      unhealthy:
        httpCodes:                                                  # Default — hardcoded, same as nginx default
          - 500
          - 502
          - 503
          - 504
        tcpFailures: 3                                              # Default — hardcoded default
        timeouts: 3
```

> 若未指定 `health-check-path` 但指定了 interval/retries/timeout，默认使用 `/` 并输出告警。

---

## 22. 错误页面

```yaml
# 输入
ingress.kubernetes.io/custom-http-errors: "404,500"
```

```yaml
# 输出（Ingress 注解映射）
k8s.apisix.apache.org/custom-error-codes: "404,500"                  # Source: ingress.kubernetes.io/custom-http-errors
```

> 注意：仅映射注解。实际错误页实现需要额外的 `ApisixPluginConfig` + `custom-error-page` 自定义插件（参见迁移文档 4.1.3）。

---

## 23. 重定向

### 23.1 permanent-redirect（永久重定向 308）

```yaml
# 输入
nginx.ingress.kubernetes.io/permanent-redirect: https://new.example.com
```

```yaml
# 输出
k8s.apisix.apache.org/http-redirect: "https://new.example.com"       # Source: nginx.ingress.kubernetes.io/permanent-redirect
k8s.apisix.apache.org/http-redirect-code: "308"                      # Source: nginx.ingress.kubernetes.io/permanent-redirect
```

### 23.2 temporal-redirect（临时重定向 302）

```yaml
# 输入
nginx.ingress.kubernetes.io/temporal-redirect: https://temp.example.com
```

```yaml
# 输出
k8s.apisix.apache.org/http-redirect: "https://temp.example.com"      # Source: nginx.ingress.kubernetes.io/temporal-redirect
k8s.apisix.apache.org/http-redirect-code: "302"                      # Source: nginx.ingress.kubernetes.io/temporal-redirect
```

### 23.3 app-root（首页重定向）

```yaml
# 输入
nginx.ingress.kubernetes.io/app-root: /dashboard
```

```yaml
# 输出
k8s.apisix.apache.org/http-redirect: "/dashboard"                    # Source: nginx.ingress.kubernetes.io/app-root
```

---

## 24. 访问日志

```yaml
# 输入
nginx.ingress.kubernetes.io/enable-access-log: "false"
```

```yaml
# 输出
k8s.apisix.apache.org/enable-access-log: "false"                    # Source: nginx.ingress.kubernetes.io/enable-access-log
```

> 支持值：`"false"`/`"off"` → `"false"`，其他值 → `"true"`

---

## 25. TLS 配置（ApisixTls）

```yaml
# 输入（Ingress spec.tls）
spec:
  tls:
    - hosts:
        - app.example.com
      secretName: app-tls-secret
```

```yaml
# 输出（自动生成 ApisixTls CRD）
apiVersion: apisix.apache.org/v2
kind: ApisixTls
metadata:
  name: my-app-tls
  namespace: default
spec:
  ingressClassName: apisix
  hosts:
    - app.example.com
  secret:
    name: app-tls-secret
    namespace: default
```

> 多个 TLS 条目会生成多个 `ApisixTls`，名称自动添加后缀 `-1`、`-2` 等。

---

## 26. 代理重定向（scheme 重写）

### 26.1 HTTP → HTTPS（自动识别为 SSL 重定向）

```yaml
# 输入
ingress.kubernetes.io/proxy-redirect-from: http://
ingress.kubernetes.io/proxy-redirect-to: https://
```

```yaml
# 输出（识别为等价于 ssl-redirect）
k8s.apisix.apache.org/http-to-https: "true"                          # Source: ingress.kubernetes.io/proxy-redirect-from, ingress.kubernetes.io/proxy-redirect-to
```

### 26.2 非 SSL 场景（不支持自动转换）

```yaml
# 输入
ingress.kubernetes.io/proxy-redirect-from: http://old.internal
ingress.kubernetes.io/proxy-redirect-to: http://new.internal
```

```yaml
# 输出 — 仅产生告警
# 告警：proxy-redirect-from=http://old.internal proxy-redirect-to=http://new.internal 不是 HTTP→HTTPS 场景，无法自动转换，需手动处理 scheme 重定向
```

---

## 27. Server Snippet / Configuration Snippet

### 27.1 server-snippet（不支持自动转换）

```yaml
# 输入
nginx.ingress.kubernetes.io/server-snippet: |
  location = /health {
    return 200 'ok';
  }
```

```yaml
# 输出 — 仅产生告警
# 告警：server-snippet 不支持自动转换，需手动迁移至 APISIX 路由或插件配置
```

### 27.2 configuration-snippet 中的 more_set_headers

```yaml
# 输入
nginx.ingress.kubernetes.io/configuration-snippet: |
  more_set_headers "X-Forwarded-For $http_x_forwarded_for";
```

```yaml
# 输出 — 产生告警（需通过 proxy-rewrite 插件手动配置）
# 告警：configuration-snippet 中包含 more_set_headers/rewrite/proxy_set_header 等指令，无法自动转换，需手动迁移至 APISIX 插件
```

> 注意：`proxy_cookie_flags` 和 `rewrite` 指令会被自动提取并转换（见第 18、3 节），但 `more_set_headers`、`proxy_set_header` 等其他指令需要手动迁移。

---

## 28. 上游 keepalive

```yaml
# 输入
nginx.ingress.kubernetes.io/upstream-keepalive-connections: "320"
nginx.ingress.kubernetes.io/upstream-keepalive-requests: "10000"
nginx.ingress.kubernetes.io/upstream-keepalive-timeout: "60"
```

```yaml
# 输出 — 仅产生告警
# 告警：upstream-keepalive-* 已识别，但 AIC 的 ApisixUpstream CRD 不支持 keepalive 字段；需在 APISIX 全局配置或 apisix_upstream 中手动设置
```

---

## 29. SSL 验证

```yaml
# 输入
nginx.ingress.kubernetes.io/ssl-verify: "true"
```

```yaml
# 输出 — 仅产生告警
# 告警：ssl-verify=true: 需在 ApisixUpstream CRD 中配置 TLS 验证（tls.client_cert/tls.client_key），或使用 proxy-ssl 插件
```

> `ssl-verify=false` 同样产生告警说明 APISIX 默认不验证上游 TLS 证书。

---

## 30. 限流乘数（limit-multiplier）

```yaml
# 输入
nginx.ingress.kubernetes.io/limit-rps: "100"
nginx.ingress.kubernetes.io/limit-multiplier: "2.5"
```

```yaml
# 输出（rate = 100 * 2.5 = 250）
plugins:
  - name: limit-req
    enable: true
    config:
      rate: 250
      burst: 0
      key: remote_addr
      rejected_code: 429
```

> `limit-multiplier` 影响所有限流注解（`limit-rps`、`limit-rpm`）。结果为整数时输出整数，有小数时输出浮点数。

---

## 附注

### 路径类型自动修正

转换器会自动调整 `pathType`：

| 原始 pathType | 路径含正则 | 转换后 pathType | 是否添加 use-regex |
| --- | --- | --- | --- |
| ImplementationSpecific | 否 | Prefix | 否 |
| Prefix | 是 | ImplementationSpecific | 是 |
| ImplementationSpecific | 是 | 保持原样 | 是 |
| Exact | 否 | 保持原样 | 否 |

### ingressClassName 自动修正

```yaml
# 输入
spec:
  ingressClassName: nginx
# 或 annotations:
#   kubernetes.io/ingress.class: nginx

# 输出
spec:
  ingressClassName: apisix
```

### 命名规则

- `ApisixPluginConfig` 名称：`{ingress-name}-plugins`（最长 64 字符）
- `BackendTrafficPolicy` 名称：`{ingress-name}-cookie-affinity` 或 `{ingress-name}-hash-by`（最长 63 字符）
- `ApisixUpstream` 名称：`{ingress-name}-upstream`（最长 63 字符）
- `ApisixTls` 名称：`{ingress-name}-tls`（或 `-tls-1`、`-tls-2` 等，最长 63 字符）
