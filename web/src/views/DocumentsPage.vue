<script setup lang="ts">
import { ref, computed } from 'vue'
import { api } from '@/api/client'

interface ManagedDoc {
  id: string
  name: string
  filename: string
  size: number
  uploadedAt: string
  category: string
}

const documents = ref<ManagedDoc[]>([])
const selectedIds = ref<Set<string>>(new Set())
const searchQuery = ref('')
const uploading = ref(false)
const dragOver = ref(false)
const selectedDoc = ref<ManagedDoc | null>(null)
const previewContent = ref('')

const categories = computed(() => {
  const cats = new Set(documents.value.map((d) => d.category))
  return ['All', ...Array.from(cats)]
})
const activeCategory = ref('All')

const filteredDocs = computed(() => {
  let list = documents.value
  if (activeCategory.value !== 'All') {
    list = list.filter((d) => d.category === activeCategory.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter((d) => d.name.toLowerCase().includes(q) || d.filename.toLowerCase().includes(q))
  }
  return list
})

function handleDrop(e: DragEvent) {
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (files) uploadFiles(Array.from(files))
}

function handleFileSelect(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (files) uploadFiles(Array.from(files))
  ;(e.target as HTMLInputElement).value = ''
}

async function uploadFiles(files: File[]) {
  const mdFiles = files.filter((f) => f.name.endsWith('.md'))
  if (mdFiles.length === 0) return
  uploading.value = true
  try {
    const formData = new FormData()
    for (const f of mdFiles) {
      formData.append('files', f)
    }
    const res = await api.upload<{ success: boolean; uploaded?: string[]; error?: string }>('/api/docs/upload', formData)
    if (res.success) {
      // Add uploaded files to local list so they appear immediately
      for (const f of mdFiles) {
        documents.value.push({
          id: Date.now().toString() + Math.random().toString(36).slice(2, 6),
          name: f.name.replace(/\.md$/, ''),
          filename: f.name,
          size: f.size,
          uploadedAt: new Date().toISOString(),
          category: guessCategory(f.name),
        })
      }
    }
  } finally {
    uploading.value = false
  }
}

function guessCategory(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes('report') || lower.includes('report')) return 'Reports'
  if (lower.includes('guide') || lower.includes('tutorial')) return 'Guides'
  if (lower.includes('changelog') || lower.includes('release')) return 'Releases'
  return 'General'
}

function toggleSelect(id: string) {
  const s = new Set(selectedIds.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selectedIds.value = s
}

function selectAll() {
  if (selectedIds.value.size === filteredDocs.value.length) {
    selectedIds.value = new Set()
  } else {
    selectedIds.value = new Set(filteredDocs.value.map((d) => d.id))
  }
}

function deleteSelected() {
  documents.value = documents.value.filter((d) => !selectedIds.value.has(d.id))
  if (selectedDoc.value && selectedIds.value.has(selectedDoc.value.id)) {
    selectedDoc.value = null
    previewContent.value = ''
  }
  selectedIds.value = new Set()
}

function openDoc(doc: ManagedDoc) {
  selectedDoc.value = doc
}

function closePreview() {
  selectedDoc.value = null
  previewContent.value = ''
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function downloadDoc(doc: ManagedDoc) {
  const a = document.createElement('a')
  a.href = `data:text/markdown;charset=utf-4,${encodeURIComponent(previewContent.value || '# ' + doc.name)}`
  a.download = doc.filename
  a.click()
}

const categoryIcons: Record<string, string> = {
  All: '📚',
  Reports: '📊',
  Guides: '📘',
  Releases: '🚀',
  General: '📄',
}
</script>

<template>
  <div class="documents-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <label class="glass-btn glass-btn--primary glass-btn--sm">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg>
        Upload .md
        <input type="file" accept=".md" multiple style="display:none" @change="handleFileSelect" />
      </label>

      <button
        v-if="selectedIds.size > 0"
        class="glass-btn glass-btn--danger glass-btn--sm"
        @click="deleteSelected"
      >
        Delete ({{ selectedIds.size }})
      </button>

      <button
        v-if="filteredDocs.length > 0"
        class="glass-btn glass-btn--ghost glass-btn--sm"
        @click="selectAll"
      >
        {{ selectedIds.size === filteredDocs.length ? 'Deselect All' : 'Select All' }}
      </button>

      <div class="toolbar-spacer" />

      <div class="category-tabs">
        <button
          v-for="cat in categories"
          :key="cat"
          class="cat-tab"
          :class="{ active: activeCategory === cat }"
          @click="activeCategory = cat"
        >
          <span class="cat-icon">{{ categoryIcons[cat] || '📄' }}</span>
          {{ cat }}
          <span v-if="cat !== 'All'" class="cat-count">{{ documents.filter(d => d.category === cat).length }}</span>
        </button>
      </div>

      <input
        v-model="searchQuery"
        class="glass-input search-input"
        placeholder="Search documents..."
      />
    </div>

    <!-- Drop zone -->
    <div
      class="drop-zone"
      :class="{ active: dragOver }"
      @dragover.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop.prevent="handleDrop"
    >
      <div class="drop-icon">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2" opacity="0.35">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="12" y1="18" x2="12" y2="12" /><line x1="9" y1="15" x2="15" y2="15" />
        </svg>
      </div>
      <p v-if="!uploading" class="drop-text">Drag & drop <strong>.md</strong> files here, or click <strong>Upload .md</strong></p>
      <p v-else class="drop-text">Uploading...</p>
    </div>

    <!-- Documents grid -->
    <div v-if="filteredDocs.length > 0" class="doc-grid">
      <div
        v-for="doc in filteredDocs"
        :key="doc.id"
        class="doc-card"
        :class="{ selected: selectedIds.has(doc.id) }"
        @click="openDoc(doc)"
      >
        <div class="doc-card-header">
          <input
            type="checkbox"
            :checked="selectedIds.has(doc.id)"
            @click.stop
            @change="toggleSelect(doc.id)"
          />
          <span class="doc-category-badge">{{ doc.category }}</span>
        </div>
        <div class="doc-card-icon">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" />
          </svg>
        </div>
        <div class="doc-card-name">{{ doc.name }}</div>
        <div class="doc-card-meta">
          <span>{{ formatSize(doc.size) }}</span>
          <span>{{ formatDate(doc.uploadedAt) }}</span>
        </div>
      </div>
    </div>

    <div v-else-if="!dragOver" class="empty-state">
      <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" opacity="0.2">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" />
      </svg>
      <p>No documents yet. Drop <code>.md</code> files above or click <strong>Upload .md</strong> to get started.</p>
    </div>

    <!-- Preview panel -->
    <Teleport to="body">
      <Transition name="slide-preview">
        <div v-if="selectedDoc" class="preview-panel">
          <div class="preview-header">
            <div class="preview-title">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" />
              </svg>
              {{ selectedDoc.name }}
            </div>
            <div class="preview-actions">
              <button class="glass-btn glass-btn--sm" @click="downloadDoc(selectedDoc)">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
                Download
              </button>
              <button class="glass-btn glass-btn--sm glass-btn--ghost" @click="closePreview">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
              </button>
            </div>
          </div>
          <div class="preview-content">
            <div class="preview-placeholder">
              <p>Document preview will appear here once the backend file management API is ready.</p>
              <p class="preview-hint">File: {{ selectedDoc.filename }} &middot; {{ formatSize(selectedDoc.size) }}</p>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.documents-page {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
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
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.toolbar-spacer { flex: 1; }

.search-input {
  width: 200px;
  padding: 6px 12px !important;
  font-size: 0.82rem !important;
}

.category-tabs {
  display: flex;
  gap: 2px;
  background: rgba(255,255,255,0.04);
  padding: 2px;
  border-radius: 8px;
}

[data-theme="light"] .category-tabs {
  background: rgba(0,0,0,0.03);
}

.cat-tab {
  padding: 5px 10px;
  border-radius: 7px;
  font-size: 0.78rem;
  font-weight: 500;
  color: var(--text-muted);
  border: none;
  background: none;
  cursor: pointer;
  transition: all 0.15s ease;
  display: flex;
  align-items: center;
  gap: 4px;
}

.cat-tab:hover {
  color: var(--text);
  background: rgba(255,255,255,0.06);
}

[data-theme="light"] .cat-tab:hover {
  background: rgba(0,0,0,0.05);
}

.cat-tab.active {
  color: var(--text);
  background: rgba(255,255,255,0.12);
  font-weight: 600;
}

[data-theme="light"] .cat-tab.active {
  background: #fff;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}

.cat-icon { font-size: 13px; }

.cat-count {
  background: rgba(10,132,255,0.2);
  color: var(--accent);
  padding: 0 5px;
  border-radius: 6px;
  font-size: 0.7rem;
  font-weight: 600;
  min-width: 16px;
  text-align: center;
}

.drop-zone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 28px;
  border: 2px dashed var(--border);
  border-radius: 12px;
  margin-bottom: 16px;
  transition: all 0.2s ease;
}

.drop-zone.active {
  border-color: var(--accent);
  background: rgba(10,132,255,0.05);
}

.drop-text {
  color: var(--text-muted);
  font-size: 0.85rem;
}

.drop-text strong {
  color: var(--text);
  font-weight: 500;
}

/* Card grid */
.doc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.doc-card {
  background: var(--bg-card);
  backdrop-filter: var(--glass-blur-light);
  -webkit-backdrop-filter: var(--glass-blur-light);
  border: 0.5px solid var(--border);
  border-radius: 10px;
  padding: 14px;
  cursor: pointer;
  transition: all 0.15s ease;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.doc-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}

[data-theme="light"] .doc-card:hover {
  box-shadow: 0 4px 12px rgba(0,0,0,0.06);
}

.doc-card.selected {
  border-color: var(--accent);
  background: rgba(10,132,255,0.08);
}

.doc-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.doc-card-header input[type="checkbox"] {
  width: 14px;
  height: 14px;
}

.doc-category-badge {
  font-size: 0.7rem;
  color: var(--accent);
  background: rgba(10,132,255,0.1);
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 500;
}

.doc-card-icon {
  display: flex;
  justify-content: center;
  padding: 8px 0;
  color: var(--text-muted);
  opacity: 0.6;
}

.doc-card-name {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.doc-card-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.72rem;
  color: var(--text-muted);
}

.empty-state {
  text-align: center;
  padding: 48px 20px;
  color: var(--text-muted);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.empty-state code {
  background: var(--bg-card);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}

/* Preview panel (slide-in from right) */
.preview-panel {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 480px;
  max-width: 90vw;
  background: var(--bg);
  border-left: 0.5px solid var(--border);
  box-shadow: -8px 0 40px rgba(0,0,0,0.3);
  z-index: 200;
  display: flex;
  flex-direction: column;
}

[data-theme="light"] .preview-panel {
  box-shadow: -8px 0 40px rgba(0,0,0,0.08);
}

.preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 0.5px solid var(--border);
  flex-shrink: 0;
}

.preview-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text);
}

.preview-actions {
  display: flex;
  gap: 6px;
}

.preview-content {
  flex: 1;
  overflow: auto;
  padding: 18px;
}

.preview-placeholder {
  text-align: center;
  color: var(--text-muted);
  padding: 40px 0;
  font-size: 0.85rem;
}

.preview-hint {
  font-size: 0.78rem;
  margin-top: 8px;
  opacity: 0.7;
}

/* Transitions */
.slide-preview-enter-active, .slide-preview-leave-active {
  transition: transform 0.25s ease, opacity 0.25s ease;
}
.slide-preview-enter-from, .slide-preview-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
