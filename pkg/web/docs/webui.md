# Web UI 使用指南

本文档介绍 ingress2apisix Web UI 的操作方式。

---

## 页面布局

页面顶部有 **4 个标签页**，分别对应不同的功能：

| 标签 | 功能 |
|---|---|
| **Convert** | 将 nginx Ingress YAML 转换为 APISIX 兼容格式 |
| **Check** | 分析 Ingress 中的注解，查看迁移状态分类 |
| **Migrate** | 对 Helm 模板做迁移前后对比 |
| **Docs** | 查看迁移指南、注解参考、使用文档 |

每个标签页都有 **左右两个编辑器**，左侧是输入，右侧是输出（只读），形成直观的对比视图。

---

## Convert 标签页

**用途**：粘贴或上传 nginx Ingress YAML，实时得到转换后的 APISIX 兼容 YAML。

### 操作步骤

1. **输入**：在左侧编辑器中粘贴 YAML，或点击 **Upload** 按钮上传 `.yaml` / `.yml` 文件
2. **配置选项**：
   - **SSL Redirect** 复选框（默认开启）：是否为 TLS 主机自动添加 `http-to-https` 注解
3. **点击 `> Convert` 按钮**
4. **查看结果**：右侧编辑器显示转换后的 YAML
5. **复制**：点击右侧的 **Copy** 按钮一键复制结果

### 输出说明

- `ingressClassName` 已改为 `apisix`
- nginx 注解已替换为 `k8s.apisix.apache.org/*`
- `kubernetes.io/ingress.class: nginx` 已移除
- 如果有需要插件的注解（如限流），会生成 `ApisixPluginConfig` 并在底部面板显示
- 如果有无法自动迁移的注解，底部面板会显示警告列表

### 快捷操作

- **Load Example**：加载示例 Ingress YAML
- **Clear**：清空输入和输出

---

## Check 标签页

**用途**：分析 Ingress YAML 中的 nginx 注解，查看每个注解的迁移状态分类。

### 操作步骤

1. 在左侧编辑器粘贴或上传 Ingress YAML
2. 点击 **Check Annotations** 按钮
3. 右侧面板显示分类统计报告

### 分类说明

| 分类 | 含义 |
|---|---|
| **CONVERTED** | 可自动转为 APISIX 原生注解，无需人工干预 |
| **PLUGIN_CONFIG** | 需生成 `ApisixPluginConfig`，工具自动完成 |
| **CUSTOM_PLUGIN** | 需自定义 Lua 插件（如 proxy-cookie-path），需手动部署插件文件 |
| **MANUAL** | 无法自动迁移，需人工按迁移方案处理（如会话亲和、mTLS） |
| **UNKNOWN** | 未识别的注解，需手动确认迁移方案 |

### 结果解读

- 如果显示 **"All annotations can be automatically migrated!"**，说明全部可自动迁移
- 如果显示 **"Some annotations need attention"**，底部面板会列出详细信息
- 底部状态栏显示数量统计标签

---

## Migrate 标签页

**用途**：对 Helm 模板 YAML 做迁移预览，查看迁移前后的差异对比。

### 操作步骤

1. 在左侧编辑器粘贴 Helm 模板 YAML（可包含 `{{ .Values.xxx }}` 等 Go 模板语法）
2. 或点击 **Upload Charts** 按钮上传多个 Helm chart 文件
3. 点击 **Migrate** 按钮
4. 右侧编辑器显示迁移后的 YAML

### 注意事项

- Go 模板语法（`{{ .Release.Name }}` 等）会被原样保留
- 无法自动迁移的注解会添加 `# TODO [ingress2apisix]` 注释
- 如果需要 `ApisixPluginConfig`，会在输出末尾显示生成的配置
- 底部面板显示迁移报告和警告

### 快捷操作

- **Load Example**：加载示例 Helm 模板
- **Upload Charts**：支持多文件上传（`.yaml` / `.yml` / `.tpl`）
- 已上传的文件以标签形式显示，可点击 `×` 移除

---

## Docs 标签页

**用途**：在页面内直接查阅相关文档，无需离开界面。

### 文档列表

| 按钮 | 内容 |
|---|---|
| **迁移指南** | nginx → APISIX 完整迁移方案，含平滑迁移等价表和非平滑迁移场景 |
| **使用指南** | 本文档，介绍 Web UI 和 CLI 的操作方式 |
| **APISIX 注解参考** | APISIX Ingress Controller 全部原生注解的详细说明和示例 |

### 操作

- 点击顶部按钮切换文档
- 左侧目录自动生成，点击可跳转到对应章节
- 顶部搜索框可过滤章节标题
- 文档支持代码块、表格、引用块等完整 Markdown 渲染

---

## 通用操作

### 编辑器

- 基于 CodeMirror，支持 YAML 语法高亮
- 支持行号显示
- 支持自动换行

### 文件上传

- 支持拖拽上传（Convert 和 Check 标签页）
- 支持点击 Upload 按钮选择文件
- 支持 `.yaml` / `.yml` 格式，Migrate 标签页额外支持 `.tpl`

### 底部面板

- Convert 标签页：显示警告和错误信息
- Check 标签页：显示完整报告详情
- Migrate 标签页：显示迁移报告
- 可点击 **Close** 按钮收起

### 状态栏

页面底部的状态栏显示当前操作状态和统计信息：
- 绿色标签：成功
- 黄色标签：有警告
- 红色标签：出错
- 蓝色/紫色标签：数量统计
