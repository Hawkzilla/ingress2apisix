# APISIX Ingress Controller 注解参考

> 来源: [Apache APISIX 官方文档](https://apisix.apache.org/docs/ingress-controller/reference/apisix-ingress-controller/annotation/)

## 注解总览

| 注解 | 说明 |
|---|---|
| `kubernetes.io/ingress.class` | 指定哪个 Ingress Controller 处理该资源 |
| `k8s.apisix.apache.org/use-regex` | 启用正则路径匹配 |
| `k8s.apisix.apache.org/enable-websocket` | 启用 WebSocket 支持 |
| `k8s.apisix.apache.org/plugin-config-name` | 引用 ApisixPluginConfig |
| `k8s.apisix.apache.org/upstream-scheme` | 上游协议 (http/https/grpc/grpcs) |
| `k8s.apisix.apache.org/upstream-retries` | 上游重试次数 |
| `k8s.apisix.apache.org/upstream-connect-timeout` | 上游连接超时 |
| `k8s.apisix.apache.org/upstream-read-timeout` | 上游读取超时 |
| `k8s.apisix.apache.org/upstream-send-timeout` | 上游发送超时 |
| `k8s.apisix.apache.org/enable-cors` | 启用 CORS |
| `k8s.apisix.apache.org/cors-allow-origin` | CORS 允许来源 |
| `k8s.apisix.apache.org/cors-allow-headers` | CORS 允许请求头 |
| `k8s.apisix.apache.org/cors-allow-methods` | CORS 允许方法 |
| `k8s.apisix.apache.org/enable-csrf` | 启用 CSRF 保护 |
| `k8s.apisix.apache.org/csrf-key` | CSRF 签名密钥 |
| `k8s.apisix.apache.org/http-to-https` | HTTP 跳转 HTTPS |
| `k8s.apisix.apache.org/http-redirect` | 自定义重定向 URL |
| `k8s.apisix.apache.org/http-redirect-code` | 重定向状态码 |
| `k8s.apisix.apache.org/rewrite-target` | 路径重写目标 |
| `k8s.apisix.apache.org/rewrite-target-regex` | 路径重写正则 |
| `k8s.apisix.apache.org/rewrite-target-regex-template` | 路径重写模板 |
| `k8s.apisix.apache.org/enable-response-rewrite` | 启用响应重写 |
| `k8s.apisix.apache.org/response-rewrite-status-code` | 响应状态码 |
| `k8s.apisix.apache.org/response-rewrite-body` | 响应体内容 |
| `k8s.apisix.apache.org/response-rewrite-body-base64` | Base64 响应体 |
| `k8s.apisix.apache.org/response-rewrite-add-header` | 添加响应头 |
| `k8s.apisix.apache.org/response-rewrite-set-header` | 设置响应头 |
| `k8s.apisix.apache.org/response-rewrite-remove-header` | 移除响应头 |
| `k8s.apisix.apache.org/auth-uri` | 外部认证地址 |
| `k8s.apisix.apache.org/auth-ssl-verify` | 认证 SSL 验证 |
| `k8s.apisix.apache.org/auth-request-headers` | 转发给认证服务的请求头 |
| `k8s.apisix.apache.org/auth-upstream-headers` | 认证响应转发给上游的头 |
| `k8s.apisix.apache.org/auth-client-headers` | 认证响应返回给客户端的头 |
| `k8s.apisix.apache.org/allowlist-source-range` | IP 白名单 |
| `k8s.apisix.apache.org/blocklist-source-range` | IP 黑名单 |
| `k8s.apisix.apache.org/http-allow-methods` | 允许的 HTTP 方法 |
| `k8s.apisix.apache.org/http-block-methods` | 禁止的 HTTP 方法 |
| `k8s.apisix.apache.org/auth-type` | 认证类型 (keyAuth/basicAuth) |
| `k8s.apisix.apache.org/svc-namespace` | 跨命名空间服务访问 |

---

## 详细说明

### Ingress Class

`kubernetes.io/ingress.class` 注解指定由哪个 Ingress Controller 处理该 Ingress 资源。当集群中部署了多个 Ingress Controller 时非常有用。

```yaml
annotations:
  kubernetes.io/ingress.class: "apisix"
```

### 重定向 (Redirect)

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/http-to-https` | 设为 `"true"` 自动将 HTTP 重定向到 HTTPS |
| `k8s.apisix.apache.org/http-redirect` | 指定自定义重定向 URL |
| `k8s.apisix.apache.org/http-redirect-code` | 重定向 HTTP 状态码 |

```yaml
annotations:
  k8s.apisix.apache.org/http-to-https: "true"
# 或
annotations:
  k8s.apisix.apache.org/http-redirect: "https://example.com/new-path"
  k8s.apisix.apache.org/http-redirect-code: "302"
```

### 路径重写 (Proxy Rewrite)

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/rewrite-target` | 将请求路径重写为指定目标 |
| `k8s.apisix.apache.org/rewrite-target-regex` | 路径匹配正则 |
| `k8s.apisix.apache.org/rewrite-target-regex-template` | 重写模板（支持捕获组） |

简单重写：

```yaml
annotations:
  k8s.apisix.apache.org/rewrite-target: "/new-path"
```

正则重写：

```yaml
annotations:
  k8s.apisix.apache.org/rewrite-target-regex: "^/api/(.*)"
  k8s.apisix.apache.org/rewrite-target-regex-template: "/backend/$1"
```

### 正则路由 (RegEx Route Matching)

`k8s.apisix.apache.org/use-regex` 设为 `"true"` 后，`path` 字段按正则表达式匹配。

```yaml
annotations:
  k8s.apisix.apache.org/use-regex: "true"
spec:
  rules:
  - http:
      paths:
      - path: "/users/[0-9]+"
        pathType: ImplementationSpecific
```

### CORS 跨域

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/enable-cors` | 启用 CORS |
| `k8s.apisix.apache.org/cors-allow-origin` | 允许的来源 |
| `k8s.apisix.apache.org/cors-allow-headers` | 允许的请求头（逗号分隔） |
| `k8s.apisix.apache.org/cors-allow-methods` | 允许的 HTTP 方法（逗号分隔） |

```yaml
annotations:
  k8s.apisix.apache.org/enable-cors: "true"
  k8s.apisix.apache.org/cors-allow-origin: "https://example.com"
  k8s.apisix.apache.org/cors-allow-headers: "Content-Type,Authorization"
  k8s.apisix.apache.org/cors-allow-methods: "GET,POST,PUT"
```

### 上游超时 (Upstream)

| 注解 | 说明 | 默认值 |
|---|---|---|
| `k8s.apisix.apache.org/upstream-scheme` | 上游协议 | `http` |
| `k8s.apisix.apache.org/upstream-retries` | 重试次数 | - |
| `k8s.apisix.apache.org/upstream-connect-timeout` | 连接超时 | `60s` |
| `k8s.apisix.apache.org/upstream-read-timeout` | 读取超时 | `60s` |
| `k8s.apisix.apache.org/upstream-send-timeout` | 发送超时 | `60s` |

```yaml
annotations:
  k8s.apisix.apache.org/upstream-scheme: "https"
  k8s.apisix.apache.org/upstream-retries: "3"
  k8s.apisix.apache.org/upstream-connect-timeout: "5s"
  k8s.apisix.apache.org/upstream-read-timeout: "5s"
  k8s.apisix.apache.org/upstream-send-timeout: "5s"
```

### IP 访问控制 (IP Restriction)

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/allowlist-source-range` | 允许的 CIDR 列表（逗号分隔） |
| `k8s.apisix.apache.org/blocklist-source-range` | 禁止的 CIDR 列表（逗号分隔） |

```yaml
annotations:
  k8s.apisix.apache.org/allowlist-source-range: "10.0.0.0/24,192.168.1.0/24"
```

### 外部认证 (Forward Auth)

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/auth-uri` | 外部认证服务地址 |
| `k8s.apisix.apache.org/auth-ssl-verify` | 是否验证认证服务 SSL |
| `k8s.apisix.apache.org/auth-request-headers` | 转发给认证服务的请求头 |
| `k8s.apisix.apache.org/auth-upstream-headers` | 认证通过后转发给上游的响应头 |
| `k8s.apisix.apache.org/auth-client-headers` | 认证通过后返回给客户端的响应头 |

```yaml
annotations:
  k8s.apisix.apache.org/auth-uri: "https://auth.example.com/verify"
  k8s.apisix.apache.org/auth-ssl-verify: "true"
  k8s.apisix.apache.org/auth-request-headers: "Authorization,User-Agent,cookie"
  k8s.apisix.apache.org/auth-upstream-headers: "X-User-ID,X-User-Role"
  k8s.apisix.apache.org/auth-client-headers: "X-Auth-Status"
```

### 认证 (Authentication)

`k8s.apisix.apache.org/auth-type` 支持 `keyAuth` 和 `basicAuth`。

```yaml
annotations:
  k8s.apisix.apache.org/auth-type: "keyAuth"
# 需要配合 ApisixConsumer 使用
---
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata:
  name: john
spec:
  ingressClassName: apisix
  authParameter:
    keyAuth:
      value:
        key: john-key
```

### CSRF 保护

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/enable-csrf` | 启用 CSRF 保护 |
| `k8s.apisix.apache.org/csrf-key` | CSRF 签名密钥 |

```yaml
annotations:
  k8s.apisix.apache.org/enable-csrf: "true"
  k8s.apisix.apache.org/csrf-key: "my-secret-csrf-key"
```

### WebSocket

```yaml
annotations:
  k8s.apisix.apache.org/enable-websocket: "true"
```

### HTTP 方法控制

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/http-allow-methods` | 允许的 HTTP 方法（逗号分隔） |
| `k8s.apisix.apache.org/http-block-methods` | 禁止的 HTTP 方法（逗号分隔） |

```yaml
annotations:
  k8s.apisix.apache.org/http-allow-methods: "GET,POST"
```

### 响应重写 (Response Rewrite)

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/enable-response-rewrite` | 启用响应重写 |
| `k8s.apisix.apache.org/response-rewrite-status-code` | 新 HTTP 状态码 |
| `k8s.apisix.apache.org/response-rewrite-body` | 响应体内容 |
| `k8s.apisix.apache.org/response-rewrite-body-base64` | Base64 编码响应体 |
| `k8s.apisix.apache.org/response-rewrite-add-header` | 添加响应头 |
| `k8s.apisix.apache.org/response-rewrite-set-header` | 设置/覆盖响应头 |
| `k8s.apisix.apache.org/response-rewrite-remove-header` | 移除响应头 |

```yaml
annotations:
  k8s.apisix.apache.org/enable-response-rewrite: "true"
  k8s.apisix.apache.org/response-rewrite-status-code: "403"
  k8s.apisix.apache.org/response-rewrite-body: "Access denied"
  k8s.apisix.apache.org/response-rewrite-add-header: "X-Reason:Forbidden,X-Env:Test"
  k8s.apisix.apache.org/response-rewrite-remove-header: "header1,header2"
```

### 插件配置引用 (Plugin Config)

```yaml
annotations:
  k8s.apisix.apache.org/plugin-config-name: "rate-limit-config"
---
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: rate-limit-config
spec:
  ingressClassName: apisix
  plugins:
  - name: limit-count
    enable: true
    config:
      count: 2
      time_window: 10
      rejected_code: 429
```

### 跨命名空间服务 (Cross Namespace)

```yaml
annotations:
  k8s.apisix.apache.org/svc-namespace: "other-namespace"
```
