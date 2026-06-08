## NGINX Ingress 自动转换注解明细

本文档列出 ingress2apisix 工具能够自动转换的所有 NGINX Ingress 注解。共计 **15** 个分类、**58** 个注解可自动转换，无需手动干预。

### 转换类型说明


| 转换类型                     | 说明                                                                |
| ------------------------ | ----------------------------------------------------------------- |
| **Native Annotation**    | 转换为 `k8s.apisix.apache.org/`* 原生注解，直接应用于 APISIX Ingress 资源        |
| **PluginConfig**         | 生成 `ApisixPluginConfig` CRD，通过 `plugin-config-name` 注解关联到 Ingress |
| **BackendTrafficPolicy** | 生成 `BackendTrafficPolicy` CRD（负载均衡、健康检查等）                         |
| **Custom Plugin**        | 通过自定义 APISIX Lua 插件实现，需提前部署对应插件                                   |


---

### CORS


| NGINX Ingress 注解         | APISIX 等价配置                                | 转换类型              |
| ------------------------ | ------------------------------------------ | ----------------- |
| `enable-cors`            | `k8s.apisix.apache.org/enable-cors`        | Native Annotation |
| `cors-allow-origin`      | `k8s.apisix.apache.org/cors-allow-origin`  | Native Annotation |
| `cors-allow-methods`     | `k8s.apisix.apache.org/cors-allow-methods` | Native Annotation |
| `cors-allow-headers`     | `k8s.apisix.apache.org/cors-allow-headers` | Native Annotation |
| `cors-allow-credentials` | cors 插件 `allow_credential`                 | PluginConfig      |
| `cors-max-age`           | cors 插件 `max_age`                          | PluginConfig      |


### SSL/HTTPS 重定向


| NGINX Ingress 注解                            | APISIX 等价配置                                            | 转换类型              |
| ------------------------------------------- | ------------------------------------------------------ | ----------------- |
| `ssl-redirect`                              | `k8s.apisix.apache.org/http-to-https`                  | Native Annotation |
| `force-ssl-redirect`                        | `k8s.apisix.apache.org/http-to-https`                  | Native Annotation |
| `proxy-redirect-from` + `proxy-redirect-to` | `k8s.apisix.apache.org/http-to-https`（仅 HTTP→HTTPS 场景） | Native Annotation |


### URL 重写


| NGINX Ingress 注解        | APISIX 等价配置                                                     | 转换类型                             |
| ----------------------- | --------------------------------------------------------------- | -------------------------------- |
| `rewrite-target`        | `k8s.apisix.apache.org/rewrite-target` 或 `rewrite-target-regex` | Native Annotation                |
| `configuration-snippet` | 单条 rewrite → 原生注解；多条 → proxy-rewrite 插件                         | Native Annotation / PluginConfig |
| `upstream-vhost`        | proxy-rewrite 插件 `host`                                         | PluginConfig                     |


### 上游超时


| NGINX Ingress 注解        | APISIX 等价配置                                      | 转换类型              |
| ----------------------- | ------------------------------------------------ | ----------------- |
| `proxy-connect-timeout` | `k8s.apisix.apache.org/upstream-connect-timeout` | Native Annotation |
| `proxy-send-timeout`    | `k8s.apisix.apache.org/upstream-send-timeout`    | Native Annotation |
| `proxy-read-timeout`    | `k8s.apisix.apache.org/upstream-read-timeout`    | Native Annotation |
| `backend-protocol`      | `k8s.apisix.apache.org/upstream-scheme`          | Native Annotation |


### 访问控制


| NGINX Ingress 注解         | APISIX 等价配置                                    | 转换类型              |
| ------------------------ | ---------------------------------------------- | ----------------- |
| `whitelist-source-range` | `k8s.apisix.apache.org/allowlist-source-range` | Native Annotation |
| `denylist-source-range`  | `k8s.apisix.apache.org/blocklist-source-range` | Native Annotation |


### 认证


| NGINX Ingress 注解        | APISIX 等价配置                                                        | 转换类型              |
| ----------------------- | ------------------------------------------------------------------ | ----------------- |
| `auth-url`              | `k8s.apisix.apache.org/auth-uri`                                   | Native Annotation |
| `auth-method`           | `k8s.apisix.apache.org/auth-method`                                | Native Annotation |
| `auth-request-headers`  | `k8s.apisix.apache.org/auth-request-headers`                       | Native Annotation |
| `auth-response-headers` | `k8s.apisix.apache.org/auth-upstream-headers`                      | Native Annotation |
| `auth-signin`           | `k8s.apisix.apache.org/auth-signin`                                | Native Annotation |
| `auth-type`             | `k8s.apisix.apache.org/auth-type`（basic→basicAuth, digest→keyAuth） | Native Annotation |
| `auth-realm`            | `k8s.apisix.apache.org/auth-realm`（forward-auth WWW-Authenticate）  | Native Annotation |
| `auth-secret`           | `k8s.apisix.apache.org/auth-secret`（需创建 ApisixConsumer CRD）        | Native Annotation |


### WebSocket


| NGINX Ingress 注解     | APISIX 等价配置                              | 转换类型              |
| -------------------- | ---------------------------------------- | ----------------- |
| `websocket-services` | `k8s.apisix.apache.org/enable-websocket` | Native Annotation |


### 正则路由


| NGINX Ingress 注解 | APISIX 等价配置                       | 转换类型              |
| ---------------- | --------------------------------- | ----------------- |
| `use-regex`      | `k8s.apisix.apache.org/use-regex` | Native Annotation |


### 会话亲和


| NGINX Ingress 注解                           | APISIX 等价配置                             | 转换类型                   |
| ------------------------------------------ | --------------------------------------- | ---------------------- |
| `session-cookie-hash`                      | 自定义 session-cookie-hash 插件              | Custom Plugin          |
| `session-cookie-expires`                   | 扩展 session-cookie-hash 插件 `max_age`     | Custom Plugin          |
| `session-cookie-max-age`                   | 扩展 session-cookie-hash 插件 `max_age`     | Custom Plugin          |
| `session-cookie-path`                      | 扩展 session-cookie-hash 插件 `cookie_path` | Custom Plugin          |
| `session-cookie-conditional-samesite-none` | 发出警告（APISIX 无等价参数）                      | Custom Plugin（warning） |


### 速率限制


| NGINX Ingress 注解    | APISIX 等价配置                | 转换类型         |
| ------------------- | -------------------------- | ------------ |
| `limit-rps`         | limit-req 插件               | PluginConfig |
| `limit-rpm`         | limit-req 插件（rate: N/min）  | PluginConfig |
| `limit-connections` | limit-conn 插件              | PluginConfig |
| `limit-multiplier`  | 自动乘算 limit-rps/limit-rpm 值 | PluginConfig |


### 请求体大小


| NGINX Ingress 注解  | APISIX 等价配置                       | 转换类型         |
| ----------------- | --------------------------------- | ------------ |
| `proxy-body-size` | client-control 插件 `max_body_size` | PluginConfig |


### Cookie 重写


| NGINX Ingress 注解    | APISIX 等价配置              | 转换类型          |
| ------------------- | ------------------------ | ------------- |
| `proxy-cookie-path` | 自定义 proxy-cookie-path 插件 | Custom Plugin |


### 真实 IP


| NGINX Ingress 注解             | APISIX 等价配置                   | 转换类型         |
| ---------------------------- | ----------------------------- | ------------ |
| `enable-real-ip`             | real-ip 插件                    | PluginConfig |
| `use-forwarded-headers`      | 配合 real-ip 插件（recursive=true） | PluginConfig |
| `compute-full-forwarded-for` | 配合 real-ip 插件（append=true）    | PluginConfig |
| `forwarded-for-header`       | 配合 real-ip 插件（source 配置）      | PluginConfig |


### SSL 验证


| NGINX Ingress 注解 | APISIX 等价配置           | 转换类型            |
| ---------------- | --------------------- | --------------- |
| `ssl-verify`     | ApisixUpstream TLS 配置 | Warning（manual） |


### 访问日志


| NGINX Ingress 注解    | APISIX 等价配置                               | 转换类型              |
| ------------------- | ----------------------------------------- | ----------------- |
| `enable-access-log` | `k8s.apisix.apache.org/enable-access-log` | Native Annotation |


### 负载均衡


| NGINX Ingress 注解   | APISIX 等价配置                 | 转换类型                 |
| ------------------ | --------------------------- | -------------------- |
| `upstream-hash-by` | BackendTrafficPolicy（chash） | BackendTrafficPolicy |


### 健康检查


| NGINX Ingress 注解        | APISIX 等价配置                                                | 转换类型                 |
| ----------------------- | ---------------------------------------------------------- | -------------------- |
| `health-check-interval` | BackendTrafficPolicy（healthCheck.active.interval）          | BackendTrafficPolicy |
| `health-check-path`     | BackendTrafficPolicy（healthCheck.active.httpPath）          | BackendTrafficPolicy |
| `health-check-retries`  | BackendTrafficPolicy（healthCheck.active.healthy.successes） | BackendTrafficPolicy |
| `health-check-timeout`  | BackendTrafficPolicy（healthCheck.active.timeout）           | BackendTrafficPolicy |


### HTTP 重定向


| NGINX Ingress 注解     | APISIX 等价配置                                      | 转换类型              |
| -------------------- | ------------------------------------------------ | ----------------- |
| `permanent-redirect` | `k8s.apisix.apache.org/http-redirect` + code 308 | Native Annotation |
| `temporal-redirect`  | `k8s.apisix.apache.org/http-redirect` + code 302 | Native Annotation |
| `app-root`           | `k8s.apisix.apache.org/http-redirect`            | Native Annotation |


### 自定义错误页面


| NGINX Ingress 注解     | APISIX 等价配置                                                      | 转换类型              |
| -------------------- | ---------------------------------------------------------------- | ----------------- |
| `custom-http-errors` | `k8s.apisix.apache.org/custom-error-codes`（custom-error-page 插件） | Native Annotation |


---

### 统计汇总


| 转换类型                 | 注解数量   |
| -------------------- | ------ |
| Native Annotation    | 30     |
| PluginConfig         | 14     |
| BackendTrafficPolicy | 5      |
| Custom Plugin        | 5      |
| Warning（manual）      | 1      |
| 部分场景可自动（组合）          | 3      |
| **合计**               | **58** |


