# Ingress Annotation Migration Check

- Scanned files: 4
- Ingress files: 2
- Auto-converted: 8
- Manual required: 5
- Unknown: 1

## Files with Issues

| File | Annotation | Status | Migration Guide |
|---|---|---|---|
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/affinity` | MANUAL | 需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/auth-tls-verify-client` | MANUAL | 需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/custom-http-errors` | MANUAL | 需自定义 Lua 插件 (custom-error-page)，参见迁移文档 4.1.3 |
| `other-svc/templates/ingress.yaml` | `ingress.kubernetes.io/proxy-body-size` | MANUAL | 需全局配置 nginx_config.http.client_max_body_size，参见迁移文档 3.1 |
| `other-svc/templates/ingress.yaml` | `nginx.ingress.kubernetes.io/proxy-buffer-size` | MANUAL | 需全局配置 nginx_config.http_configuration_snippet.proxy_buffer_size，参见迁移文档 3.1 |
| `other-svc/templates/ingress.yaml` | `nginx.ingress.kubernetes.io/some-custom-thing` | UNKNOWN | 未识别的注解，请手动确认迁移方案 |

## Full Report

| File | Annotation | Status | Migration |
|---|---|---|---|
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/affinity` | MANUAL | 需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/auth-tls-verify-client` | MANUAL | 需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/configuration-snippet` | PLUGIN_CONFIG | → 单条 rewrite 用原生注解，多条用 ApisixPluginConfig + proxy-rewrite |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/custom-http-errors` | MANUAL | 需自定义 Lua 插件 (custom-error-page)，参见迁移文档 4.1.3 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/enable-cors` | CONVERTED | → k8s.apisix.apache.org/enable-cors |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/limit-rps` | PLUGIN_CONFIG | → ApisixPluginConfig + limit-req 插件 |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/proxy-connect-timeout` | CONVERTED | → k8s.apisix.apache.org/upstream-connect-timeout |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/proxy-cookie-path` | CUSTOM_PLUGIN | → ApisixPluginConfig + proxy-cookie-path 自定义 Lua 插件 (需部署 plugins/proxy-cookie-path.lua) |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/rewrite-target` | CONVERTED | → k8s.apisix.apache.org/rewrite-target 或 rewrite-target-regex |
| `my-app/templates/ingress.yaml` (Helm) | `nginx.ingress.kubernetes.io/ssl-redirect` | CONVERTED | → k8s.apisix.apache.org/http-to-https |
| `other-svc/templates/ingress.yaml` | `ingress.kubernetes.io/proxy-body-size` | MANUAL | 需全局配置 nginx_config.http.client_max_body_size，参见迁移文档 3.1 |
| `other-svc/templates/ingress.yaml` | `nginx.ingress.kubernetes.io/proxy-buffer-size` | MANUAL | 需全局配置 nginx_config.http_configuration_snippet.proxy_buffer_size，参见迁移文档 3.1 |
| `other-svc/templates/ingress.yaml` | `nginx.ingress.kubernetes.io/some-custom-thing` | UNKNOWN | 未识别的注解，请手动确认迁移方案 |
| `other-svc/templates/ingress.yaml` | `nginx.ingress.kubernetes.io/whitelist-source-range` | CONVERTED | → k8s.apisix.apache.org/allowlist-source-range |
