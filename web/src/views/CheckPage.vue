<script setup lang="ts">
import { ref, computed } from 'vue'
import { api } from '@/api/client'
import YamlEditor from '@/components/common/YamlEditor.vue'

interface CheckResponse {
  success: boolean
  converted: number
  pluginConfig: number
  customPlugin: number
  manual: number
  unknown: number
  totalFiles: number
  ingressFiles: number
  hasManual: boolean
  hasUnknown: boolean
  warnings: string[]
  files: Array<{
    path: string
    hasHelmTpl: boolean
    findings: Array<{
      annotation: string
      status: string
      detail: string
    }>
  }>
}

const inputYaml = ref('')
const loading = ref(false)
const result = ref<CheckResponse | null>(null)
const error = ref('')
const searchText = ref('')

const EXAMPLE_YAML = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ingress
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/auth-url: "http://auth.example.com/verify"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: api-svc
            port:
              number: 80
`

function loadExample() {
  inputYaml.value = EXAMPLE_YAML
}

async function doCheck() {
  if (!inputYaml.value.trim()) return
  loading.value = true
  error.value = ''
  result.value = null
  try {
    result.value = await api.post<CheckResponse>('/api/check', { yaml: inputYaml.value })
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function handleFileUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => { inputYaml.value = reader.result as string }
  reader.readAsText(file)
  ;(e.target as HTMLInputElement).value = ''
}

const statusColors: Record<string, string> = {
  auto: 'green', converted: 'green',
  plugin: 'cyan', pluginConfig: 'cyan',
  'aic-native': 'purple',
  manual: 'yellow',
  unsupported: 'red',
  customPlugin: 'purple',
  unknown: 'blue',
}

const statusLabels: Record<string, string> = {
  auto: 'Auto', converted: 'Converted',
  plugin: 'Plugin', pluginConfig: 'Plugin',
  'aic-native': 'AIC Native',
  manual: 'Manual',
  unsupported: 'Unsupported',
  customPlugin: 'Custom Plugin',
  unknown: 'Unknown',
}

const allFindings = computed(() => {
  if (!result.value?.files) return []
  const items: Array<{ path: string; annotation: string; status: string; detail: string }> = []
  for (const f of result.value.files) {
    for (const finding of f.findings) {
      items.push({ path: f.path, ...finding })
    }
  }
  return items
})

const filteredFindings = computed(() => {
  if (!searchText.value.trim()) return allFindings.value
  const q = searchText.value.toLowerCase()
  return allFindings.value.filter(
    (f) => f.annotation.toLowerCase().includes(q) || f.path.toLowerCase().includes(q) || f.detail.toLowerCase().includes(q)
  )
})
</script>

<template>
  <div class="check-page">
    <div class="toolbar">
      <button class="glass-btn glass-btn--primary" :disabled="loading" @click="doCheck">
        <template v-if="loading"><span class="spinner" /> Checking...</template>
        <template v-else>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></svg>
          Check Annotations
        </template>
      </button>
      <div class="toolbar-spacer" />

      <label class="glass-btn glass-btn--ghost glass-btn--sm">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg>
        Upload
        <input type="file" accept=".yaml,.yml,.json" style="display:none" @change="handleFileUpload" />
      </label>
      <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="loadExample">Load Example</button>
      <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="inputYaml = ''; result = null; error = ''">Clear</button>
    </div>

    <div class="check-layout">
      <div class="input-section">
        <div class="pane-header">
          <span class="pane-dot green" />
          <span class="pane-label">Input</span>
        </div>
        <YamlEditor v-model="inputYaml" placeholder="Paste Ingress YAML or JSON here..." />
      </div>

      <div class="report-section">
        <div class="pane-header">
          <span class="pane-dot blue" />
          <span class="pane-label">Annotation Report</span>
          <div v-if="result" class="report-search">
            <input v-model="searchText" class="search-input glass-input" placeholder="Filter..." />
          </div>
        </div>

        <div v-if="error" class="error-msg">{{ error }}</div>

        <div v-if="result" class="report-body">
          <!-- Summary badges -->
          <div class="summary-bar">
            <span class="glass-badge glass-badge--green">{{ result.converted }} Converted</span>
            <span class="glass-badge glass-badge--cyan">{{ result.pluginConfig }} PluginConfig</span>
            <span v-if="result.customPlugin" class="glass-badge glass-badge--purple">{{ result.customPlugin }} Custom Plugin</span>
            <span v-if="result.manual" class="glass-badge glass-badge--yellow">{{ result.manual }} Manual</span>
            <span v-if="result.unknown" class="glass-badge glass-badge--blue">{{ result.unknown }} Unknown</span>
            <span class="summary-files">{{ result.ingressFiles }}/{{ result.totalFiles }} files</span>
          </div>

          <div v-if="result.warnings.length" class="warnings">
            <div v-for="(w, i) in result.warnings" :key="i" class="warning-item">
              <span class="warning-icon">&#9888;</span> {{ w }}
            </div>
          </div>

          <table class="report-table">
            <thead>
              <tr>
                <th>File</th>
                <th>Annotation</th>
                <th>Status</th>
                <th>Detail</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(f, i) in filteredFindings" :key="i">
                <td class="cell-path">{{ f.path }}</td>
                <td class="cell-annotation"><code>{{ f.annotation }}</code></td>
                <td>
                  <span class="glass-badge" :class="`glass-badge--${statusColors[f.status] || 'blue'}`">
                    {{ statusLabels[f.status] || f.status }}
                  </span>
                </td>
                <td class="cell-detail">{{ f.detail }}</td>
              </tr>
              <tr v-if="filteredFindings.length === 0">
                <td colspan="4" class="cell-empty">No annotations found</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="!loading && !error" class="empty-state">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" opacity="0.3"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
          <p>Paste Ingress YAML and click Check to analyze annotations</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.check-page {
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

.toolbar-spacer { flex: 1; }

.check-layout {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 10px;
}

.input-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border: 0.5px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.report-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  border: 0.5px solid var(--border);
  border-radius: 10px;
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

.pane-dot { width: 8px; height: 8px; border-radius: 50%; }
.pane-dot.green { background: var(--green); }
.pane-dot.blue { background: var(--accent); }
.pane-label { flex: 1; }

.search-input {
  padding: 4px 10px !important;
  font-size: 0.8rem !important;
  width: 180px;
}

.error-msg {
  padding: 10px 14px;
  color: var(--red);
  font-size: 0.85rem;
}

.report-body {
  flex: 1;
  overflow: auto;
  padding: 10px;
}

.summary-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 8px 0;
  margin-bottom: 8px;
}

.summary-files {
  margin-left: auto;
  font-size: 0.78rem;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.warnings {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  margin-bottom: 10px;
  background: rgba(255,214,10,0.08);
  border: 1px solid rgba(255,214,10,0.15);
  border-radius: 8px;
}

.warning-item { font-size: 0.82rem; color: var(--yellow); }
.warning-icon { margin-right: 6px; }

.report-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.82rem;
}

.report-table th {
  text-align: left;
  padding: 8px 10px;
  color: var(--text-muted);
  font-weight: 500;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  position: sticky;
  top: 0;
  background: var(--bg-input);
  z-index: 1;
}

.report-table td {
  padding: 7px 10px;
  border-bottom: 0.5px solid var(--border);
  vertical-align: top;
}

.cell-path { font-weight: 500; white-space: nowrap; font-size: 0.78rem; color: var(--text-muted); }
.cell-annotation code { font-size: 0.78rem; }
.cell-detail { color: var(--text-muted); font-size: 0.8rem; }
.cell-empty { text-align: center; color: var(--text-muted); padding: 24px !important; }

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--text-muted);
  font-size: 0.85rem;
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
</style>
