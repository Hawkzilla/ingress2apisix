# 1. 目的说明

本文档的目的是将 `ingress-nginx` 资源转换为 `APISIX Ingress` 资源，并尽量实现与原有配置等价的访问行为、路由语义和相关能力。

# 2. 限制

由于 `APISIX Ingress` 与 `ingress-nginx` 的实现机制存在一定差别，迁移时不能简单地将原有注解逐项替换。为了尽量追求功能一致，部分能力需要通过 `APISIX` 插件实现，部分能力则需要依赖 `APISIX` 特定的 `CRD` 资源实现。

## 2.1 需要通过插件实现的能力

### 2.1.1 404、500 自定义错误页或错误后端

`ingress-nginx` 中常见配置如下：

```yaml
metadata:
  annotations:
    ingress.kubernetes.io/custom-http-errors: 404,500
```

该类配置的目的是在上游返回特定状态码时，由网关接管错误响应，并转发到自定义错误页或错误服务。

在 `APISIX` 中，这类能力通常不能直接通过标准 `Ingress` 字段表达，需要借助插件能力实现。迁移时需要重点确认以下内容：

- 哪些状态码需要被接管，例如 `404`、`500`
- 是返回固定内容，还是转发到统一错误后端
- 是否需要保留原始响应头、响应体或请求上下文

因此，这部分应视为“需要额外插件适配”的能力，而不是原生 `Ingress` 字段的一对一替换。

### 2.1.2 Cookie 属性追加或改写

`ingress-nginx` 中常见配置如下：

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_cookie_flags sessionid SameSite=None Secure;
```

该类配置的目的是对指定 `cookie` 追加属性，例如：

- `SameSite=None`
- `Secure`

在 `ingress-nginx` 中，这通常通过嵌入 `NGINX` 指令片段完成；但在 `APISIX` 中，不建议继续依赖类似的底层配置片段，而需要通过插件方式实现。

迁移时需要明确以下信息：

- 目标 cookie 名称，例如 `sessionid`
- 需要追加的属性集合
- 是否只匹配特定 cookie
- 是否涉及正则匹配或多 cookie 处理逻辑

因此，这类 `cookie` 处理逻辑应归类为“需要插件补齐的行为”。

### 2.1.3 自定义多区域相关内容

部分原有配置中会包含明显的业务语义或区域语义，例如多区域、多站点、多入口等逻辑。这类内容通常并不属于标准 `Ingress` 路由能力本身，而是通过额外的网关控制逻辑实现。

在 `APISIX` 迁移中，这类能力通常需要结合插件、路由拆分、头部透传、条件匹配等方式实现，不能假设存在统一的原生字段直接映射。

## 2.2 需要通过 APISIX 特定 CRD 或插件组合实现的能力

### 2.2.1 会话亲和

在 `ingress-nginx` 中，会话亲和通常依赖如下配置：

- `ingress.kubernetes.io/affinity: cookie`
- `ingress.kubernetes.io/session-cookie-name`
- `ingress.kubernetes.io/session-cookie-hash`

在 `APISIX` 中，会话亲和主要通过 `BackendTrafficPolicy` 实现，而不是直接通过 `Ingress` 注解完成。

因此，会话亲和在迁移中应明确归类为“需要 `BackendTrafficPolicy` 参与实现”的能力。

### 2.2.2 限速

在 `ingress-nginx` 中，限速通常可以通过注解进行表达；在 `APISIX` 中，限速能力通常通过插件实现。

目前迁移时建议基于 `ApisixPluginConfig` 资源定义 `limit-req` 插件，并在 `Ingress` 上通过插件配置引用的方式挂载。

这种方式的优点是：

- 配置更加集中
- 可复用
- 与 `Ingress` 路由定义解耦

因此，限速能力应归类为“通过 `ApisixPluginConfig + limit-req` 实现”的能力。

### 2.2.3 认证

原有 `ingress-nginx` 中与认证相关的能力，通常依赖如下注解：

- `auth-url`
- `auth-response-headers`
- `auth-signin`
- 其他认证相关扩展注解

在 `APISIX` 中，这部分能力通常需要通过插件实现，例如基于统一的认证插件配置来实现外部认证、认证头透传和认证失败处理逻辑。

因此，认证能力在迁移中应归类为“通过插件实现”的能力，而不是简单的注解替换。

### 2.2.4 HTTPS + IP 访问

在 `ingress-nginx` 中，部分场景下即使未显式为每个域名单独建模，也可能表现为“通过 IP 访问 HTTPS 似乎可用”。但在 `APISIX` 中，TLS 证书选择更加依赖 `SNI`，因此该能力需要显式设计。

目前如需兼容 `HTTPS + IP` 访问，主要依赖以下能力组合：

- `ApisixTls`
- `.apisix.ssl.fallback_sni`

其中：

- `ApisixTls` 用于显式声明证书与主机名的绑定关系
- `.apisix.ssl.fallback_sni` 用于在缺失合适 `SNI` 时提供兜底匹配能力

需要注意的是，这部分能力本质上属于 `TLS` 和网关实例配置能力，不是普通 `Ingress` 字段本身就能完整表达的。

### 2.2.5 同一个 Ingress 中存在多个 rewrite 逻辑

在 `ingress-nginx` 中，常见做法是通过如下方式实现复杂重写逻辑：

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      rewrite ...
      rewrite ...
```

这类能力在 `APISIX` 中通常需要通过 `ApisixPluginConfig + proxy-rewrite` 组合实现。

如果一个 `Ingress` 中存在多条不同路径、不同 rewrite 规则，则迁移时通常需要考虑：

- 是否拆分为多个 `Ingress`
- 是否拆分为多个路由
- 是否拆分为多个插件配置

因此，这类能力应归类为“通过 `ApisixPluginConfig + proxy-rewrite` 实现”的能力。


## 2.3 已知的差异(nginx ingress vs apisix)

### 2.3.1 service解析差异

- 当ingress资源后端使用ExternalName svc时 需要注意 apisix不会去带端口解析 也就是 会将例如 ingress-error-pages.kube-system.svc.cluster.local + port 定义为最终端点 ingress-error-pages.kube-system.svc.cluster.local
  最终会获得DNS解析 得到ip 但如果pod和后端服务或者pod不一致 则可能不通




# 3. 平滑迁移

对于一部分语义较明确、且在 `APISIX Ingress` 中存在直接或近似等价能力的 `ingress-nginx` 注解，可以优先采用平滑迁移方式处理，即在不显著改变原有路由结构的前提下，将对应能力替换为 `APISIX` 的注解表达。

需要说明的是，平滑迁移并不代表二者底层实现完全一致，而是指：

- 原有访问入口尽量不变
- 原有路由规则尽量不变
- 原有行为尽量保持一致
- 优先使用 `Ingress` 注解完成迁移，减少额外资源拆分

对于无法仅通过注解完成迁移的场景，仍需回到第 2 节所述方案，使用插件或 `CRD` 资源补齐。

## 3.1 可平滑迁移的注解等价表

| ingress-nginx 注解                                                                                                | APISIX Ingress 注解                                                                                        | 说明                                                                                                           |
|-----------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| `ingress.kubernetes.io/force-ssl-redirect: "true"`                                                          | `k8s.apisix.apache.org/http-to-https: "true"`                                                            | 用于将 HTTP 请求重定向到 HTTPS。这是较常见、也较容易进行平滑迁移的能力。                                                                   |
| `nginx.ingress.kubernetes.io/ssl-redirect: "true"`                                                        | `k8s.apisix.apache.org/http-to-https: "true"`                                                            | 语义与强制 HTTPS 跳转一致，迁移时可统一收敛到 APISIX 的 `http-to-https` 注解。                                                      |
| `ingress.kubernetes.io/rewrite-target: /`                                                                  | `k8s.apisix.apache.org/rewrite-target: /`                                                                | 用于简单路径重写。当重写逻辑较简单时可以直接迁移；若涉及多段 rewrite 或复杂规则，应改用 `proxy-rewrite` 插件。                                         |
| `nginx.ingress.kubernetes.io/rewrite-target: /$2 以及复杂的带正则捕获的重写`                                          | `k8s.apisix.apache.org/rewrite-target-regex 以及 k8s.apisix.apache.org/rewrite-target-regex-template 配合使用` | 对包含正则分组的简单重写可近似迁移；需结合实际路径匹配规则验证效果。复杂重写不建议只依赖单注解。                                                             |
| `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"`                                                   | `k8s.apisix.apache.org/upstream-read-timeout: "3600s"`                                                   | 用于控制网关读取上游响应的超时时间。迁移时建议统一明确时间单位。                                                                             |
| `nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"`                                                  | `k8s.apisix.apache.org/upstream-send-timeout: "3600s"`                                                   | 用于控制向上游发送请求的超时时间。建议迁移时显式补齐时间单位。                                                                              |
| `nginx.ingress.kubernetes.io/proxy-connect-timeout: "3600"`                                          | `k8s.apisix.apache.org/upstream-connect-timeout: "3600s"`                                                | 用于控制连接上游的超时时间。迁移后需验证长连接或慢连接场景。                                                                               |
| `nginx.ingress.kubernetes.io/enable-cors: "true"`                                                         | `k8s.apisix.apache.org/enable-cors: "true"`                                                              | 用于启用跨域能力。若仅是启用开关，通常可平滑迁移。                                                                                    |
| `nginx.ingress.kubernetes.io/cors-allow-origin: "*"`                                                       | `k8s.apisix.apache.org/cors-allow-origin: "*"`                                                           | 用于设置允许的来源。若原配置包含更复杂的动态来源逻辑，仍需单独验证。                                                                           |
| `nginx.ingress.kubernetes.io/cors-allow-methods: "PUT, GET, POST, OPTIONS"`                               | `k8s.apisix.apache.org/cors-allow-methods: "PUT, GET, POST, OPTIONS"`                                    | 用于设置允许的方法，通常可直接迁移。                                                                                           |
| `nginx.ingress.kubernetes.io/cors-allow-headers: "Upload,X-Csrftoken,X-Auth-Token,Authorization"`          | `k8s.apisix.apache.org/cors-allow-headers: "Upload,X-Csrftoken,X-Auth-Token,Authorization"`              | 用于设置允许的请求头，迁移时建议完整保留原值。                                                                                      |
| `(nginx).ingress.kubernetes.io/backend-protocol: GRPC/HTTPS`                                                | `k8s.apisix.apache.org/upstream-scheme: "grpc/https"`                                                         | 用于声明上游为 gRPC 服务。迁移后仍需结合服务端口、协议和探针行为进行验证。                                                                     |
| `(nginx).ingress.kubernetes.io/proxy-body-size: "0"`                                                   | `整体配置 nginx_config.http.client_max_body_size: 0 不再需要单独设置`                                                | 用于放开或调整请求体大小限制。迁移后应确认 APISIX 实际是否允许无限制或是否需要按字节单位重新定义。                                                        |
| `nginx.ingress.kubernetes.io/auth-response-headers: Authorization`                                        | `k8s.apisix.apache.org/auth-upstream-headers: Authorization`                                             | 返回添加的头部          需要额外使用 k8s.apisix.apache.org/auth-request-headers: User-Agent,cookie转发头部给转发给 APISIX 的外部认证服务 |
| `nginx.ingress.kubernetes.io/auth-url: http://oath-gateway.ems.svc.cluster.local/decisions$request_uri`   | `k8s.apisix.apache.org/auth-uri: http://oath-gateway.ems.svc.cluster.local/decisions$request_uri`        | 外部认证地址       需要额外使用 k8s.apisix.apache.org/auth-request-headers: User-Agent,cookie转发头部给APISIX 的外部认证服务         |
| `nginx.ingress.kubernetes.io/proxy-buffer-size: 512k`                                                     | `整体配置 nginx_config.http_configuration_snippet.proxy_buffer_size: 512K 不再需要单独设置`                          |                                                                                                              |
| `nginx.ingress.kubernetes.io/proxy-buffers-number: '8'`                                                   | `整体配置 nginx_config.http_configuration_snippet.proxy_buffers: "8 512k" 不再需要单独设置`                          |                                                                                                              |
| `ingress.kubernetes.io/configuration-snippet: 'rewrite ^/iam/(.*) /$1 break;'`                             | `k8s.apisix.apache.org/rewrite-target-regex 以及 k8s.apisix.apache.org/rewrite-target-regex-template 配合使用` | 对包含正则分组的简单重写可近似迁移；需结合实际路径匹配规则验证效果。复杂重写不建议只依赖单注解。                                                             |
| `ingress.kubernetes.io/whitelist-source-range: 10.20.0.0/24,192.168.10.0/24`                             | `k8s.apisix.apache.org/allowlist-source-range: 10.20.0.0/24,192.168.10.0/24` | 访问白名单。                                                                                                       |

## 3.2 可平滑迁移的字段等价表
| ingress-nginx 字段                                                      | APISIX Ingress 字段                          | 说明                                                                                                                                                                                                         |
|-----------------------------------------------------------------------|--------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `.spec.rules[0].paths[0].pathType: ImplementationSpecific`            | `.spec.rules[0].paths[0].pathType: Prefix` | 1.当对应ingress path明确没有正则 并且为prefix匹配 应该使用此代替 <br/> 2.当对应ingress path有正则 并且为prefix匹配 应该不使用此代替 保持原样并使用k8s.apisix.apache.org/use-regex: "true" 注解确保路径按正则解析<br/> 3. 当对应ingress path无正则 并且为Exact 则可以不修改 但不需要添加注解 |
| `.spec.ingressClassName: nginx or kubernetes.io/ingress.class: nginx` | `.spec.ingressClassName: apisix`           | 指定控制器为apisix                                                                                                                                                                                               |


## 3.3 平滑迁移说明

平滑迁移的适用前提主要包括：

- 原有注解语义较单一
- `APISIX Ingress` 存在对应注解能力
- 不依赖 `NGINX configuration-snippet` 一类底层指令扩展
- 不依赖会话亲和、复杂认证、复杂重写、多级错误页接管等高级行为

对于表格中的注解，迁移时建议遵循以下原则：

1. 先做一对一替换，确保主路径可用。
2. 再对超时、跨域、上传大小等行为做回归验证。
3. 若发现与原 `ingress-nginx` 行为存在差异，应及时回退到插件或 `CRD` 方案，而不是继续叠加临时注解规避问题。

# 4. 非平滑迁移

对于一部分 `ingress-nginx` 配置，无法通过单纯替换为 `APISIX Ingress` 注解来保持行为一致。这类场景通常依赖：

- `NGINX` 私有指令能力
- 与 `NGINX` 执行阶段强相关的配置片段
- 需要额外上游建模能力的特性
- 需要独立证书、认证、限流、会话策略的网关能力

这类场景应归类为“非平滑迁移”。迁移时通常需要：

- 引入 `ApisixPluginConfig`
- 引入 `ApisixUpstream`
- 引入 `ApisixTls`
- 或者拆分原有 `Ingress`，按路径、认证、重写、会话策略分别建模

## 4.1 典型非平滑迁移场景(注意 使用的ApisixPluginConfig 定义粒度为ns级别)

### 4.1.1 会话亲和

#### 4.1.1.1 相关 ingress-nginx 注解

| ingress-nginx 注解 | 说明                                              |
| --- |-------------------------------------------------|
| `ingress.kubernetes.io/affinity: cookie` | 表示启用基于 Cookie 的会话亲和，请求会尽量根据指定 Cookie 持续命中同一后端实例。 |
| `ingress.kubernetes.io/session-cookie-name` | 指定用于会话亲和的 Cookie 名称，例如 `escookie`。              |
| `ingress.kubernetes.io/session-cookie-hash: sha1` | 指定会话 Cookie 的哈希算法；该行为在 APISIX 中没有直接等价字段。        |

#### 4.1.1.2 APISIX 迁移配置示例

`session-cookie-hash` 在 APISIX Ingress 注解中没有 1:1 的原生等价字段。APISIX Ingress 也没有 `k8s.apisix.apache.org/upstream-hash` 这类注解；会话亲和需要通过 `BackendTrafficPolicy` 等资源表达。

该方案目标是**行为等价**（稳定路由），而不是完全复刻 `ingress-nginx` 的内部实现细节。

**步骤 A：部署并注册自定义插件**

集群管理员需提前将 `plugins/session-cookie-hash.lua` 部署到 APISIX 插件目录，并在 `config.yaml` 的 `plugins` 列表中注册 `session-cookie-hash`。

**步骤 B：配置 ApisixPluginConfig**

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: session-cookie-hash-conf
  namespace: openstack
spec:
  ingressClassName: apisix
  plugins:
    - name: session-cookie-hash
      enable: true
      config:
        cookie_name: "escookie"
        algorithm: "sha1"           # 可选: sha1 / md5 / sha256
        header_name: "X-Session-Hash"
        fallback: "pass"            # pass: 缺失 cookie 时不设置头；empty: 设置为空字符串
        generate_cookie: true       # 缺失 cookie 时生成一个新值，并通过 Set-Cookie 返回
        cookie_httponly: false      # 仅显式设为 true 时才写 HttpOnly
        # cookie_path: "/"          # 可选；不填或设为 "" 时不写 Path 属性
```

**步骤 C：配置 BackendTrafficPolicy 承载会话亲和**

```yaml
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: emla-grafana-cookie
  namespace: openstack
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: grafana-dashboard
  loadbalancer:
    type: chash
    hashOn: cookie
    key: escookie
```

其中 `key` 对应 `nginx.ingress.kubernetes.io/session-cookie-name` 的值，例如 `route` 或 `escookie`。

**自动转换说明**

- 当存在 `session-cookie-hash`（值为 `sha1`/`md5`/`sha256`）时，转换器会自动生成 `session-cookie-hash` 插件配置
- 当存在 `affinity: cookie` 但缺少 `session-cookie-hash` 时，转换器默认使用 `sha1` 生成插件，并输出告警提示可显式选择算法
- `cookie_name` 来自 `session-cookie-name`；若缺失，默认使用 `INGRESSCOOKIE` 并输出告警提示
- 当请求缺失该 cookie 时，插件默认生成新 cookie，写入当前请求头，并在响应中返回 `Set-Cookie`
- `cookie_path` 默认不输出；只有显式配置非空值时才会写入 `Path=...`
- `cookie_httponly` 默认关闭；只有显式设为 `true` 时才会写入 `HttpOnly`
- `affinity: cookie` 会自动生成 `BackendTrafficPolicy`，不会生成不存在的 `k8s.apisix.apache.org/upstream-hash` 注解
- 若 `session-cookie-hash` 为不支持的值，转换器会输出清晰告警并要求手动调整

### 4.1.2 upstream location改写

#### 4.1.2.1 相关 ingress-nginx 注解

| ingress-nginx 注解 | 说明 |
| --- | --- |
| `ingress.kubernetes.io/proxy-redirect-from: http://` | 指定对 upstream 返回响应中的 `Location` 头进行匹配，当其前缀为 `http://` 时，参与后续改写。 |
| `ingress.kubernetes.io/proxy-redirect-to: https://` | 指定将 upstream 返回响应中的 `Location` 头前缀改写为 `https://`，从而避免客户端被重定向回 HTTP。 |

#### 4.1.2.2 APISIX 迁移配置示例

- 集群管理员提前准备好 `fixed-location-rewrite` 插件，示例代码如下：

```lua
local core = require("apisix.core")

local plugin_name = "fixed-location-rewrite"

local schema = {
    type = "object",
    properties = {
        from_scheme = {
            type = "string",
            minLength = 1,
            default = "http://",
        },
        to_scheme = {
            type = "string",
            minLength = 1,
            default = "https://",
        },
        status_codes = {
            type = "array",
            minItems = 1,
            items = {
                type = "integer",
                minimum = 100,
                maximum = 599,
            },
            default = {301, 302, 303, 307, 308},
        },
    },
}

local _M = {
    version = 0.1,
    priority = 899,
    name = plugin_name,
    schema = schema,
}

function _M.check_schema(conf)
    return core.schema.check(schema, conf)
end

function _M.header_filter(conf, ctx)
    local current_status = ngx.status
    local matched = false

    for _, code in ipairs(conf.status_codes or {}) do
        if current_status == code then
            matched = true
            break
        end
    end

    if not matched then
        return
    end

    local location = ngx.header["Location"]
    if not location then
        return
    end

    if type(location) == "table" then
        location = location[1]
    end

    if type(location) ~= "string" then
        return
    end

    if location:sub(1, #conf.from_scheme) ~= conf.from_scheme then
        return
    end

    local rewritten = conf.to_scheme .. location:sub(#conf.from_scheme + 1)
    core.response.set_header("Location", rewritten)
end

return _M
```

- 在 Ingress 所在命名空间创建 `ApisixPluginConfig`：

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: fixed-location-rewrite
  namespace: ems
spec:
  ingressClassName: apisix
  plugins:
    - name: fixed-location-rewrite
      enable: true
```

- 在 Ingress 上添加注解引用该插件配置：

```yaml
k8s.apisix.apache.org/plugin-config-name: fixed-location-rewrite
```

### 4.1.3 自定义 HTTP 错误处理

#### 4.1.3.1 相关 ingress-nginx 注解

| ingress-nginx 注解 | 说明        |
| --- |-----------|
| `ingress.kubernetes.io/custom-http-errors: 404,500` | 拦截指定的错误码 重定向到 Ingress 中配置的 default-backend 服务 |

#### 4.1.3.2 APISIX 迁移配置示例   待开发

- 静态404/500 lua插件返回代码
```lua
--
-- Licensed to the Apache Software Foundation (ASF) under one or more
-- contributor license agreements.  See the NOTICE file distributed with
-- this work for additional information regarding copyright ownership.
-- The ASF licenses this file to You under the Apache License, Version 2.0
-- (the "License"); you may not use this file except in compliance with
-- the License.  You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.
--

local plugin_name = "custom-error-page"

local ngx = ngx
local core = require("apisix.core")
local apisix_plugin = require("apisix.plugin")
local apisix_utils = require("apisix.core.utils")

local function default_html(img)
    return string.format([[
<!DOCTYPE html>
<html>
  <head>
    <style>
      * {
        margin: 0;
        padding: 0;
      }
      html, body {
        width: 100%%;
        height: 100%%;
      }
      img {
        display: block;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        margin: auto;
      }
    </style>
  </head>
  <body>
    <img src="%s" alt="">
  </body>
</html>
]], img)
end

local metadata_schema = {
    type = "object",
    additionalProperties = false,
    properties = {
        id = {
            type = "string",
        },
        enable = {
            type = "boolean",
            default = true,
        },
        error_404 = {
            type = "object",
            additionalProperties = false,
            properties = {
                body = {
                    type = "string",
                    default = default_html("/es404.png"),
                },
                ["content-type"] = {
                    type = "string",
                    default = "text/html; charset=utf-8",
                },
            },
        },
        error_500 = {
            type = "object",
            additionalProperties = false,
            properties = {
                body = {
                    type = "string",
                    default = default_html("/es500.png"),
                },
                ["content-type"] = {
                    type = "string",
                    default = "text/html; charset=utf-8",
                },
            },
        },
    },
    anyOf = {
        { required = { "error_404" } },
        { required = { "error_500" } },
    },
}

local schema = {
    type = "object",
    properties = {},
}

local _M = {
    version = 0.1,
    priority = 0,
    name = plugin_name,
    schema = schema,
    metadata_schema = metadata_schema,
}

local default_conf = {
    enable = true,
    error_404 = {
        body = default_html("/es404.png"),
        ["content-type"] = "text/html; charset=utf-8",
    },
    error_500 = {
        body = default_html("/es500.png"),
        ["content-type"] = "text/html; charset=utf-8",
    },
}

local function make_response(page)
    return {
        body = page.body,
        headers = {
            ["Content-Type"] = page["content-type"],
        },
    }
end

function _M.check_schema(conf, schema_type)
    if schema_type == core.schema.TYPE_METADATA then
        return core.schema.check(metadata_schema, conf)
    end

    return true
end

local function get_effective_conf()
    local metadata = apisix_plugin.plugin_metadata(plugin_name)
    if not metadata or not metadata.value then
        return default_conf
    end

    if metadata.value.enable == false then
        return {
            enable = false,
        }
    end

    local conf = {
        enable = true,
        error_404 = default_conf.error_404,
        error_500 = default_conf.error_500,
    }

    if metadata.value.error_404 then
        conf.error_404 = metadata.value.error_404
    end

    if metadata.value.error_500 then
        conf.error_500 = metadata.value.error_500
    end

    return conf
end

function _M.header_filter(_, ctx)
    local conf = get_effective_conf()
    if not conf.enable then
        return
    end

    local custom_response

    if ngx.status == 404 and conf.error_404 then
        custom_response = make_response(conf.error_404)
    elseif ngx.status == 500 and conf.error_500 then
        custom_response = make_response(conf.error_500)
    end

    if not custom_response then
        return
    end

    for key, value in pairs(custom_response.headers) do
        ngx.header[key] = value
    end

    custom_response.body = apisix_utils.resolve_var(custom_response.body, ngx.var)

    ngx.header["Content-Length"] = #custom_response.body
    ngx.header["Content-Encoding"] = nil

    ctx.custom_error_page_body = custom_response.body
end

function _M.body_filter(_, ctx)
    if not ctx.custom_error_page_body then
        return
    end

    local body = core.response.hold_body_chunk(ctx)
    if ngx.arg[2] == false and not body then
        return
    end

    ngx.arg[1] = ctx.custom_error_page_body
    ngx.arg[2] = true
    ctx.custom_error_page_body = nil
end

return _M

```

- apisixpluginconfig配置
```yaml
[root@node-2 plugins]# cat custom-error-page-conf
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: custom-error-page-conf
  namespace: ems
spec:
  ingressClassName: apisix
  plugins:
    - name: custom-error-page
      enable: true
      config: {}
```
- ingress上需要注解
```yaml
k8s.apisix.apache.org/plugin-config-name: custom-error-page-conf
```


### 4.1.4 cookie flag动态改写

#### 4.1.4.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                                                   | 说明                                              |
|------------------------------------------------------------------------------------|-------------------------------------------------|
| `nginx.ingress.kubernetes.io/configuration-snippet: proxy_cookie_flags sessionid SameSite=None Secure;` | 对带有sessionid的cookie 注入新的SameSite=None Secure 属性 |


#### 4.1.4.2 APISIX 迁移配置示例

该能力在 APISIX 中通过自定义 `proxy-cookie-flags` Lua 插件统一实现。集群管理员需提前将 `plugins/proxy-cookie-flags.lua` 部署到 APISIX 插件目录，并在 `config.yaml` 的 `plugins` 列表中注册。

插件配置规则说明：

| 字段 | 说明 |
| --- | --- |
| `match` | Cookie 名称匹配模式。支持精确名称（如 `sessionid`）、通配符 `*`（匹配所有）、正则 `~pattern`（如 `~^sess_`） |
| `flags` | 要注入或替换的属性列表。支持：`Secure`、`noSecure`、`HttpOnly`、`noHttpOnly`、`SameSite=None`、`SameSite=Lax`、`SameSite=Strict`、`noSameSite` |

插件行为说明：

- 匹配规则按顺序执行，**第一条匹配的规则生效**后立即停止，不会继续匹配后续规则
- `SameSite=<value>` 会先移除已有的 SameSite 属性，再注入新值
- `Secure` / `HttpOnly` 采用"已存在则跳过"策略，不会重复添加
- `noSecure` / `noHttpOnly` / `noSameSite` 用于移除已有属性
- 多个 `Set-Cookie` 响应头中每个 Cookie 独立匹配规则

原始 `ingress-nginx` 中两种 Cookie 改写场景均由本插件统一处理：

**场景 A：configuration-snippet 中的 proxy_cookie_flags 指令**

原始 `ingress-nginx` 通过 `configuration-snippet` 嵌入 `proxy_cookie_flags` nginx 指令，对指定 Cookie 追加或替换安全属性：

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_cookie_flags sessionid SameSite=None Secure;
      proxy_cookie_flags auth_token HttpOnly Secure;
      proxy_cookie_flags * SameSite=Lax;
```

创建 `ApisixPluginConfig`：

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: cookie-flags-config
  namespace: <namespace>
spec:
  ingressClassName: apisix
  plugins:
    - name: proxy-cookie-flags
      enable: true
      config:
        rules:
          # sessionid 强制 SameSite=None + Secure（跨站场景常用）
          - match: "sessionid"
            flags: ["SameSite=None", "Secure"]
          # auth_token 强制 HttpOnly + Secure（防 XSS 读取）
          - match: "auth_token"
            flags: ["HttpOnly", "Secure"]
          # 其余 Cookie 默认 SameSite=Lax
          - match: "*"
            flags: ["SameSite=Lax"]
```

在 Ingress 上添加注解引用该插件配置：

```yaml
k8s.apisix.apache.org/plugin-config-name: cookie-flags-config
```

**自动转换工具处理逻辑**

`ingress2apisix` 转换器会自动处理 `configuration-snippet` 中的 `proxy_cookie_flags` 指令：

1. 从 `configuration-snippet` 中正则提取所有 `proxy_cookie_flags` 指令，转换为 `rules` 配置
2. 生成独立的 `ApisixPluginConfig` 资源
3. 移除原注解，添加 `plugin-config-name` 引用


```bash
# 输入：包含 configuration-snippet 的 Ingress
ingress2apisix -f ingress.yaml

# 输出自动包含：
# 1. 转换后的 Ingress（含 plugin-config-name 注解，原注解已移除）
# 2. 独立的 ApisixPluginConfig 资源（含 proxy-cookie-flags 规则）
```

### 4.1.4.3 Cookie Path 改写

#### 相关 ingress-nginx 注解

| ingress-nginx 注解 | 说明 |
| --- | --- |
| `ingress.kubernetes.io/proxy-cookie-path: ~(.*) "$1"` | 对响应中 `Set-Cookie` 头的 `Path` 属性进行正则或精确替换。等价于 nginx 的 `proxy_cookie_path` 指令。 |

nginx 原生 `proxy_cookie_path` 指令说明：

```nginx
# 精确替换：将 Set-Cookie 中 Path=/old-path 改为 /new-path
proxy_cookie_path /old-path /new-path;

# 正则替换：将 Path=/api/xxx 改为 /xxx
proxy_cookie_path ~^/api/(.*) /$1;

# 匹配所有路径（通常用于统一加前缀或尾部斜杠）
proxy_cookie_path ~(.*) "$1";
```

#### 背景

`proxy_cookie_path` 在 nginx 中的作用域是 `http`、`server`、`location`，它对响应中**所有** `Set-Cookie` 头的 `Path` 属性进行统一改写。nginx 本身没有按 cookie 名称筛选的能力——要么全部改，要么不改。

本插件在此基础上扩展了 `cookie` 字段，可以**按 cookie 名称精确控制**哪些 cookie 需要改写 Path，哪些保持不变。

#### APISIX 迁移配置示例

该能力在 APISIX 中通过自定义 `proxy-cookie-path` Lua 插件实现。

**插件配置说明**

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `path_pairs` | 是 | 路径替换规则数组，按顺序匹配，命中即停止 |
| `path_pairs[].match` | 是 | 匹配模式。精确路径如 `/old-path`；正则前缀 `~`，如 `~(.*)`、`~^/api/(.*)` |
| `path_pairs[].replacement` | 是 | 替换值。支持正则分组引用如 `$1`、`$2` |
| `path_pairs[].cookie` | 否 | 限定只对指定 cookie 名称生效。省略或为空则匹配所有 cookie |

**插件行为**

- 在 `header_filter` 阶段运行，改写的是**响应**方向的 `Set-Cookie` 头
- `Path` 属性匹配不区分大小写（`Path`、`path`、`PATH` 均可识别）
- 多条规则按数组顺序依次匹配，**第一条命中的规则生效**，后续规则不再执行
- 如果 `Set-Cookie` 中没有 `Path` 属性，该 cookie 不受影响
- 如果没有任何规则命中，cookie 保持原样

---

**场景 A：对所有 cookie 做正则替换**

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: cookie-path-all
  namespace: default
spec:
  ingressClassName: apisix
  plugins:
    - name: proxy-cookie-path
      enable: true
      config:
        path_pairs:
          - match: "~(.*)"
            replacement: "$1"
```

效果：所有 `Set-Cookie` 中的 `Path` 值原样保留（此配置通常用于后续扩展时作为占位）。

---

**场景 B：精确路径替换**

```yaml
config:
  path_pairs:
    - match: "/old-path"
      replacement: "/new-path"
```

效果：

| 改写前 | 改写后 |
| --- | --- |
| `Set-Cookie: sid=abc; Path=/old-path` | `Set-Cookie: sid=abc; Path=/new-path` |
| `Set-Cookie: sid=abc; Path=/other` | `Set-Cookie: sid=abc; Path=/other`（不匹配，不变） |
| `Set-Cookie: sid=abc` | `Set-Cookie: sid=abc`（无 Path，不变） |

---

**场景 C：正则提取路径子串**

```yaml
config:
  path_pairs:
    - match: "~^/api/(.*)"
      replacement: "/$1"
```

效果：

| 改写前 | 改写后 |
| --- | --- |
| `Set-Cookie: sid=abc; Path=/api/v1` | `Set-Cookie: sid=abc; Path=/v1` |
| `Set-Cookie: sid=abc; Path=/api/users/123` | `Set-Cookie: sid=abc; Path=/users/123` |
| `Set-Cookie: sid=abc; Path=/other` | 不变（正则不匹配） |

---

**场景 D：按 cookie 名称精确控制（扩展能力，nginx 原生不支持）**

```yaml
config:
  path_pairs:
    # 只对 sessionid cookie 做路径替换
    - cookie: "sessionid"
      match: "~^/api/(.*)"
      replacement: "/$1"
    # 对其他所有 cookie 做不同的替换
    - match: "/old-path"
      replacement: "/new-path"
```

效果：

| Cookie | 改写前 Path | 改写后 Path | 命中规则 |
| --- | --- | --- | --- |
| `sessionid` | `/api/v1` | `/v1` | 第 1 条（指定 cookie + 正则） |
| `sessionid` | `/other` | `/other` | 第 1 条匹配了 cookie 但正则不匹配，跳过；第 2 条不匹配，不变 |
| `token` | `/old-path` | `/new-path` | 第 1 条 cookie 不匹配，跳过；第 2 条命中 |
| `token` | `/api/v1` | `/api/v1` | 两条都不命中，不变 |

---

**场景 E：多条规则组合**

```yaml
config:
  path_pairs:
    # 第一条：sessionid 只改 /api 下的路径
    - cookie: "sessionid"
      match: "~^/api/(.*)"
      replacement: "/$1"
    # 第二条：csrftoken 只改精确路径
    - cookie: "csrftoken"
      match: "/dashboard"
      replacement: "/app"
    # 第三条：其余 cookie 统一去掉尾部斜杠
    - match: "~^(.*)/$"
      replacement: "$1"
```

---

#### 自动转换工具处理逻辑

`ingress2apisix` 转换器会自动解析 `ingress.kubernetes.io/proxy-cookie-path` 注解值，生成 `ApisixPluginConfig`：

**输入 Ingress：**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    ingress.kubernetes.io/proxy-cookie-path: ~(.*) "$1"
spec:
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

**自动生成的输出：**

1. `ApisixPluginConfig` 资源：

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: my-app-plugins
  namespace: <namespace>
spec:
  ingressClassName: apisix
  plugins:
    - name: proxy-cookie-path
      enable: true
      config:
        path_pairs:
          - match: "~(.*)"
            replacement: "$1"
```

2. 转换后的 Ingress 自动添加引用：

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/plugin-config-name: my-app-plugins
```

**注意：** 自动转换生成的规则**不带 `cookie` 字段**，即对所有 cookie 生效（与 nginx 原生行为一致）。如果需要按 cookie 名称控制，需在生成后手动编辑 `ApisixPluginConfig` 添加 `cookie` 字段。

**验证方法**

```bash
# 使用 httpbin 的 /response-headers 端点返回自定义 Set-Cookie
# 观察 Path 是否被改写

# 精确替换测试
curl -v -H "Host: app.example.com" \
  'http://<APISIX>/response-headers?Set-Cookie=sid%3Dabc%3B%20Path%3D/old-path'

# 正则替换测试
curl -v -H "Host: app.example.com" \
  'http://<APISIX>/response-headers?Set-Cookie=sid%3Dabc%3B%20Path%3D/api/v1'

# 多 cookie 测试（验证 cookie 字段过滤）
curl -v -H "Host: app.example.com" \
  'http://<APISIX>/response-headers?Set-Cookie=sessionid%3Dabc%3B%20Path%3D/api/v1&Set-Cookie=token%3Dxyz%3B%20Path%3D/api/v1'
```

```bash
ingress2apisix convert -f ingress.yaml

# 输出自动包含：
# 1. 转换后的 Ingress（含 plugin-config-name 注解，原注解已移除）
# 2. 独立的 ApisixPluginConfig 资源（含 proxy-cookie-path 规则）
```

### 4.1.5 头部转化

#### 4.1.5.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                                                        | 说明      |
|-----------------------------------------------------------------------------------------|---------|
| `nginx.ingress.kubernetes.io/configuration-snippet: more_set_headers "X-Forwarded-For $http_x_forwarded_for";` | 动态设置头部 X-Forwarded-For |

#### 4.1.5.2 APISIX 迁移配置示例

- 在ns下准备插件配置 使用proxy-rewrite 插件重写配置
```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: common-headers
  namespace: iam
spec:
  ingressClassName: apisix
  plugins:
  - config:
      headers:
        set:
          X-Forwarded-For: $http_x_forwarded_for
    enable: true
    name: proxy-rewrite

```
- 在ingress上添加注解 使用此配置
```yaml
k8s.apisix.apache.org/plugin-config-name: common-headers
```


### 4.1.6 mTLS 证书认证

#### 4.1.6.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                                                        | 说明                             |
|-----------------------------------------------------------------------------------------|--------------------------------|
| `nginx.ingress.kubernetes.io/auth-tls-secret` | 指定“信任的 CA 证书” 以通过网关检查客户端证书是否合法 |
| `nginx.ingress.kubernetes.io/auth-tls-verify-client` | 强制要求客户端必须提供证书                  |

#### 4.1.6.2 APISIX 迁移配置示例

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixTls
metadata:
  name: sample-tls
  namespace: openstack
spec:
  ingressClassName: apisix
  hosts:
    - ss.grpc.7yg.rmdcrmildxmn.example.io
  secret:
    name: ecms-tls-federate
    namespace: openstack
  client:
    caSecret:
      name: ecms-tls-federate
      namespace: openstack
    depth: 2
```
注意:
1. 要求client caSecret对应的secret需要具有 ca.crt
2. 对应的ingress配置中不要设置spec.tls.secretName 否则会造成多个ssl配置下发 会导致配置不符合预期 对应的spec.tls.secretName 可以迁移到ApisixTls.spec.secret下


### 4.1.7 同ingress多路径改写

#### 4.1.7.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                     | 说明            |
|------------------------------------------------------|---------------|
| `ingress.kubernetes.io/configuration-snippet:'rewrite ^/shard/0/(.*) /$1 break;rewrite ^/am/(.*) /$1 break;'`       | 多路径重写         |

#### 4.1.7.2 APISIX 迁移配置示例

- 在ns下准备插件配置 使用proxy-rewrite 插件重写配置

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: ecms-web-rewrite
  namespace: openstack
spec:
  ingressClassName: apisix
  plugins:
    - name: proxy-rewrite
      enable: true
      config:
        regex_uri:
          - "^/shard/0/(.*)"
          - "/$1"
          - " ^/am/(.*)"
          - "/$1"
```
理论上这里的匹配和转换后的nginx template语义一致即可

- 在ingress上添加注解 使用此配置
```yaml
k8s.apisix.apache.org/plugin-config-name: ecms-web-rewrite
```

### 4.1.8 basic 认证

#### 4.1.8.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                     | 说明 |
|------------------------------------------------------|--|
| `nginx.ingress.kubernetes.io/auth-type: basic`       | 启用 HTTP Basic 认证 |
| `nginx.ingress.kubernetes.io/auth-realm: '401: Authentication Required'`       | 定义 401 认证提示信息 |
| `nginx.ingress.kubernetes.io/auth-secret: ecms-basic-auth`       | 提供用户名/密码数据源 |


#### 4.1.8.2 APISIX 迁移配置示例

- 在ns下准备插件配置 使用basic-auth 插件重写配置

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: basic-auth-conf
  namespace: openstack
spec:
  ingressClassName: apisix
  plugins:
    - name: basic-auth
      enable: true
      config:
        realm: "401: Authentication Required"

```
```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata:
  name: ecms-user
  namespace: openstack
spec:
  ingressClassName: apisix
  authParameter:
    basicAuth:
      value:
        username: admin@ecms.io
        password: xx
```

- 在ingress上添加注解 使用此配置
```yaml
k8s.apisix.apache.org/plugin-config-name: basic-auth-conf
```
### 4.1.9 rps限制


#### 4.1.9.1 相关 ingress-nginx 注解

| ingress-nginx 注解                                     | 说明      |
|------------------------------------------------------|---------|
| `nginx.ingress.kubernetes.io/limit-rps: '1000'`       | 限制每秒请求数        |
| `ingress.kubernetes.io/configuration-snippet: 'limit_req_status 429;`       | 自定义限流被拒绝时返回的 HTTP 状态码 |

#### 4.1.9.2 APISIX 迁移配置示例

- 在ns下准备插件配置 使用limit-req 插件重写配置

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: limit-rps-emla
  namespace: openstack
spec:
  ingressClassName: apisix
  plugins:
    - config:
        burst: 0
        key: remote_addr
        rate: 1000
        rejected_code: 429
      enable: true
      name: limit-req



```

- 在ingress上添加注解 使用此配置
```yaml
k8s.apisix.apache.org/plugin-config-name: limit-rps-emla
```

- 验证限流生效方法
```yaml
seq 1 1200 | xargs -I{} -P200 curl -s -o /dev/null -w "%{http_code}\n" http://(host)/ | sort | uniq -c
```








