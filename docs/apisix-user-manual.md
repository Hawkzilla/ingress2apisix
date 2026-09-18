# APISIX 网关用户使用手册

> 面向对象：平台运维人员、应用开发人员
> 适用范围：本文按当前环境的 APISIX Ingress Controller 2.0.1-es + APISIX 3.16-es 系列 standalone（API-driven）组合核验（集群级实例为 3.16.0-es.1）；实际版本、镜像与组件清单请按 1.4 节命令从环境获取。其他版本请按对应版本文档重新核验。
> **本文不写死任何环境参数**：IP、域名、VIP、端口、密钥、实例名、命名空间等一律用 `<占位符>` 表示，请先按第 1 章的方法从实际环境获取，再代入命令。
> 配置主体：**以标准 Kubernetes Ingress（networking.k8s.io/v1）为主**；APISIX CRD 用于补充 Ingress 未覆盖的能力，例如插件、消费者、上游策略、TLS/SNI 和全局规则。

## 目录

- [1. 使用前：如何获取环境数据](#1-使用前如何获取环境数据)
- [2. 部署与运维（平台运维）](#2-部署与运维平台运维)
- [3. 路由配置（应用开发者）](#3-路由配置应用开发者)
- [4. 常见场景示例](#4-常见场景示例)
- [5. 故障排查](#5-故障排查)
- [6. 附录](#6-附录)

---

## 1. 使用前：如何获取环境数据

> 本章是全文的基础。所有后续命令中的 `<占位符>` 都来自本章的采集结果。

### 1.1 先确认：集群里有哪些 APISIX 实例

同一集群**可以部署多套互相隔离的 APISIX 实例**，每套实例有自己的命名空间、数据面、Admin 端口和 Keepalived VIP。**一条 Ingress 通常通过 `spec.ingressClassName` 选择实例**，还应确认该 IngressClass 的 `spec.controller` 与 GatewayProxy 指向目标实例。

```bash
# ① 列出所有 IngressClass（每行代表一套实例）
kubectl get ingressclass -o wide

# ② 查看某个 class 对应的 controller 名称与 GatewayProxy（数据面连接信息）
kubectl get ingressclass <INSTANCE_CLASS> -o jsonpath='controller={.spec.controller}{"\n"}gatewayproxy={.spec.parameters.namespace}/{.spec.parameters.name}{"\n"}'

# ③ 列出所有 GatewayProxy（= 所有实例的数据面）
kubectl get gatewayproxy -A
```

判读：

| 字段 | 含义 |
|---|---|
| `IngressClass.spec.controller` | 实例的控制器唯一标识；不同实例必须不同 |
| `IngressClass.spec.parameters` | 指向该实例的 GatewayProxy（关联 Admin Service 与 Admin Key） |
| `GatewayProxy.metadata.namespace` | 该实例的命名空间（网关组件都在这） |

> **注意**：`GatewayProxy` 是 APISIX Ingress Controller 用来描述“数据面连接信息（Admin 地址、Admin Key）”的 CRD，通过 `IngressClass.spec.parameters` 与 IngressClass 绑定；业务通常只需使用平台分配的 IngressClass，并从平台获取对应的网关入口地址。

### 1.2 环境数据获取速查表

| 需要的数据 | 占位符 | 获取命令 |
|---|---|---|
| 实例的 IngressClass | `<INSTANCE_CLASS>` | `kubectl get ingressclass -o wide` |
| 实例命名空间 | `<INSTANCE_NS>` | 上表 GatewayProxy 所在 namespace |
| 实例的 Admin Service/端口 | `<ADMIN_SVC>` / `<ADMIN_PORT>` | `kubectl -n <INSTANCE_NS> get gatewayproxy <GATEWAYPROXY_NAME> -o jsonpath='{.spec.provider.controlPlane.service.name}:{.spec.provider.controlPlane.service.port}{"\n"}'` |
| **Admin Key** | `$ADMIN_API_KEY` | 优先从 GatewayProxy 引用的 Secret 获取；不要在普通终端输出或记录真实密钥 |
| 数据面节点与节点 IP | `<APISIX_NODE_IP>` | `kubectl -n <INSTANCE_NS> get pods -o wide`（hostNetwork 时 Pod IP = 节点 IP） |
| 网关 HTTP/HTTPS 端口 | `<HTTP_PORT>` / `<HTTPS_PORT>` | `kubectl -n <INSTANCE_NS> get svc` 或读 APISIX ConfigMap 的 `node_listen`/`ssl.listen` |
| **Keepalived VIP**（网关入口，非节点 IP） | `<KEEPALIVED_VIP>` | `kubectl -n <INSTANCE_NS> get cm -l component=keepalived -o yaml | grep -A3 virtual_ipaddress`（keepalived.conf 中的 `virtual_ipaddress`） |
| Keepalived VRID | `<VRID>` | 同上，`grep virtual_router_id` |
| 实例名称前缀 | `<INSTANCE_NAME>` | `kubectl -n <INSTANCE_NS> get ds,deploy` 中的资源名前缀 |
| 数据面/控制器镜像版本 | — | `kubectl -n <INSTANCE_NS> get ds,deploy -o jsonpath='{range .items[*]}{.kind} {.metadata.name} {.spec.template.spec.containers[*].image}{"\n"}{end}'` |
| 控制器配置（同步周期、调试端口） | — | `kubectl -n <INSTANCE_NS> get cm <controller-cm> -o jsonpath='{.data.config\.yaml}'` |
| 控制器调试端口 | `<DEBUG_PORT>` | 上述 ConfigMap 中 `server_addr` 的端口（且 `enable_server=true`） |
| 业务域名解析情况 | `<业务域名>` | `getent hosts <业务域名>` 或 `dig +short <业务域名>` |
| 节点角色与污点 | — | `kubectl get node -o custom-columns='NAME:.metadata.name,TAINTS:.spec.taints'` |
| 命名空间是否需要加入网络租户 | `<NET_TENANT>` | `kubectl get raptortenant,raptorsubnet -A`（未使用 Raptor CNI 的集群跳过） |

Admin Key 可能由 GatewayProxy 的 `valueFrom.secretKeyRef` 引用 Secret。Admin Key 仅供平台运维使用，不能写入文档、脚本仓库或普通 ConfigMap；若必须临时读取，请避免回显并在使用后轮换。某些旧部署会把密钥以内联值放在 GatewayProxy 或 APISIX ConfigMap 中，应按密钥泄露处理。

```bash
# 仅查看引用关系，不打印 Secret 内容
kubectl -n <INSTANCE_NS> get gatewayproxy <GATEWAYPROXY_NAME> -o jsonpath='{.spec.provider.controlPlane.auth.adminKey.valueFrom.secretKeyRef}'; echo
# 临时交互式输入（不回显），供后续 curl 使用
read -rsp 'Admin key: ' ADMIN_API_KEY; echo
```

### 1.3 IngressClass 选择（强制阅读）

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>        # ← 通常决定使用哪套实例
  rules:
    - host: my-app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

上线前自检：

```bash
# 1) class 是否存在、指向哪套实例
kubectl get ingressclass <INSTANCE_CLASS> -o yaml

# 2) 我的 Ingress 用的是哪个 class
kubectl -n <业务命名空间> get ingress <Ingress名> -o jsonpath='{.spec.ingressClassName}{"\n"}'

# 3) Ingress 是否已被网关接管（status 会回填该实例的数据面地址）
kubectl -n <业务命名空间> get ingress <Ingress名> -o jsonpath='{.status}{"\n"}'
```

常见误区：

| 现象 | 原因 |
|---|---|
| 资源创建成功但访问 404 | `ingressClassName` 写成了别的实例的 class（或旧 nginx class） |
| 两个团队用同一个域名互不影响 | 不同 class 属于不同实例；**同一实例内**重复 host+path 才会冲突 |
| 修改 class 后旧实例仍有残留路由 | 控制器会回收，数秒后可用 Admin API 复查（见 5.6） |
| 迁移期新旧网关并存 | class 不同即两套路由；DNS 指向谁就由谁生效 |
| class 正确但访问仍 404 | 客户端访问了另一实例的 VIP/端口；多实例必须使用与 class 对应的网关入口 |

### 1.4 确认版本与配置模式

```bash
# 版本（镜像 tag）
kubectl -n <INSTANCE_NS> get ds,deploy -o jsonpath='{range .items[*]}{.kind}{" "}{.metadata.name}{" "}{.spec.template.spec.containers[*].image}{"\n"}{end}'

# 配置模式（期望看到 role: traditional + config_provider: yaml，即 standalone，无 etcd）
kubectl -n <INSTANCE_NS> get cm <apisix-cm> -o jsonpath='{.data.config\.yaml}' | grep -E 'role|config_provider|admin_listen|node_listen'
```

配置模式对排障的影响：

1. 配置由控制器/ADC 下发并保存在 APISIX 内存（shared dict），**Pod 重启后需等待控制器重新下发**，期间可能 404；恢复时间取决于事件、同步、网络和组件状态，不能承诺固定上限。
2. Admin API 只开放 `/apisix/admin/configs`（GET/PUT/HEAD），其他 Admin 路径（如 `/apisix/admin/routes`）返回 404 属正常现象。
3. 控制器事件驱动同步，并带周期性兜底（`sync_period` 可在控制器 ConfigMap 查看）。

---

## 2. 部署与运维（平台运维）

### 2.1 架构与数据流

```text
业务 YAML（Ingress / ApisixPluginConfig / ApisixConsumer / BackendTrafficPolicy ...）
        │ kubectl apply
        ▼
Ingress Controller（Deployment，通常 2 容器）
  ├─ manager     ：监听 K8s 资源、翻译为 ADC 模型
  └─ adc-server  ：生成 APISIX 配置
        │  PUT /apisix/admin/configs （Admin Key 认证）
        ▼
APISIX 数据面（本环境为 DaemonSet + hostNetwork；其他部署按实际资源，监听 <HTTP_PORT>/<HTTPS_PORT>）
        ▲
        │ 本环境 VIP 由 Keepalived DaemonSet 漂移（其他部署按实际入口）
客户端 ── <KEEPALIVED_VIP> ──┘
```

关键机制（排障必知）：

1. 每个实例的 `controller_name` 唯一，只处理自己 IngressClass 的资源，多实例互不干扰。
2. 数据面配置在内存；重启后需要控制器重新下发，通常为事件驱动的秒级恢复，但不能承诺固定上限。
3. 控制器同步是事件驱动 + 周期兜底；变更通常在秒级生效。Admin API 的 `X-Digest` 可确认配置版本，但仍需检查具体字段并执行数据面请求。

### 2.2 部署前检查清单

新增一套实例或扩容数据面节点前，逐项确认：

> 除 APISIX 数据面外，Ingress Controller 的 Deployment 也必须配置匹配的 `tolerations`/`nodeSelector`；带有 `NoSchedule` 污点的节点若未容忍，控制器不会被调度。

| 检查项 | 获取/验证方法 | 通过标准 |
|---|---|---|
| 目标节点存在且 Ready | `kubectl get node -o wide` | Ready |
| 节点污点与实例 tolerations 匹配 | `kubectl get node <node> -o jsonpath='{.spec.taints}'` | 实例能容忍该污点 |
| 命名空间已加入网络租户（Raptor 等） | `kubectl get raptortenant -A` | 新命名空间在租户 namespaces 列表中 |
| VIP 未被占用 | `ping <KEEPALIVED_VIP>` / `arping -D` 或联系网络 | 无冲突 |
| VIP 已在底层网络放通 | OpenStack allowed_address_pairs / 交换机配置 | 集群外可 ping 通 |
| 镜像仓库可达 | 在目标节点 `crictl pull <image>` | 拉取成功 |
| VRID 唯一 | 遍历现有实例 cm 的 `virtual_router_id` | 不重复 |
| DNS 规划 | `dig +short <新域名>` | 解析到目标 VIP |

```bash
# 一次性采集：现有实例的 VIP 与 VRID（用于规划新实例）
for ns in $(kubectl get gatewayproxy -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' | sort -u); do
  echo "== namespace: $ns"
  kubectl -n $ns get cm -o yaml 2>/dev/null | grep -E 'virtual_router_id|virtual_ipaddress' -A1
done
```

## 3. 路由配置（应用开发者）

> 本章命令中的 `<INSTANCE_CLASS>`、`<KEEPALIVED_VIP>`、`<HTTP_PORT>`、`<业务域名>` 请按第 1 章获取。

### 3.1 最小可用示例（标准 Ingress）

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>       # ★ 必须属于目标实例
  rules:
    - host: my-app.example.com             # ★ 替换为实际业务域名
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

```bash
kubectl apply -f my-app-ingress.yaml

# 验证（联调期用 Host 头直连 VIP，不依赖 DNS）
curl -s -H 'Host: my-app.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
```

### 3.2 路径匹配规则

| pathType | 行为 | 示例 |
|---|---|---|
| `Prefix`（常用） | 前缀匹配，控制器会生成 `/path` 与 `/path/*` 两条路由 | `/api` 命中 `/api`、`/api/user` |
| `Exact` | 精确匹配 | `/exact` 只命中 `/exact`，`/exact/sub` 返回 404 |
| `ImplementationSpecific` | 由控制器解释；配合 `use-regex` 时按正则处理 | `/users/[0-9]+` |

> 服务端 404 时先区分来源：
> - 网关 404：响应由网关生成（响应头含网关标识，且后端访问日志中无该请求）；
> - 后端 404：响应来自业务容器，后端访问日志中能看到该请求。

### 3.3 多域名、多路径、跨命名空间

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: multi
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: a.example.com
      http:
        paths:
          - {path: /api,   pathType: Prefix, backend: {service: {name: api-svc,   port: {number: 8080}}}}
          - {path: /admin, pathType: Prefix, backend: {service: {name: admin-svc, port: {number: 80}}}}
    - host: b.example.com
      http:
        paths:
          - {path: /, pathType: Prefix, backend: {service: {name: web-svc, port: {number: 80}}}}
---
# 跨命名空间后端：Ingress 与 Service 不在同一命名空间
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cross-ns
  namespace: <业务命名空间>
  annotations:
    k8s.apisix.apache.org/svc-namespace: <后端所在命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: cross.example.com
      http:
        paths:
          - {path: /, pathType: Prefix, backend: {service: {name: other-svc, port: {number: 80}}}}
```

启用 validating webhook 时，使用 `svc-namespace` 的 Ingress 在 apply 阶段可能出现 `Referenced Service '<ingress命名空间>/<服务名>' not found` 的 admission warning。这是 webhook 对跨命名空间引用检查不完整造成的提示；仍应确认目标 Service、EndpointSlice 和数据面请求正常。

### 3.4 HTTPS 与 HTTP 跳转

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-tls
  namespace: <业务命名空间>
  annotations:
    k8s.apisix.apache.org/http-to-https: "true"      # HTTP 自动跳转 HTTPS
spec:
  ingressClassName: <INSTANCE_CLASS>
  tls:
    - hosts: ["my-tls.example.com"]
      secretName: my-tls-secret                       # kubernetes.io/tls 类型
  rules:
    - host: my-tls.example.com
      http:
        paths:
          - {path: /, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}
```

```bash
# 创建证书 Secret
openssl req -x509 -newkey rsa:2048 -nodes -keyout /tmp/tls.key -out /tmp/tls.crt -days 30 \
  -subj "/CN=my-tls.example.com" -addext "subjectAltName=DNS:my-tls.example.com"
kubectl -n <业务命名空间> create secret tls my-tls-secret --cert=/tmp/tls.crt --key=/tmp/tls.key

# HTTP 跳转验证
curl -s -D - -o /dev/null -H 'Host: my-tls.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/ | grep -iE 'HTTP/|location'

# HTTPS 验证（SNI 必须正确，--resolve 的 IP 换成实际 VIP）
curl -sk --resolve my-tls.example.com:<HTTPS_PORT>:<KEEPALIVED_VIP> https://my-tls.example.com:<HTTPS_PORT>/
```

说明：

1. 控制器会把 Ingress `tls` 中引用的 Secret 自动下发给 APISIX，无需额外 CRD（除非要做特殊 SNI 或双向认证）。
2. 跳转后的 Location 通常带 HTTPS 端口号；对外发布 443 时由外层负载均衡/DNS 处理端口映射。
3. 仅当实例配置了 fallback SNI/证书时，未匹配 SNI 的 HTTPS 请求才会命中该证书；如需替换 fallback 证书按 Chart 参数处理。

### 3.5 注解速查表

当前验证构建（APISIX Ingress Controller 2.0.1-es）支持的常用 Ingress 注解如下；其他版本请以对应版本文档为准。

#### 路由与上游

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/use-regex: "true"` | path 按正则匹配（pathType 用 ImplementationSpecific） |
| `k8s.apisix.apache.org/enable-websocket: "true"` | 开启 WebSocket 升级 |
| `k8s.apisix.apache.org/upstream-scheme` | 上游协议 http/https/grpc/grpcs |
| `k8s.apisix.apache.org/upstream-retries` | 重试次数 |
| `k8s.apisix.apache.org/upstream-connect-timeout / -read-timeout / -send-timeout` | 连接/读/写超时（如 `3s`） |
| `k8s.apisix.apache.org/svc-namespace` | 跨命名空间后端 |

#### 路径重写

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/rewrite-target: /new-path` | 请求路径整体替换为目标路径 |
| `k8s.apisix.apache.org/rewrite-target-regex: "^/api/(.*)"` + `rewrite-target-regex-template: "/backend/$1"` | 正则捕获重写 |

#### 安全

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/allowlist-source-range` | IP 白名单（不在名单通常返回 403） |
| `k8s.apisix.apache.org/blocklist-source-range` | IP 黑名单 |
| `k8s.apisix.apache.org/http-allow-methods / http-block-methods` | 方法白/黑名单（拒绝返回 405） |
| `k8s.apisix.apache.org/enable-cors: "true"` + `cors-allow-origin / cors-allow-methods / cors-allow-headers` | CORS |
| `k8s.apisix.apache.org/enable-csrf: "true"` + `csrf-key` | CSRF 防护（缺 token 返回 401） |
| `k8s.apisix.apache.org/auth-type: keyAuth` | Key 认证，需 ApisixConsumer |
| `k8s.apisix.apache.org/auth-type: basicAuth` | Basic 认证，需 ApisixConsumer |
| `k8s.apisix.apache.org/auth-uri` + `auth-ssl-verify / auth-request-headers / auth-upstream-headers / auth-client-headers` | 外部认证（forward-auth） |
| `k8s.apisix.apache.org/auth-signin` | 认证失败跳转登录页（配合 auth-uri；当前发行版扩展） |

#### 响应处理

| 注解 | 说明 |
|---|---|
| `k8s.apisix.apache.org/http-to-https: "true"` | HTTP→HTTPS 跳转 |
| `k8s.apisix.apache.org/http-redirect: "/new$uri"` + `http-redirect-code` | 自定义重定向 |
| `k8s.apisix.apache.org/enable-response-rewrite: "true"` + `response-rewrite-status-code / -body / -body-base64 / -set-header / -add-header / -remove-header` | 响应改写 |
| `k8s.apisix.apache.org/custom-error-codes` | 指定错误码使用自定义错误页 |
| `k8s.apisix.apache.org/plugin-config-name: <插件配置名>` | 挂载 ApisixPluginConfig（补充能力入口） |

> `custom-error-codes`、`auth-signin` 等属于当前发行版提供的扩展能力，使用前请确认实例镜像和对应插件已启用，官方版本不一定包含。

完整清单见附录 A。

### 3.6 用 CRD 补充 Ingress（推荐组合）

原则：**路由优先用 Ingress，Ingress 未覆盖的能力用 CRD**。需要实例隔离的 CRD 写 `spec.ingressClassName` 指向目标实例；`BackendTrafficPolicy` 按目标 Service/Route 关联生效，字段要求以当前 CRD 版本为准。

| CRD | 用途 | 与 Ingress 的关系 |
|---|---|---|
| `ApisixPluginConfig` | 插件（限流、IP 限制、CORS、traffic-split 等） | Ingress 注解 `plugin-config-name` 引用 |
| `ApisixConsumer` | 认证消费者（keyAuth/basicAuth） | 配合 Ingress 的 `auth-type` 注解 |
| `BackendTrafficPolicy` | 负载均衡、会话亲和、超时重试、上游协议 | 按 Service 目标生效，无需注解 |
| `ApisixTls` | 自定义 SNI 证书（含双向认证） | 补充 Ingress TLS |
| `ApisixGlobalRule` | 全局插件（如统一响应头） | 对整个实例所有路由生效 |

```yaml
# 限流：ApisixPluginConfig + Ingress 注解引用
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: my-ratelimit
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>       # ★ 实例选择
  plugins:
    - name: limit-count
      enable: true
      config:
        count: 3
        time_window: 60
        rejected_code: 429
        key_type: var
        key: remote_addr
---
# Key 认证消费者
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata:
  name: my-consumer
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>       # ★ 实例选择
  authParameter:
    keyAuth:
      value:
        key: <自定义 key>
---
# 会话亲和（按 Cookie 哈希）
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: my-affinity
  namespace: <业务命名空间>
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: my-app                          # 对该 Service 的所有路由生效
  loadbalancer:
    type: chash                             # roundrobin / chash / ewma / least_conn
    hashOn: cookie
    key: <cookie 名>
```

排查提示：控制器会把 CRD 内容**内联合并**到数据面的 route/service 上（`ApisixPluginConfig` 不一定以独立资源出现在 `/apisix/admin/configs` 的 `plugin_configs` 中），检查是否生效要直接看对应 route/service 的 `plugins` 字段（见 5.6）。

!注意 `BackendTrafficPolicy` 的粒度是整个 Service，不是单条 Ingress 路径；同一 Service 被多个策略引用时生效顺序不确定。

### 3.7 变更生效时间与发布建议

| 动作 | 生效时间 |
|---|---|
| 修改 Ingress / CRD | 事件驱动，通常秒级；可用 `X-Digest` 确认配置版本，并用数据面请求验证 |
| 数据面 Pod 重启 | 配置在内存中，需等控制器重新下发，期间可能 404；恢复时间受组件和网络状态影响 |
| 证书更新 | 随 Ingress/ApisixTls 变更同步 |

发布建议：

1. 先 `kubectl diff -f app.yaml` 或 `kubectl apply --dry-run=server`，再正式 apply。
2. 一次只改一个维度（host / path / 注解），变更后用 5.6 的方法确认已下发。
3. 回滚 = 恢复上一版 YAML；不要手工调用 Admin API 改配置（会被控制器覆盖）。
4. 迁移切换用“先建新路由、后切 DNS”，不要先删除旧路由。

!注意 标准模式下配置是整体下发的，任一插件配置校验失败会导致同批次其它 Ingress 一并不生效（数据面日志出现 `invalid routes at index N`），变更后务必按 5.6 确认配置已更新。

---

## 4. 常见场景示例

> 以下均为通用写法，验证时把 `<KEEPALIVED_VIP>`、`<HTTP_PORT>`、域名替换为 1.2 获取的实际值。

### 4.1 HTTP 域名路由

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-basic
  namespace: <业务命名空间>
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend: {service: {name: my-app, port: {number: 80}}}
```

```bash
curl -s -H 'Host: app.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
```

### 4.2 HTTPS + HTTP 自动跳转

见 3.4。预期：HTTP 返回 301/308 且 Location 指向 HTTPS；HTTPS 返回 200 且证书与域名匹配。

### 4.3 路径重写

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-rewrite
  namespace: <业务命名空间>
  annotations:
    k8s.apisix.apache.org/rewrite-target: /new-path
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: rewrite.example.com
      http:
        paths:
          - {path: /api, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-rewrite-regex
  namespace: <业务命名空间>
  annotations:
    k8s.apisix.apache.org/rewrite-target-regex: "^/api/(.*)"
    k8s.apisix.apache.org/rewrite-target-regex-template: "/backend/$1"
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: rewrite-regex.example.com
      http:
        paths:
          - {path: /api, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}
```

```bash
# 在后端访问日志中确认收到的路径
curl -s -o /dev/null -H 'Host: rewrite.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/api/test
kubectl -n <业务命名空间> logs <后端Pod> --tail=5
# 预期：rewrite-target 场景后端收到 /new-path
#       regex 场景后端收到 /backend/test
```

### 4.4 正则路由

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/use-regex: "true"
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: regex.example.com
      http:
        paths:
          - {path: "/users/[0-9]+", pathType: ImplementationSpecific, backend: {service: {name: my-app, port: {number: 80}}}}
```

```bash
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: regex.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/users/123   # 预期 200
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: regex.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/users/abc   # 预期 404
```

### 4.5 限流

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata: {name: my-ratelimit, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
    - name: limit-count
      enable: true
      config: {count: 3, time_window: 60, rejected_code: 429, key_type: var, key: remote_addr}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ratelimit
  namespace: <业务命名空间>
  annotations:
    k8s.apisix.apache.org/plugin-config-name: my-ratelimit
spec:
  ingressClassName: <INSTANCE_CLASS>
  rules:
    - host: ratelimit.example.com
      http:
        paths:
          - {path: /, pathType: Prefix, backend: {service: {name: my-app, port: {number: 80}}}}
```

```bash
for i in 1 2 3 4 5; do
  curl -s -o /dev/null -w "%{http_code} " -H 'Host: ratelimit.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
done; echo
# 预期：200 200 200 429 429（count=3/60s）
```

上述示例使用 `limit-count` 的默认本地策略，计数按 APISIX 节点分别维护；多节点需要共享配额时，应选择并验证相应的共享存储策略。其他限流插件：`limit-req`（QPS + burst）、`limit-conn`（并发连接数），写法相同。

### 4.6 认证

**Key 认证**

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata: {name: my-consumer, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  authParameter:
    keyAuth:
      value: {key: <自定义 key>}
---
# Ingress 侧只需一个注解
metadata:
  annotations:
    k8s.apisix.apache.org/auth-type: keyAuth
```

```bash
H='Host: auth.example.com'; U=http://<KEEPALIVED_VIP>:<HTTP_PORT>/
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" $U                                  # 预期 401（未带 key）
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" -H 'apikey: <自定义 key>' $U        # 预期 200
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" -H 'apikey: wrong' $U               # 预期 401
```

**Basic 认证**

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixConsumer
metadata: {name: my-basicuser, namespace: <业务命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  authParameter:
    basicAuth:
      value: {username: <用户>, password: <口令>}
```

```bash
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: basic.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/            # 预期 401
curl -s -o /dev/null -w '%{http_code}\n' -u <用户>:<口令> -H 'Host: basic.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/  # 预期 200
```

**外部认证（forward-auth）**

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/auth-uri: "http://<认证服务地址>/verify"     # 完整 URL，认证服务返回 2xx 视为通过
    k8s.apisix.apache.org/auth-request-headers: "Authorization"        # 转发给认证服务的请求头
    k8s.apisix.apache.org/auth-upstream-headers: "X-User,X-Role"       # 认证响应头透传给后端
```

```bash
# 认证服务返回 2xx：请求放行，且认证响应头会出现在后端收到的请求里
curl -s -D - -H 'Host: fwd.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/ | grep -iE 'HTTP/|x-user'
# 认证服务返回非 2xx 时请求被拒绝，具体状态码和响应体以认证服务及插件配置为准；本环境错误认证返回 403
```

### 4.7 IP 白名单

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/allowlist-source-range: "10.0.0.0/8,192.168.0.0/16"
```

```bash
# 客户端在名单内 → 200；不在名单内 → 403，响应体 {"message":"Your IP address is not allowed"}
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: ip.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
```

### 4.8 CORS

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/enable-cors: "true"
    k8s.apisix.apache.org/cors-allow-origin: "https://allowed.example"
    k8s.apisix.apache.org/cors-allow-methods: "GET,POST"
    k8s.apisix.apache.org/cors-allow-headers: "Origin,Authorization"
```

```bash
curl -s -D - -o /dev/null -X OPTIONS \
  -H 'Host: cors.example.com' -H 'Origin: https://allowed.example' \
  -H 'Access-Control-Request-Method: GET' http://<KEEPALIVED_VIP>:<HTTP_PORT>/ | grep -i access-control
# 预期返回：Access-Control-Allow-Origin / -Methods / -Headers / -Max-Age
```

### 4.9 响应改写与自定义错误页

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/enable-response-rewrite: "true"
    k8s.apisix.apache.org/response-rewrite-status-code: "403"
    k8s.apisix.apache.org/response-rewrite-body: "Access denied"
    k8s.apisix.apache.org/response-rewrite-set-header: "X-Reason:Forbidden"
```

```bash
curl -s -D - -H 'Host: resp.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
# 预期：403 + X-Reason: Forbidden + 响应体 Access denied
```

统一错误页：如实例已部署错误页服务，可用 `plugin-config-name` 挂载对应 ApisixPluginConfig，使指定错误码返回定制页面（配合 `custom-error-codes` 注解）。这属于当前发行版扩展，使用前确认实例镜像和插件已启用。

### 4.10 重定向

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/http-redirect: "/anything$uri"
    k8s.apisix.apache.org/http-redirect-code: "308"
```

```bash
curl -s -D - -o /dev/null -H 'Host: redirect.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/x/y | grep -iE 'HTTP/|location'
# 预期：308 + Location: /anything/x/y
```

### 4.11 超时与重试

```yaml
metadata:
  annotations:
    k8s.apisix.apache.org/upstream-connect-timeout: "3s"
    k8s.apisix.apache.org/upstream-read-timeout: "10s"
    k8s.apisix.apache.org/upstream-send-timeout: "10s"
    k8s.apisix.apache.org/upstream-retries: "2"
    k8s.apisix.apache.org/upstream-scheme: "http"
```

验证方式（到数据面配置中确认）：

```bash
curl -s http://<APISIX_NODE_IP>:<ADMIN_PORT>/apisix/admin/configs -H "X-API-KEY: $ADMIN_API_KEY" \
  | jq '[.upstreams[] | select(.labels."k8s/name"=="app-timeout")][0] | {timeout, retries, scheme, nodes}'
# 预期：timeout.connect=3 / send=10 / read=10，retries=2，scheme=http
```

`upstream-retries` 控制额外尝试次数；是否切换节点取决于底层 upstream 失败类型。连接建立失败通常可触发切换，后端已建立连接后返回的 HTTP 502 不保证自动切换。

### 4.12 会话亲和（Cookie 哈希）

```yaml
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata: {name: my-affinity, namespace: <业务命名空间>}
spec:
  targetRefs:
    - {group: "", kind: Service, name: my-app}
  loadbalancer:
    type: chash
    hashOn: cookie
    key: <cookie 名>
```

```bash
for i in 1 2 3 4 5 6; do
  curl -s -H 'Host: app.example.com' -H 'Cookie: <cookie 名>=abc' http://<KEEPALIVED_VIP>:<HTTP_PORT>/ | tr -d '\n'; echo
done
# 预期：相同 Cookie 值通常命中同一后端；更换 Cookie 值可能命中另一后端
```

说明：APISIX 只按 Cookie 做哈希，**Cookie 需要由应用或 `session-cookie-hash` 之类的插件下发**。

### 4.13 全局响应头（ApisixGlobalRule）

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixGlobalRule
metadata: {name: my-global-headers, namespace: <实例命名空间>}
spec:
  ingressClassName: <INSTANCE_CLASS>
  plugins:
    - name: response-rewrite
      enable: true
      config:
        headers:
          set:
            X-Request-Start: "t=$msec"
```

```bash
curl -s -D - -o /dev/null -H 'Host: app.example.com' http://<KEEPALIVED_VIP>:<HTTP_PORT>/ | grep -i x-request-start
```

### 4.14 WebSocket / gRPC

- WebSocket：Ingress 加 `k8s.apisix.apache.org/enable-websocket: "true"`，由 APISIX 处理 Upgrade；需用实际 WebSocket 客户端验证握手和长连接。
- gRPC：`upstream-scheme: grpc` 或 `grpcs`；需要证书时用 `ApisixTls`。本环境已验证该注解可生成上游配置，协议可用性仍需用 `grpcurl` 按实际服务验证。
- 验证方式：WebSocket 用 `websocat` 或等效客户端，gRPC 用 `grpcurl` 直连网关端口；当前集群未预装这些客户端，不能仅凭 HTTP 请求判定协议成功。

### 4.15 上游异常的表现（便于定位）

| 情况 | 现象 | 定位方法 |
|---|---|---|
| Service 不存在 | 路由存在但访问 503 | `kubectl describe ingress` 显示 `<error: services "xxx" not found>`；检查 Events/admission 输出 |
| 后端无 EndpointSlice | 503 | `kubectl get endpointslice -l kubernetes.io/service-name=<svc>`；同时确认 Service 端口、协议和地址 |
| 端口写错/进程未监听 | 502 | APISIX 日志中的 upstream 错误 |
| 后端处理超时 | 504 | 调整 `upstream-read-timeout` 或排查后端 |

---

## 5. 故障排查

### 5.1 六步排障法（按顺序执行）

```text
① 看 K8s 资源    kubectl get/describe ingress、CRD status、events
        ↓ 资源正常？
② 看控制器日志    deploy/<实例>-ingress-controller -c manager / -c adc-server
        ↓ 无报错？
③ 看翻译结果     Controller Debug API（server_addr 端口，见 5.5）
        ↓ 翻译正常？
④ 看已应用配置   curl <APISIX_NODE_IP>:<ADMIN_PORT>/apisix/admin/configs  ← 核心
        ↓ 配置正确？
⑤ 看数据面网络   APISIX 日志、Keepalived、VIP、后端直连
        ↓
⑥ 定位并修复     结合 5.2 状态码速查与 5.6 配置检查定位
```

判断口诀：**“配置对不对看 Admin API，为什么没下发给看控制器日志，下发了不通看数据面。”**

### 5.2 HTTP 状态码速查

| 状态码 | 常见含义 | 优先排查 |
|---|---|---|
| 404 | 网关没有匹配到路由 | ingressClassName、host、path、实例是否已同步 |
| 404 | 后端自己的 404（后端日志有记录） | 业务应用路径 |
| 403 | IP 白名单 / forward-auth 拒绝 / response-rewrite 伪造 | 对应注解与认证服务 |
| 401 | keyAuth / basicAuth / CSRF 校验失败 | Consumer、凭据、token |
| 405 | http-allow/block-methods 拦截 | 注解 |
| 429 | limit-count / limit-req / limit-conn 触发 | 限流配置 |
| 301/308 | http-to-https / http-redirect | 跳转目标、是否循环 |
| 502 | 常见于连接被拒绝、端口错误或其他 upstream 失败 | 后端 Pod、Service targetPort、APISIX 日志 |
| 503 | 常见于没有可用 upstream 节点（Service 不存在或 EndpointSlice 为空） | Service/EndpointSlice、Pod 就绪状态 |
| 504 | 常见于 upstream 连接/读写超时 | upstream-* timeout、后端性能 |

### 5.3 第一步：检查 K8s 资源

```bash
kubectl -n <业务命名空间> get ingress <name> -o wide
kubectl -n <业务命名空间> describe ingress <name>          # 关注 Backends 与 Events
kubectl -n <业务命名空间> get ingress <name> -o jsonpath='{.status}{"\n"}'

# CRD 状态（不同 CRD/版本的 condition 类型可能不同）
kubectl -n <业务命名空间> get apisixpluginconfig,apisixconsumer,backendtrafficpolicy -o wide
kubectl -n <业务命名空间> get apisixpluginconfig <name> -o jsonpath='{.status}{"\n"}'

# 最近事件
kubectl -n <业务命名空间> get events --sort-by=.lastTimestamp | tail -20
```

判读要点：

- `status.loadBalancer.ingress` 为空 → 控制器尚未处理（class 不匹配 / 控制器异常）。
- `describe` 的 Backends 显示 `<error: services "xxx" not found>` → Service 名字或命名空间不对。
- 检查 CRD `status.conditions` 中的 `Accepted`/`Ready` 状态及 `message`；不同 CRD/版本的 condition 类型可能不同，不能只依据单一 `Accepted=True` 判断。

### 5.4 第二步：检查控制器日志

```bash
NS=<INSTANCE_NS>

# 主控制器日志
kubectl -n $NS logs deploy/<实例>-ingress-controller -c manager --tail=200 -f

# 配置生成 sidecar
kubectl -n $NS logs deploy/<实例>-ingress-controller -c adc-server --tail=200 -f

# 只筛错误
kubectl -n $NS logs deploy/<实例>-ingress-controller -c manager --tail=1000 | grep -iE 'error|failed|warn|not found'

# 前一次崩溃日志
kubectl -n $NS logs deploy/<实例>-ingress-controller -c manager --previous --tail=200
```

关键日志对照：

| 日志 | 含义 | 处理 |
|---|---|---|
| `reconciling ingress {"ingress": "xxx"}` | 正常处理流程 | — |
| `Referenced Service 'ns/svc' not found` | 常见于跨命名空间场景的 admission warning，也可能表示 Service 确实不存在 | 核对 Service 名称/命名空间，并确认 EndpointSlice |
| `adc execution failed / sync failed` | 配置生成或下发失败 | 结合 5.5/5.6 定位 |
| `failed to ...` + 资源名 | 单个资源解析失败 | 检查注解值格式 |
| 完全没有该 Ingress 的 reconcile 日志 | 控制器未监听 / class 不匹配 | 核对 IngressClass 与 controller 名称 |

### 5.5 第三步：查看控制器“内存中”的翻译结果（Debug API）

调试端口来自控制器 ConfigMap（`enable_server` / `server_addr`），仅当 `enable_server=true` 且端口已监听时可用：

```bash
kubectl -n <INSTANCE_NS> get cm <controller-cm> -o jsonpath='{.data.config\.yaml}' | grep -E 'enable_server|server_addr|metrics_addr'

# 端口转发后访问（注意列表页结尾斜杠）
kubectl -n <INSTANCE_NS> port-forward deploy/<实例>-ingress-controller 19092:<DEBUG_PORT> &
curl -s http://127.0.0.1:19092/debug/
# 返回的列表是各 GatewayProxy 名称，按其提示查看单个实例的完整翻译结果
```

用途：确认“我的 Ingress 是否被翻译进去了”。若这里没有你的路由而日志有 reconcile，说明翻译/过滤阶段有问题；若这里有、Admin API 没有，说明下发链路有问题。

### 5.6 第四步：用 Admin API 检查已应用配置（核心手段）

在本文验证的 standalone 模式下，使用 **`GET /apisix/admin/configs`** 查看当前数据面**正在使用**的全量配置；传统 etcd 模式的 Admin API 端点和权限模型不同。

```bash
APIKEY=$ADMIN_API_KEY          # 获取方法见 1.2
NODE=<APISIX_NODE_IP>          # 任一数据面节点 IP
PORT=<ADMIN_PORT>

# ① 可用性与摘要（X-Digest / X-Last-Modified 在响应头）
curl -s -D - -o /dev/null http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | grep -iE 'HTTP/|x-digest|x-last-modified'

# ② 资源数量总览
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | jq -c '{routes:(.routes|length),upstreams:(.upstreams|length),services:(.services|length),ssls:(.ssls|length),consumers:(.consumers|length),global_rules:(.global_rules|length)}'

# ③ 按 K8s 资源名查路由（labels 记录了 namespace/name/kind/controller）
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | jq '[.routes[] | select(.labels."k8s/name"=="<Ingress名>")]'

# ④ 查询该路由的上游节点/超时/重试（name 规则见下）
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | jq '[.upstreams[] | select(.name=="<namespace>_<Ingress名>_0-0")][0]'

# ⑤ 查插件是否挂上（PluginConfig 会内联到 route/service）
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | jq '[.routes[] | select(.labels."k8s/name"=="<Ingress名>") | {name, plugins}]'

# ⑥ 查证书/SNI
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" \
  | jq -c '[.ssls[]? | select(.snis != null) | {id, snis, status}]'

# ⑦ 查消费者（凭据可能明文暴露，注意终端输出安全）
curl -s http://$NODE:$PORT/apisix/admin/configs -H "X-API-KEY: $APIKEY" | jq -c '.consumers'
```

命名与结构（由控制器生成）：

```text
route.name / service.name / upstream.name ≈ <namespace>_<Ingress名>_<ruleIndex>-<pathIndex>
labels: k8s/namespace, k8s/kind, k8s/name, k8s/controller-name（+ managed-by 标记）
```

**“配置没有应用 / 不是当前配置”的判断方法**：

| 检查 | 方法 | 结论 |
|---|---|---|
| 摘要是否变化 | 变更前后对比响应头 `X-Digest` | 只能确认配置版本；仍需检查字段并执行数据面请求 |
| 配置时间 | `X-Last-Modified`（Unix 秒）与 apply 时间比较 | 仅作辅助判断，不能替代字段和业务请求验证 |
| 路由是否存在 | jq 按 `k8s/name` 过滤 | 不存在 = 未被该实例接收（class/控制器问题） |
| 字段是否为新值 | 查看 hosts/uris/plugins/upstream | 存在但字段是旧值 = 中间环节问题 |
| 多节点是否一致 | 各节点分别取 `X-Digest` | 不一致 = 某节点未同步 |
| 实例是否选错 | 看 `k8s/controller-name` label | 是另一个 controller = 查错了实例 |

```bash
# 所有数据面节点一致性一键检查（节点列表来自 kubectl）
for ip in $(kubectl -n <INSTANCE_NS> get pods -o jsonpath='{range .items[*]}{.status.podIP}{"\n"}{end}' | sort -u); do
  printf '%s ' "$ip"
  curl -s -D - -o /dev/null http://$ip:<ADMIN_PORT>/apisix/admin/configs -H "X-API-KEY: $ADMIN_API_KEY" | grep -i '^X-Digest'
done
```

> jq 提示：部分发行版 jq 未编译正则模块，`test()/match()` 会报错，请改用 `contains()` 或 `==` 过滤。

### 5.7 第五步：数据面与网络

```bash
# APISIX 访问日志 / 错误日志
kubectl -n <INSTANCE_NS> get ds -o wide  # 先确认实际 APISIX_DS_NAME
kubectl -n <INSTANCE_NS> logs ds/<APISIX_DS_NAME> --tail=50 -f
kubectl -n <INSTANCE_NS> logs ds/<APISIX_DS_NAME> --tail=200 | grep -iE 'error|warn|upstream'

# 在数据面节点本机验证网关（排除 VIP/DNS 因素）
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: <业务域名>' http://127.0.0.1:<HTTP_PORT>/

# 在集群外客户端验证 VIP
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: <业务域名>' http://<KEEPALIVED_VIP>:<HTTP_PORT>/

# VIP / Keepalived
ip addr show | grep <KEEPALIVED_VIP>
kubectl -n <INSTANCE_NS> logs ds/<KEEPALIVED_DS_NAME> --tail=50

# 后端直连（判定是网关还是后端问题；Controller 主要读取 EndpointSlice）
kubectl -n <业务命名空间> get endpointslice -l kubernetes.io/service-name=<service>
kubectl -n <业务命名空间> get endpoints <service>  # 必要时同时检查 legacy Endpoints
kubectl -n <业务命名空间> run tmp-curl --rm -it --image=<带curl的镜像> --restart=Never -- \
  curl -s -o /dev/null -w '%{http_code}\n' http://<service>.<业务命名空间>.svc.cluster.local:<port>/
```

## 6. 附录

### 附录 A：Ingress 注解全量清单（当前验证构建 2.0.1-es）

```text
k8s.apisix.apache.org/use-regex
k8s.apisix.apache.org/enable-websocket
k8s.apisix.apache.org/plugin-config-name
k8s.apisix.apache.org/upstream-scheme
k8s.apisix.apache.org/upstream-retries
k8s.apisix.apache.org/upstream-connect-timeout
k8s.apisix.apache.org/upstream-read-timeout
k8s.apisix.apache.org/upstream-send-timeout
k8s.apisix.apache.org/enable-cors
k8s.apisix.apache.org/cors-allow-origin
k8s.apisix.apache.org/cors-allow-headers
k8s.apisix.apache.org/cors-allow-methods
k8s.apisix.apache.org/enable-csrf
k8s.apisix.apache.org/csrf-key
k8s.apisix.apache.org/http-to-https
k8s.apisix.apache.org/http-redirect
k8s.apisix.apache.org/http-redirect-code
k8s.apisix.apache.org/rewrite-target
k8s.apisix.apache.org/rewrite-target-regex
k8s.apisix.apache.org/rewrite-target-regex-template
k8s.apisix.apache.org/enable-response-rewrite
k8s.apisix.apache.org/response-rewrite-status-code
k8s.apisix.apache.org/response-rewrite-body
k8s.apisix.apache.org/response-rewrite-body-base64
k8s.apisix.apache.org/response-rewrite-add-header
k8s.apisix.apache.org/response-rewrite-set-header
k8s.apisix.apache.org/response-rewrite-remove-header
k8s.apisix.apache.org/auth-uri
k8s.apisix.apache.org/auth-ssl-verify
k8s.apisix.apache.org/auth-request-headers
k8s.apisix.apache.org/auth-upstream-headers
k8s.apisix.apache.org/auth-client-headers
k8s.apisix.apache.org/auth-signin
k8s.apisix.apache.org/custom-error-codes
k8s.apisix.apache.org/allowlist-source-range
k8s.apisix.apache.org/blocklist-source-range
k8s.apisix.apache.org/http-allow-methods
k8s.apisix.apache.org/http-block-methods
k8s.apisix.apache.org/auth-type
k8s.apisix.apache.org/svc-namespace
kubernetes.io/ingress.class（旧写法，建议统一改用 spec.ingressClassName）
```

### 附录 B：常用参数在哪里看

| 想确认的参数 | 查看位置 |
|---|---|
| APISIX 端口、SSL、worker、日志格式 | 实例命名空间中 APISIX 的 ConfigMap：`kubectl -n <INSTANCE_NS> get cm <apisix-cm> -o jsonpath='{.data.config\.yaml}'` |
| 控制器同步周期、调试端口、Admin 地址 | 控制器 ConfigMap：`kubectl -n <INSTANCE_NS> get cm <controller-cm> -o jsonpath='{.data.config\.yaml}'` |
| Admin Key、Admin Service | `kubectl -n <INSTANCE_NS> get gatewayproxy <GATEWAYPROXY_NAME> -o yaml`；避免直接输出 Secret 内容 |
| VIP / VRID / 网卡 | 实例命名空间中 Keepalived 相关 ConfigMap |
| 实例使用的镜像版本 | `kubectl -n <INSTANCE_NS> get ds,deploy -o jsonpath='{...}'` |
| 网关服务端口映射 | `kubectl -n <INSTANCE_NS> get svc -o wide` |

### 附录 C：命令速查

```bash
# 列出所有实例（IngressClass）及其 GatewayProxy
kubectl get ingressclass -o wide
kubectl get gatewayproxy -A

# 查看某实例的全部路由
curl -s http://<APISIX_NODE_IP>:<ADMIN_PORT>/apisix/admin/configs -H "X-API-KEY: $ADMIN_API_KEY" \
  | jq -r '.routes[] | [.name, (.uris|join(",")), (.labels."k8s/name" // "-")] | @tsv'

# 查看 Ingress 与 class
kubectl get ingress -A -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,CLASS:.spec.ingressClassName,HOSTS:.spec.rules[*].host'

# 查看控制器翻译结果
kubectl -n <INSTANCE_NS> port-forward deploy/<实例>-ingress-controller 19092:<DEBUG_PORT> &
curl -s http://127.0.0.1:19092/debug/

# 实测某条路由（绕过 DNS）
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: <业务域名>' http://<KEEPALIVED_VIP>:<HTTP_PORT>/
```

### 附录 D：FAQ

**Q1：我应该用哪个 IngressClass？**
先用 `kubectl get ingressclass` 列出所有实例；新业务默认使用平台分配的生产实例 class，独立项目使用其专属 class。不确定时询问平台运维。

**Q2：能不能直接调 Admin API 改配置？**
不能。`PUT /apisix/admin/configs` 是控制器/ADC 写入配置的通道，手工写入会在下一次同步被覆盖，且无法审计。所有变更必须通过 Ingress/CRD。

**Q3：为什么配置改了要等一会儿才生效？**
通常是事件驱动的秒级更新。若延迟较长，按 5.4/5.5/5.6 逐层排查；数据面重启后需要控制器重新下发，恢复时间受组件和网络状态影响。

**Q4：一个 Ingress 能服务多个域名吗？**
可以，`spec.rules` 写多个 host；也可以拆成多个 Ingress 分别管理。

**Q5：多个 Ingress 写同一个 host 会怎样？**
同一实例内重复定义相同的 host+path 可能产生路由冲突或覆盖，务必避免；不同 class 属于不同实例，互不影响。

**Q6：如何确认路由走的是哪个实例？**
看数据面配置中 route 的 `labels."k8s/controller-name"`（5.6），或看 Ingress 的 `status.loadBalancer.ingress`（不同实例回填不同地址）。

**Q7：为什么 Admin API 看不到 routes 端点？**
standalone 模式下不存在 `/apisix/admin/routes`，统一使用 `/apisix/admin/configs`（5.6）。

**Q8：Admin 端口直接暴露在节点 IP 上，安全吗？**
取决于部署方式（hostNetwork 时会监听节点 IP）。建议通过网络 ACL 限制仅运维网段可访问，Admin Key 按密钥管理，不要在文档/终端记录中泄露。

### 附录 E：变更/上线自检清单（通用）

| 检查项 | 命令/方法 | 期望 |
|---|---|---|
| class 正确 | `kubectl get ingress <name> -o jsonpath='{.spec.ingressClassName}'` | 等于平台分配的实例 class |
| 资源被接收 | `kubectl get ingress <name> -o jsonpath='{.status}'` | 回填了该实例的地址 |
| CRD 状态正常 | `kubectl get <crd> <name> -o jsonpath='{.status.conditions}'` | 结合 Accepted/Ready 与 message 判断 |
| 控制器无异常 | `kubectl logs deploy/<实例>-ingress-controller -c manager | grep -i error` | 无持续报错 |
| 配置已下发 | 对比 `X-Digest`、具体字段并执行数据面请求 | 版本、配置和业务结果均符合预期 |
| 多节点一致 | 各数据面节点 `X-Digest` 对比 | 完全一致 |
| 域名解析 | `dig +short <业务域名>` | 指向 <KEEPALIVED_VIP> |
| 端到端 | `curl -H 'Host: <业务域名>' http://<KEEPALIVED_VIP>:<HTTP_PORT>/` | 业务期望响应 |
| 错误场景 | 未授权/超限/不存在路径 | 返回预期的 401/429/404 |

---

**文档维护**：本文不包含具体环境参数，若拓扑/实例发生变化，只需按第 1 章重新采集；注解与默认行为以 APISIX / APISIX Ingress Controller 对应版本文档为准。
