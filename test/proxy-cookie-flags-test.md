# proxy-cookie-flags 插件测试指南

## 一、准备测试后端

部署一个会返回 Set-Cookie 的后端服务：

```bash
kubectl apply -f test/cookie-backend.yaml
```

验证后端自身返回的 Cookie（无任何修饰）：

```bash
kubectl port-forward svc/cookie-backend 8888:80 &
curl -v http://localhost:8888/cookies/set?sessionid=abc123 2>&1 | grep -i set-cookie
```

预期（原样返回，无额外属性）：
```
< Set-Cookie: sessionid=abc123; Path=/
```

---

## 二、部署 APISIX 资源

```bash
kubectl apply -f test/test-cookie-flags.yaml
```

这会创建：
1. `ApisixPluginConfig/cookie-flags-config` — 启用 proxy-cookie-flags 插件，配置 3 条规则
2. `Ingress/cookie-test` — 引用 plugin-config，绑定后端服务

验证：

```bash
kubectl get apisixpluginconfig cookie-flags-config -o yaml
kubectl get ingress cookie-test -o yaml
```

---

## 三、功能测试

### 测试 1：sessionid → SameSite=None + Secure

```bash
curl -v -H "Host: cookie.test.local" \
  http://<APISIX_EXTERNAL_IP>/cookies/set?sessionid=abc123 2>&1 | grep -i set-cookie
```

**预期：**
```
< Set-Cookie: sessionid=abc123; Path=/; SameSite=None; Secure
```

### 测试 2：auth_token → HttpOnly + Secure

```bash
curl -v -H "Host: cookie.test.local" \
  http://<APISIX_EXTERNAL_IP>/cookies/set?auth_token=xyz789 2>&1 | grep -i set-cookie
```

**预期：**
```
< Set-Cookie: auth_token=xyz789; Path=/; HttpOnly; Secure
```

### 测试 3：其他 cookie（通配符 *）→ SameSite=Lax

```bash
curl -v -H "Host: cookie.test.local" \
  http://<APISIX_EXTERNAL_IP>/cookies/set?tracker=t1 2>&1 | grep -i set-cookie
```

**预期：**
```
< Set-Cookie: tracker=t1; Path=/; SameSite=Lax
```

### 测试 4：同一次请求返回多个 Cookie

```bash
curl -v -H "Host: cookie.test.local" \
  "http://<APISIX_EXTERNAL_IP>/response-headers?Set-Cookie=sessionid=abc,auth_token=xyz,tracker=t1" \
  2>&1 | grep -i set-cookie
```

**预期（每个 cookie 独立匹配规则）：**
```
< Set-Cookie: sessionid=abc; SameSite=None; Secure
< Set-Cookie: auth_token=xyz; HttpOnly; Secure
< Set-Cookie: tracker=t1; SameSite=Lax
```

### 测试 5：替换已有 SameSite 属性

```bash
# 后端返回 SameSite=Strict，插件应替换为 SameSite=None
curl -v -H "Host: cookie.test.local" \
  "http://<APISIX_EXTERNAL_IP>/response-headers?Set-Cookie=sessionid=abc;SameSite=Strict" \
  2>&1 | grep -i set-cookie
```

**预期：**
```
< Set-Cookie: sessionid=abc; SameSite=None; Secure
```

---

## 四、故障排查

| 现象 | 排查方向 |
|---|---|
| Cookie 完全没变化 | `kubectl get apisixpluginconfig` 确认插件已启用；检查 APISIX error.log |
| 插件报 schema 错误 | 检查 rules 数组格式，确保 match 和 flags 都存在 |
| 只有部分 cookie 被修改 | 规则按顺序匹配，第一个匹配的规则生效，检查 match 顺序 |
| SameSite 未替换 | 插件逻辑是先删旧 SameSite 再加新的，检查 Lua 代码 |
| APISIX 未接管请求 | 确认 ingressClassName: apisix，确认 ApisIX Ingress Controller 正在运行 |

---

## 五、APISIX 原生调试

直接查看 APISIX 侧的路由和插件配置：

```bash
# 列出所有路由
curl -s http://<APISIX_ADMIN_IP>:9180/apisix/admin/routes -H "X-API-KEY: your-key" | jq

# 查看具体路由的插件配置
curl -s http://<APISIX_ADMIN_IP>:9180/apisix/admin/routes/<route_id> -H "X-API-KEY: your-key" | jq '.value.plugins'

# 查看 APISIX error.log
kubectl logs -n <apisix-namespace> <apisix-pod> -c apisix | grep -i cookie-flags
```
