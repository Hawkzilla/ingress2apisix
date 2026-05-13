# ingress2apisix 使用指南

> **注意**：ingress2apisix CLI 目前仍在持续迭代中，功能和行为可能随版本更新而变化。始终使用最新版本以获得最完整的功能支持和最佳兼容性。
---

## 五种工作模式

### 1. 文件模式（默认）

将 nginx Ingress YAML 文件转换为 APISIX 兼容格式。

```bash
# 转换到标准输出
ingress2apisix -f ingress.yaml

# 转换到文件
ingress2apisix -f ingress.yaml -o apisix-ready.yaml

# 自定义命名空间、IngressClass
ingress2apisix -f ingress.yaml -o output.yaml \
  --default-namespace production \
  --ingress-class my-apisix

# 关闭 SSL 重定向注解的自动添加
ingress2apisix -f ingress.yaml --ssl-redirect=false
```

**支持的输入格式**：
- 单个 Ingress 资源
- `kind: List`（K8s 标准多资源列表）
- `kind: IngressList`（兼容旧格式）
- 多文档 YAML（`---` 分隔）

输出格式与输入格式保持一致：单个 Ingress 输入输出单个 Ingress，List 输入输出 List。

**转换后会发生什么**：
- `nginx.ingress.kubernetes.io/*` 和 `ingress.kubernetes.io/*` 注解 → `k8s.apisix.apache.org/*`
- `ingressClassName: nginx` → `ingressClassName: apisix`
- `kubernetes.io/ingress.class: nginx` 注解 → 已移除
- 限流等无原生注解的场景 → 生成 `ApisixPluginConfig` CRD
- 无法自动迁移的注解 → 在 warnings 中列出并给出迁移建议

### 2. Check 模式

扫描 Helm charts 目录中的 Ingress 资源，检测 nginx 注解并生成迁移状态报告。**不做任何修改**。

```bash
# 基本扫描
ingress2apisix --check ./charts

# 详细注解列表
ingress2apisix --check ./charts --verbose

# 输出 markdown 报告
ingress2apisix --check ./charts --check-output report.md

# 组合
ingress2apisix --check ./charts --check-output report.md --verbose
```

**报告分类**：

| 分类 | 含义 |
|---|---|
| `CONVERTED` | 可自动转为 APISIX 原生注解 |
| `PLUGIN_CONFIG` | 需生成 ApisixPluginConfig（工具自动完成） |
| `CUSTOM_PLUGIN` | 需自定义 Lua 插件（工具生成配置，需手动部署插件文件） |
| `MANUAL` | 无法自动迁移，需人工按迁移方案处理 |
| `UNKNOWN` | 未识别的注解，需确认是否需要迁移 |

### 3. Migrate 模式

原地修改 Helm charts 目录中的文件，将 nginx 注解替换为 APISIX 注解。

```bash
# 原地修改
ingress2apisix --migrate ./charts/

# 先预览，不实际修改
ingress2apisix --migrate ./charts/ --migrate-dry-run

# 修改前创建 .bak 备份文件
ingress2apisix --migrate ./charts/ --migrate-backup

# 显示详细变更信息
ingress2apisix --migrate ./charts/ --verbose

# 最安全：备份 + 预览
ingress2apisix --migrate ./charts/ --migrate-backup --migrate-dry-run
```

**Migrate 模式会做什么**：

1. **注解转换** — 文本级别替换 nginx 注解为 APISIX 注解
2. **保留 Helm 模板语法** — `{{ .Values.xxx }}` 等 Go 模板表达式原样保留
3. **ingressClassName** — `nginx` → `apisix`
4. **移除旧注解** — `kubernetes.io/ingress.class: nginx`
5. **生成 PluginConfig** — 在 `templates/apisix-plugin-configs.yaml` 生成 `ApisixPluginConfig` Helm 模板
6. **更新 values.yaml** — 为插件参数添加默认值
7. **手动标注** — 对无法自动迁移的注解添加 `# TODO [ingress2apisix]` 注释

**迁移前后对比**：

迁移前：
```yaml
metadata:
  name: {{ .Release.Name }}-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/limit-rps: {{ .Values.rateLimit }}
    nginx.ingress.kubernetes.io/affinity: "cookie"
spec:
  ingressClassName: nginx
```

迁移后：
```yaml
metadata:
  name: {{ .Release.Name }}-ingress
  annotations:
    k8s.apisix.apache.org/http-to-https: "true"
    # TODO [ingress2apisix]: affinity — 需 BackendTrafficPolicy CRD，参见迁移文档 4.1.1
    k8s.apisix.apache.org/plugin-config-name: {{ .Release.Name }}-ingress-plugins
spec:
  ingressClassName: apisix
```

同时生成 `templates/apisix-plugin-configs.yaml`：
```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: {{ .Release.Name }}-ingress-plugins
  labels:
    managed-by: ingress2apisix
spec:
  plugins:
    - name: limit-req
      enable: true
      config:
        rate: {{ .Values.rateLimit }}
        burst: 0
        key: remote_addr
        rejected_code: 429
```

### 4. 集群模式

通过 client-go 直接操作 Kubernetes 集群中的 Ingress 资源。

```bash
# 预览（不实际修改）
ingress2apisix --apply --dry-run

# 指定命名空间
ingress2apisix --apply --namespace production

# 全部命名空间
ingress2apisix --apply

# 自定义 kubeconfig 和 context
ingress2apisix --apply --kubeconfig /path/to/kubeconfig --context my-cluster

# 预览 + 自定义 kubeconfig
ingress2apisix --apply --dry-run --kubeconfig ~/.kube/staging-config
```

**集群模式流程**：
1. 从集群读取 Ingress 资源（指定 namespace 或全部）
2. 转换 nginx 注解为 APISIX 注解
3. PATCH 更新 Ingress 的 annotations 和 ingressClassName
4. 创建或更新 `ApisixPluginConfig` CRD（如果需要插件）

### 5. Web UI 模式

提供浏览器界面，支持 YAML 编辑、文件上传、注解检查和实时对比。

```bash
# 启动（默认 localhost:8080）
ingress2apisix --web

# 自定义地址
ingress2apisix --web --web-addr 0.0.0.0:3000
```

浏览器打开后有 4 个标签页：

| 标签页 | 功能 |
|---|---|
| **Convert** | 左右双编辑器，左侧粘贴 nginx Ingress YAML，右侧实时显示转换结果 |
| **Check** | 粘贴 Ingress YAML，查看注解迁移状态分类统计 |
| **Migrate** | 粘贴 Helm 模板 YAML，对比迁移前后差异 |
| **Docs** | 查看迁移指南、APISIX 注解参考、本使用文档 |

---

## 命令行参数速查

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-f` | *(文件模式必填)* | 输入 Ingress YAML 文件路径 |
| `-o` | stdout | 输出文件路径 |
| `--default-namespace` | `default` | 资源无命名空间时使用的默认值 |
| `--api-version` | `apisix.apache.org/v2` | APISIX CRD API 版本 |
| `--ingress-class` | `apisix` | 输出 Ingress 的 ingressClassName |
| `--ssl-redirect` | `true` | TLS 主机自动添加 http-to-https 注解 |
| `--check` | | Check 模式：扫描目录 |
| `--check-output` | stdout | Check 报告输出文件（markdown） |
| `--verbose` | `false` | 显示详细注解列表 |
| `--migrate` | | Migrate 模式：原地修改目录 |
| `--migrate-dry-run` | `false` | 预览迁移变更 |
| `--migrate-backup` | `false` | 修改前创建 .bak 备份 |
| `--apply` | `false` | 集群模式 |
| `--dry-run` | `false` | 集群模式仅预览 |
| `--kubeconfig` | `~/.kube/config` | kubeconfig 路径 |
| `--context` | 当前 context | Kubernetes context |
| `--namespace` | 全部 | 限定命名空间 |
| `--web` | `false` | 启动 Web UI |
| `--web-addr` | `localhost:8080` | Web UI 监听地址 |
| `--version` | | 显示版本号 |

---

## 典型工作流

### 场景 1：评估迁移复杂度

先用 Check 模式扫描，了解有多少注解可自动迁移：

```bash
ingress2apisix --check ./my-charts/ --check-output migration-report.md --verbose
```

查看 `migration-report.md` 中的 `MANUAL` 和 `UNKNOWN` 条目，决定迁移策略。

### 场景 2：批量迁移 Helm Charts

```bash
# 第一步：备份
ingress2apisix --migrate ./my-charts/ --migrate-backup --migrate-dry-run

# 第二步：确认无误后执行
ingress2apisix --migrate ./my-charts/ --migrate-backup

# 第三步：手动处理 TODO 标注的项
grep -r "TODO.*ingress2apisix" ./my-charts/
```

### 场景 3：逐步迁移集群

```bash
# 第一步：预览所有命名空间
ingress2apisix --apply --dry-run

# 第二步：先迁移测试命名空间
ingress2apisix --apply --namespace test-env --dry-run

# 第三步：确认后 apply
ingress2apisix --apply --namespace test-env

# 第四步：逐个命名空间迁移
ingress2apisix --apply --namespace staging
ingress2apisix --apply --namespace production
```

### 场景 4：快速转换单个文件

```bash
# 转换并查看
ingress2apisix -f old-ingress.yaml

# 转换并保存
ingress2apisix -f old-ingress.yaml -o new-ingress.yaml
```

---

