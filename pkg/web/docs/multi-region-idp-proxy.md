# multi-region-idp-proxy 插件使用说明

## 涉及的 Path 列表

以下 path 需要启用多区域 IDP 代理（从旧方案 `MULTI_REGION_GLOBAL_PATH` 迁移）：

```
/ems_dashboard_api/api/oem/query
/ems_dashboard_api/auth
/ems_dashboard_api/auth_login
/ems_dashboard_api/register
/ems_dashboard_api/forgot_password
/ems_dashboard_api/api/security/password_strength
/ems_dashboard_api/captcha
/ems_dashboard_api/api/language
/ems_dashboard_api/api/user_session
/ems_dashboard_api/api/service_providers/tokens
/ems_dashboard_api/api/kubeapps/ota_release_status/multi-region
/auth/logout
/auth_login
/ems_dashboard_api/api/platform/identity
/ems_dashboard_api/api/menu/left_nav/multi-region
/ems_dashboard_api/api/menu/left_nav/iam
/ems_dashboard_api/api/componententity
/ems_dashboard_api/api/register/invite_emails
/ecs/api/register/invite_emails
/ecs/api/keystone/projects
/ecs/api/security/password_strength
/ems_dashboard_api/switch_cloud
```

> 迁移时需要将这些 path 放入带 `k8s.apisix.apache.org/plugin-config-name` 注解的 Ingress 中，其余 path 保留在不带注解的 Ingress 中。

## 概述

`multi-region-idp-proxy` 插件用于多区域 IDP（Identity Provider）/ SP（Service Provider）部署场景。它将原始 nginx 的 `configuration-snippet` 中的条件 Cookie 处理和动态 `proxy_pass` 逻辑，转换为 APISIX 自定义 Lua 插件。

## 从旧方案迁移

### 旧方案（已废弃）

旧方案通过修改 nginx-ingress controller 的 Deployment 环境变量和模板来注入多区域逻辑：

**1. Deployment 环境变量**

```yaml
- name: MULTI_REGION_GLOBAL_NAMESPACE
  value: multi-region,iam,
- name: MULTI_REGION_GLOBAL_INGRESS
- name: MULTI_REGION_GLOBAL_SERVICE
- name: MULTI_REGION_GLOBAL_PATH
  value: /ems_dashboard_api/api/oem/query,/ems_dashboard_api/auth,/ems_dashboard_api/auth_login,...
- name: MULTI_REGION_PROXY_PASS
  value: |
    # MULITI REGION SP CHANGE COOKIES
    if ($http_cookie ~* "region_label=fromidp") { ... }
    # MULITI REGION IDP PROXY_PASS COOKIES
    if ($http_cookie ~* "region_url=(.*)") { ... }
```

**2. nginx-ingress 模板注入逻辑**

```
{{ $global_label := "" }}
{{ range $global_path := split (getenv "MULTI_REGION_GLOBAL_NAMESPACE") "," }}
  {{ if eq $global_path $ing.Namespace }}
    {{ $global_label = "GLOBAL NAMESPACE" }}
  {{ end }}
{{ end }}
{{ range $global_path := split (getenv "MULTI_REGION_GLOBAL_PATH") "," }}
  {{ if eq $global_path ( $ing.Path | escapeLiteralDollar ) }}
    {{ $global_label = "GLOBAL PATH" }}
  {{ end }}
{{ end }}
```

**旧方案的问题：**

| 问题 | 说明 |
|------|------|
| 侵入性强 | 需要修改 nginx-ingress controller 的 Deployment YAML 和模板文件 |
| 维护困难 | 环境变量列表随着 path 增多越来越长，难以管理 |
| 升级风险 | nginx-ingress controller 升级时需要同步维护模板改动 |
| 粒度粗糙 | 只能按 namespace / ingress / service / path 级别控制，无法灵活组合 |
| 全局影响 | 修改 Deployment 会导致 nginx-ingress controller 重启，影响所有 Ingress |
| 不可控 | 环境变量是集群级别的，无法让单个团队自行决定是否启用 |

### 新方案（推荐）

新方案通过 APISIX 插件 + `ApisixPluginConfig` + Ingress 注解实现，**由使用方决定在哪些 Ingress 上启用**：

```yaml
# 1. 创建 PluginConfig（一次性，由集群管理员或团队创建）
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: idp-proxy-config
  namespace: ems
spec:
  ingressClassName: apisix
  plugins:
    - name: multi-region-idp-proxy
      enable: true
      config:
        proxy_scheme: "https"
        proxy_port: 443
        allowed_region_hosts:
          - "10.0.0.12"
          - "region-a.example.com"

# 2. 在需要的 Ingress 上加注解（使用方自行决定）
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ems-dashboard
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
spec:
  rules:
    - host: ems.example.com
      http:
        paths:
          - path: /ems_dashboard_api
            pathType: Prefix
            backend:
              service:
                name: ems-svc
                port:
                  number: 80
```

**新方案的优势：**

| 对比项 | 旧方案 | 新方案 |
|--------|--------|--------|
| 控制方式 | 环境变量（集群级别） | Ingress 注解（Ingress 级别） |
| 控制权 | 集群管理员修改 Deployment | 使用方自行加注解 |
| 粒度 | namespace / ingress / service / path | 单个 Ingress |
| 升级影响 | 修改 Deployment 会导致 controller 重启 | 仅影响加了注解的 Ingress |
| 可审计 | 需要查看 Deployment 环境变量 | `kubectl get ingress -A -o yaml | grep plugin-config-name` |
| 安全性 | 无白名单 | 内置 `allowed_region_hosts` 白名单 |

### 迁移对照表

| 旧方案（环境变量） | 新方案（APISIX 插件） |
|---|---|
| `MULTI_REGION_GLOBAL_NAMESPACE=multi-region,iam` | 在对应 namespace 的 Ingress 上加注解 |
| `MULTI_REGION_GLOBAL_INGRESS=xxx` | 在指定 Ingress 上加注解 |
| `MULTI_REGION_GLOBAL_SERVICE=xxx` | 在对应服务的 Ingress 上加注解 |
| `MULTI_REGION_GLOBAL_PATH=/ems_dashboard_api/auth,...` | 按 path 拆分 Ingress，需要的挂注解，不需要的不挂 |
| `MULTI_REGION_PROXY_PASS=...`（nginx snippet） | `multi-region-idp-proxy` 插件自动处理 |
| 修改 Deployment 模板 | `kubectl apply` PluginConfig + 注解 |

## 多 path 场景处理

一个 Ingress 可能包含多个 path，每个 path 对应不同的后端服务。`ApisixPluginConfig` 是挂在整个 Ingress 上的，**所有 path 都会加载插件**。

插件**只有在 Cookie 匹配时才会触发改写**，不影响其他 path 的正常请求。但需要注意：

> **分支 A 的 Set-Cookie 清理（header_filter）会作用于所有 path 的响应。** 如果某个 path 的后端返回了 `sessionid` 或 `csrftoken` 的 Set-Cookie，也会被删除。

因此，如果只有部分 path 需要 IDP 代理，**必须拆分 Ingress**。

### 场景 A：所有 path 都需要 IDP 代理

直接在 Ingress 上挂 PluginConfig，所有 path 生效：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /dashboard
            pathType: Prefix
            backend:
              service:
                name: dashboard-svc
                port:
                  number: 80
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-svc
                port:
                  number: 8080
```

### 场景 B：只有部分 path 需要 IDP 代理（拆分 Ingress）

将需要 IDP 代理的 path 拆分到独立的 Ingress：

```yaml
# Ingress 1：需要 IDP 代理的 path（挂 PluginConfig）
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-idp-paths
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /dashboard
            pathType: Prefix
            backend:
              service:
                name: dashboard-svc
                port:
                  number: 80
          - path: /auth
            pathType: Prefix
            backend:
              service:
                name: auth-svc
                port:
                  number: 80
---
# Ingress 2：不需要 IDP 代理的 path（不挂 PluginConfig）
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-api-paths
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-svc
                port:
                  number: 8080
```

### 场景 C：从旧方案迁移 path 列表

旧方案通过 `MULTI_REGION_GLOBAL_PATH` 环境变量控制哪些 path 注入逻辑：

```yaml
# 旧方案的 path 列表
MULTI_REGION_GLOBAL_PATH: /ems_dashboard_api/api/oem/query,/ems_dashboard_api/auth,/ems_dashboard_api/auth_login,/ems_dashboard_api/register,...
```

迁移步骤：

1. **识别需要 IDP 代理的 path 列表**
2. **将这些 path 放入带 PluginConfig 注解的 Ingress**
3. **其余 path 保留在不带注解的 Ingress 中**

```yaml
# 需要 IDP 代理的 path
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ems-multi-region-paths
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
spec:
  rules:
    - host: ems.example.com
      http:
        paths:
          - path: /ems_dashboard_api/auth
            pathType: Exact
            backend:
              service:
                name: ems-svc
                port:
                  number: 80
          - path: /ems_dashboard_api/auth_login
            pathType: Exact
            backend:
              service:
                name: ems-svc
                port:
                  number: 80
          - path: /auth/logout
            pathType: Exact
            backend:
              service:
                name: ems-svc
                port:
                  number: 80
          # ... 其他需要 IDP 代理的 path
---
# 不需要 IDP 代理的 path（兜底路由）
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ems-default-paths
spec:
  rules:
    - host: ems.example.com
      http:
        paths:
          - path: /ems_dashboard_api
            pathType: Prefix
            backend:
              service:
                name: ems-svc
                port:
                  number: 80
```

### 场景 D：多个 host，部分 host 需要 IDP 代理

按 host 拆分 Ingress，各自独立挂载：

```yaml
# Ingress 1：需要 IDP 代理的 host
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: region-a-app
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
spec:
  rules:
    - host: region-a.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app-svc
                port:
                  number: 80
---
# Ingress 2：不需要 IDP 代理的 host
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-app
spec:
  rules:
    - host: public.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app-svc
                port:
                  number: 80
```

### 拆分注意事项

| 注意点 | 说明 |
|--------|------|
| Path 匹配类型 | 拆分时注意 `Prefix` / `Exact` / `ImplementationSpecific` 的区别，避免路由冲突 |
| 同 host 多 Ingress | APISIX Ingress Controller 会自动合并同 host 的多个 Ingress 为一个 APISIX Route |
| 兜底路由 | 不需要 IDP 代理的 path 可以用一个不带注解的 Ingress 兜底（如 `path: /`） |
| Set-Cookie 影响 | 分支 A 的 Set-Cookie 清理会影响挂载了 PluginConfig 的所有 path 的响应 |

## 插件功能

### 分支 A：IDP 登录回调（region_label=fromidp）

**触发条件**：请求 Cookie 中包含 `region_label=fromidp`

**行为**：

| 步骤 | 说明 |
|------|------|
| Cookie 改写 | 将 sp_* 系列 Cookie 翻译为标准名称发送给后端 |
| X-Csrftoken | 从 sp_csrftoken 取值设到请求头 |
| Set-Cookie 清理 | 响应中删除以 sessionid 和 csrftoken 开头的 Set-Cookie，防止覆盖客户端已有 Session |

**Cookie 映射**：

| 原始 Cookie（请求中） | 改写后 Cookie（发给后端） |
|----------------------|------------------------|
| sp_sessionid=xxx | sessionid=xxx |
| sp_escookie=xxx | escookie=xxx |
| sp_csrftoken=xxx | csrftoken=xxx |
| sp_ems_dashboard_api_language=xxx | ems_dashboard_api_language=xxx |
| region_label=fromidp | 删除 |
| 其他 Cookie | 删除 |

**示例**：

```
请求 Cookie: region_label=fromidp;sp_sessionid=abc;sp_escookie=def;sp_csrftoken=ghi
改写后:      sessionid=abc;escookie=def;csrftoken=ghi
请求头:      X-Csrftoken: ghi
```

### 分支 B：动态路由到目标区域（region_url=host）

**触发条件**：请求 Cookie 中包含 `region_url=<host>`

**行为**：

| 步骤 | 说明 |
|------|------|
| Cookie 拼接 | 将本地 + 跨区域 Cookie 合并发给目标区域后端 |
| 追加标记 | Cookie 尾部追加 region_label=fromidp，让目标区域识别这是 IDP 回调 |
| 动态路由 | 请求转发到 region_url 指定的主机 |
| 白名单校验 | 目标地址必须在 allowed_region_hosts 中，否则拒绝路由 |

**Cookie 拼接**：

| Cookie 字段 | 来源 |
|-------------|------|
| sessionid | 本地 |
| escookie | 本地 |
| csrftoken | 本地 |
| sp_sessionid | 跨区域 |
| sp_escookie | 跨区域 |
| sp_csrftoken | 跨区域 |
| sp_ems_dashboard_api_language | 从 sp_http_language Cookie 读取 |
| region_label=fromidp | 固定值，标记为 IDP 回调 |

**示例**：

```
请求 Cookie: region_url=10.0.0.12;sessionid=s1;escookie=e1;csrftoken=c1;sp_sessionid=sp1;sp_escookie=ep1;sp_csrftoken=cp1;sp_http_language=zh
改写后:      sessionid=s1;escookie=e1;csrftoken=c1;sp_sessionid=sp1;sp_escookie=ep1;sp_csrftoken=cp1;sp_ems_dashboard_api_language=zh;region_label=fromidp
路由:        https://10.0.0.12:443
```

### 分支优先级

两个分支互斥，region_label=fromidp 优先匹配。如果两个条件同时存在，只走分支 A。

## 配置说明

| 配置项 | 类型 | 默认值 | 必填 | 说明 |
|--------|------|--------|------|------|
| proxy_scheme | string | https | 否 | 动态路由的协议。http 或 https |
| proxy_port | integer | 443 | 否 | 动态路由的端口。Cookie 中 region_url 未带端口时自动补上 |
| allowed_region_hosts | array | 空（允许所有） | 否 | 白名单。只允许路由到列表中的地址，防止 Cookie 被篡改后请求被劫持 |

## 部署步骤

### 1. 部署插件文件

```bash
kubectl cp plugins/multi-region-idp-proxy.lua \
  <namespace>/<apisix-pod>:/usr/local/apisix/apisix/plugins/multi-region-idp-proxy.lua \
  -c apisix
```

### 2. 注册插件

在 APISIX 的 config.yaml 中添加：

```yaml
plugins:
  # ... 其他插件
  - multi-region-idp-proxy
```

### 3. 重启 APISIX

```bash
kubectl delete pod -n <namespace> <apisix-pod>
```

### 4. 创建 ApisixPluginConfig

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: idp-proxy-config
  namespace: <namespace>
spec:
  ingressClassName: apisix
  plugins:
    - name: multi-region-idp-proxy
      enable: true
      config:
        proxy_scheme: "https"
        proxy_port: 443
        allowed_region_hosts:
          - "10.0.0.12"
          - "region-a.example.com"
```

```bash
kubectl apply -f idp-proxy-config.yaml
```

### 5. 关联 Ingress

在目标 Ingress 上添加注解（由使用方自行决定哪些 Ingress 需要启用）：

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/plugin-config-name: idp-proxy-config
```

```bash
kubectl apply -f <ingress>.yaml
```

## 验证方法

### 分支 A：Cookie 改写

```bash
curl -s -H "Host: <域名>" \
  -b "region_label=fromidp;sp_sessionid=sp-sess-123;sp_escookie=sp-es-456;sp_csrftoken=sp-csrf-789;sp_ems_dashboard_api_language=zh" \
  http://<APISIX>/get
```

预期返回的 Cookie 头：
```json
{
  "headers": {
    "Cookie": "sessionid=sp-sess-123;escookie=sp-es-456;csrftoken=sp-csrf-789;ems_dashboard_api_language=zh"
  }
}
```

### 分支 A：Set-Cookie 过滤

```bash
curl -v -H "Host: <域名>" \
  -b "region_label=fromidp;sp_sessionid=sp-sess;sp_escookie=sp-es;sp_csrftoken=sp-csrf" \
  "http://<APISIX>/response-headers?Set-Cookie=sessionid%3Dnew-sess%3B%20Path%3D/&Set-Cookie=csrftoken%3Dnew-csrf%3B%20Path%3D/&Set-Cookie=other%3Dval"
```

预期响应头：只剩 Set-Cookie: other=val

### 分支 B：动态路由

```bash
curl -s -H "Host: <域名>" \
  -b "region_url=10.0.0.12;sessionid=sess-1;escookie=es-1;csrftoken=csrf-1;sp_sessionid=sp-sess-1;sp_escookie=sp-es-1;sp_csrftoken=sp-csrf-1;sp_http_language=zh" \
  http://<APISIX>/get
```

预期：请求被路由到 10.0.0.12:443（如果目标不可达，返回 502/504，说明路由生效）

### 分支 B：白名单拦截

```bash
curl -s -H "Host: <域名>" \
  -b "region_url=evil.com;sessionid=sess-1" \
  http://<APISIX>/get
```

预期：返回 200（走原始后端），APISIX 日志中有 "blocked region host: evil.com"

### 无触发条件：透传

```bash
curl -s -H "Host: <域名>" \
  -b "normal_cookie=value" \
  http://<APISIX>/get
```

预期：返回 200，Cookie 保持原样不变

## 排查

```bash
# 查看哪些 Ingress 启用了多区域代理
kubectl get ingress -A -o yaml | grep -B5 "idp-proxy-config"

# 查看 APISIX error log
kubectl logs -n <namespace> -l app.kubernetes.io/name=apisix --tail=50 | grep -i "multi-region"

# 确认插件文件存在
kubectl exec -n <namespace> <apisix-pod> -c apisix -- \
  ls -la /usr/local/apisix/apisix/plugins/multi-region-idp-proxy.lua

# 确认 PluginConfig
kubectl get apisixpluginconfig idp-proxy-config -n <namespace> -o yaml
```

## 注意事项

1. 分支 B 的 proxy_scheme 和 proxy_port 必须与目标区域后端的实际协议和端口一致，否则连接失败
2. 白名单是安全防护的关键配置，生产环境必须设置，防止 Cookie 被篡改后请求被劫持到任意地址
3. 分支优先级：region_label=fromidp 先于 region_url 匹配，两者同时存在时只走分支 A
4. 空值处理：如果 Cookie 字段值为空，不会出现在改写后的 Cookie 中
5. 该插件的 priority = 4100，在 session-cookie-hash（3990）和 proxy-cookie-flags（3995）之前执行
6. 多 path 场景下，如果只有部分 path 需要 IDP 代理，必须将需要的 path 拆分到独立 Ingress 并单独挂载 PluginConfig，避免 Set-Cookie 清理影响不需要的 path
