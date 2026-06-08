<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useConverterStore } from '@/stores/converter'
import YamlEditor from '@/components/common/YamlEditor.vue'

const store = useConverterStore()
const inputEditor = ref<InstanceType<typeof YamlEditor> | null>(null)

function highlightYaml(yaml: string): string {
  if (!yaml) return ''
  return yaml
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Comments
    .replace(/(#[^\n]*)/g, '<span class="hl-comment">$1</span>')
    // Keys
    .replace(/^(\s*)([\w][\w\s./-]*?)(:)/gm, '$1<span class="hl-key">$2</span>$3')
    // String values (after colon)
    .replace(/(:\s*)([|>]?['"][^'"]*['"])/g, '$1<span class="hl-string">$2</span>')
    // Boolean / null
    .replace(/(:\s*)(true|false|null|~)(?=\s|$)/gm, '$1<span class="hl-bool">$2</span>')
    // Numbers
    .replace(/(:\s*)(\d[\d.]*)(?=\s|$)/gm, '$1<span class="hl-number">$2</span>')
    // Dashes (list items)
    .replace(/^(\s*)(- )/gm, '$1<span class="hl-list">$2</span>')
    // Anchors / aliases
    .replace(/(&\w[\w-]*)/g, '<span class="hl-anchor">$1</span>')
    .replace(/(\*\w[\w-]*)/g, '<span class="hl-anchor">$1</span>')
    // --- separators
    .replace(/^(---)/gm, '<span class="hl-sep">$1</span>')
}

const EXAMPLE_YAML = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ingress
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - path: /api(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: api-svc
            port:
              number: 80
`

const isDragging = ref(false)
const dividerPos = ref(40)
const panelRef = ref<HTMLDivElement | null>(null)

function loadExample() {
  store.inputYaml = EXAMPLE_YAML
}

function copyOutput() {
  if (store.outputYaml) navigator.clipboard.writeText(store.outputYaml)
}

function downloadOutput() {
  if (!store.outputYaml) return
  const blob = new Blob([store.outputYaml], { type: 'text/yaml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'apisix-output.yaml'
  a.click()
  URL.revokeObjectURL(url)
}

function handleFileUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => { store.inputYaml = reader.result as string }
  reader.readAsText(file)
  ;(e.target as HTMLInputElement).value = ''
}

function handleDrop(e: DragEvent) {
  const file = e.dataTransfer?.files[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => { store.inputYaml = reader.result as string }
  reader.readAsText(file)
}

function startDrag(e: MouseEvent) {
  isDragging.value = true
  const startX = e.clientX
  const startPercent = dividerPos.value
  const container = panelRef.value
  if (!container) return
  const onMove = (ev: MouseEvent) => {
    const dx = ev.clientX - startX
    dividerPos.value = Math.min(80, Math.max(20, startPercent + (dx / container.offsetWidth) * 100))
  }
  const onUp = () => {
    isDragging.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

function handleKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault()
    store.convert()
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'X') {
    e.preventDefault()
    store.clear()
  }
}

onMounted(() => document.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', handleKeydown))
</script>

<template>
  <div class="convert-page" @dragover.prevent @drop.prevent="handleDrop">
    <!-- Toolbar -->
    <div class="toolbar">
      <button class="glass-btn glass-btn--primary" :disabled="store.loading" @click="store.convert()">
        <template v-if="store.loading">
          <span class="spinner" /> Converting...
        </template>
        <template v-else>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3" /></svg>
          Convert
        </template>
      </button>

      <div class="toolbar-group">
        <label class="toolbar-label">Target</label>
        <select v-model="store.target" class="glass-select">
          <option value="APISIX">APISIX</option>
          <option value="GatewayAPI">Gateway API</option>
        </select>
      </div>

      <label class="glass-toggle toolbar-toggle">
        <input type="checkbox" v-model="store.sslRedirect" />
        <span class="glass-toggle__track"><span class="glass-toggle__thumb" /></span>
        <span class="toggle-label">SSL Redirect</span>
      </label>

      <div class="toolbar-spacer" />

      <label class="glass-btn glass-btn--ghost glass-btn--sm">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg>
        Upload
        <input type="file" accept=".yaml,.yml" style="display:none" @change="handleFileUpload" />
      </label>
      <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="loadExample">Load Example</button>
      <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="store.clear()">Clear</button>
    </div>

    <!-- Disclaimer -->
    <div class="disclaimer-bar">
      <span class="disclaimer-icon">&#9888;&#65039;</span>
      <span>当前转换结果仅供参考，生产环境请以最终测试验证结果为准</span>
    </div>

    <!-- Editor panels -->
    <div ref="panelRef" class="editor-panels" :class="{ dragging: isDragging }">
      <div class="editor-pane input-pane" :style="{ width: dividerPos + '%' }">
        <div class="pane-header">
          <span class="pane-dot green" />
          <span class="pane-label">Input (Ingress YAML)</span>
        </div>
        <YamlEditor
          ref="inputEditor"
          v-model="store.inputYaml"
          placeholder="Paste your Kubernetes Ingress YAML here..."
        />
      </div>

      <div class="divider" @mousedown="startDrag"><div class="divider-line" /></div>

      <div class="editor-pane output-pane" :style="{ width: (100 - dividerPos) + '%' }">
        <div class="pane-header">
          <span class="pane-dot blue" />
          <span class="pane-label">Output ({{ store.target }} YAML)</span>
          <div class="pane-actions">
            <button class="icon-btn" title="Copy" @click="copyOutput">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
            </button>
            <button class="icon-btn" title="Download" @click="downloadOutput">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
            </button>
          </div>
        </div>
        <div class="output-pre" :class="{ empty: !store.outputYaml }"><pre v-if="store.outputYaml" v-html="highlightYaml(store.outputYaml)"></pre><pre v-else class="output-placeholder">Converted YAML will appear here...</pre></div>
      </div>
    </div>

    <!-- Error panel -->
    <div v-if="store.error" class="error-panel">
      <span class="error-icon">!</span>
      <span>{{ store.error }}</span>
    </div>

    <!-- Warnings -->
    <div v-if="store.warnings.length > 0" class="warning-panel">
      <div v-for="(w, i) in store.warnings" :key="i" class="warning-item">
        <span class="warning-icon">&#9888;</span>
        <span>{{ w }}</span>
      </div>
    </div>

    <!-- Status bar -->
    <div v-if="store.summary || store.warnings.length > 0" class="status-bar">
      <span v-if="store.summary" class="stat">{{ store.summary }}</span>
      <span class="toolbar-spacer" />
      <span class="shortcut-hint">&#8984;/Ctrl+Enter to convert</span>
    </div>
  </div>
</template>

<style scoped>
.convert-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 48px);
  padding: 12px 20px;
  gap: 10px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  background: var(--bg-card);
  backdrop-filter: var(--glass-blur-light);
  -webkit-backdrop-filter: var(--glass-blur-light);
  border: 0.5px solid var(--border);
  border-radius: 10px;
  flex-wrap: wrap;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.toolbar-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-weight: 500;
}

.toolbar-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toggle-label {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.toolbar-spacer { flex: 1; }

.editor-panels {
  display: flex;
  flex: 1;
  min-height: 0;
  border-radius: 10px;
  overflow: hidden;
  border: 0.5px solid var(--border);
  background: var(--bg-card);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  box-shadow: var(--shadow-card);
}

.editor-panels.dragging {
  cursor: col-resize;
  user-select: none;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  min-width: 200px;
  overflow: hidden;
}

.pane-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: var(--bg-card);
  border-bottom: 0.5px solid var(--border);
  font-size: 0.82rem;
  font-weight: 500;
  flex-shrink: 0;
}

.pane-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.pane-dot.green { background: var(--green); }
.pane-dot.blue { background: var(--accent); }
.pane-label { flex: 1; }

.pane-actions {
  display: flex;
  gap: 4px;
}

.icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.icon-btn:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.divider {
  width: 6px;
  cursor: col-resize;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--bg-card);
  border-left: 0.5px solid var(--border);
  border-right: 0.5px solid var(--border);
  transition: background 0.15s;
}

.divider:hover { background: var(--accent); }

.divider-line {
  width: 2px;
  height: 32px;
  border-radius: 1px;
  background: var(--border);
}

.divider:hover .divider-line { background: rgba(255,255,255,0.3); }

.error-panel {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(255,69,58,0.1);
  border: 1px solid rgba(255,69,58,0.2);
  border-radius: 8px;
  color: var(--red);
  font-size: 0.85rem;
}

.error-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--red);
  color: #fff;
  font-size: 0.7rem;
  font-weight: 700;
  flex-shrink: 0;
}

.warning-panel {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 14px;
  background: rgba(255,214,10,0.08);
  border: 1px solid rgba(255,214,10,0.15);
  border-radius: 8px;
  max-height: 120px;
  overflow-y: auto;
}

.warning-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 0.82rem;
  color: var(--yellow);
}

.warning-icon { font-size: 0.9rem; flex-shrink: 0; }

.status-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 14px;
  background: var(--bg-card);
  backdrop-filter: var(--glass-blur-light);
  -webkit-backdrop-filter: var(--glass-blur-light);
  border: 0.5px solid var(--border);
  border-radius: 8px;
  font-size: 0.8rem;
}

.stat { color: var(--text-muted); }

.stat-num {
  font-weight: 600;
  font-family: var(--font-mono);
  color: var(--accent);
}

.shortcut-hint {
  color: var(--text-muted);
  font-size: 0.75rem;
  opacity: 0.6;
}

.spinner {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

.output-pre {
  flex: 1;
  margin: 0;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  line-height: 1.6;
}

.output-pre pre {
  margin: 0;
  padding: 14px;
  background: var(--bg-input);
  color: var(--text);
  white-space: pre;
  tab-size: 2;
}

.output-pre.empty pre {
  color: var(--text-muted);
  font-style: italic;
  font-family: var(--font-sans);
}

.output-placeholder {
  color: var(--text-muted);
  font-style: italic;
  font-family: var(--font-sans);
  margin: 0;
  padding: 14px;
}

/* YAML syntax highlighting */
.output-pre :deep(.hl-key) {
  color: #9b59b6;
  font-weight: 500;
}

[data-theme="light"] .output-pre :deep(.hl-key) {
  color: #8e44ad;
}

.output-pre :deep(.hl-string) {
  color: #e67e22;
}

[data-theme="light"] .output-pre :deep(.hl-string) {
  color: #d35400;
}

.output-pre :deep(.hl-comment) {
  color: #636e72;
  font-style: italic;
}

[data-theme="light"] .output-pre :deep(.hl-comment) {
  color: #95a5a6;
}

.output-pre :deep(.hl-bool) {
  color: #e74c3c;
  font-weight: 500;
}

[data-theme="light"] .output-pre :deep(.hl-bool) {
  color: #c0392b;
}

.output-pre :deep(.hl-number) {
  color: #00b894;
  font-weight: 500;
}

[data-theme="light"] .output-pre :deep(.hl-number) {
  color: #00a884;
}

.output-pre :deep(.hl-list) {
  color: #00cec9;
}

[data-theme="light"] .output-pre :deep(.hl-list) {
  color: #0984e3;
}

.output-pre :deep(.hl-anchor) {
  color: #fdcb6e;
  text-decoration: underline;
}

.output-pre :deep(.hl-sep) {
  color: #636e72;
  font-weight: 700;
}

[data-theme="light"] .output-pre :deep(.hl-sep) {
  color: #95a5a6;
}

.disclaimer-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: rgba(255,159,10,0.08);
  border: 0.5px solid rgba(255,159,10,0.2);
  border-radius: 8px;
  font-size: 0.8rem;
  color: var(--orange);
  font-weight: 500;
  flex-shrink: 0;
}

[data-theme="light"] .disclaimer-bar {
  background: rgba(255,149,0,0.06);
  border-color: rgba(255,149,0,0.15);
}

.disclaimer-icon {
  font-size: 15px;
  flex-shrink: 0;
}
</style>
