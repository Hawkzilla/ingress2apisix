<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { Announcement } from '@/types/api'
import { api } from '@/api/client'

const announcements = ref<Announcement[]>([])
const showForm = ref(false)
const editingId = ref('')
const loading = ref(false)

const form = ref({
  title: '',
  content: '',
  level: 'info' as 'info' | 'warning' | 'error',
  active: true,
})

async function loadAnnouncements() {
  try {
    announcements.value = await api.get<Announcement[]>('/api/announcements')
  } catch { /* silent */ }
}

function showNewForm() {
  editingId.value = ''
  form.value = { title: '', content: '', level: 'info', active: true }
  showForm.value = true
}

function editAnnouncement(ann: Announcement) {
  editingId.value = ann.id
  form.value = { title: ann.title, content: ann.content, level: ann.level, active: ann.active }
  showForm.value = true
}

async function saveAnnouncement() {
  loading.value = true
  try {
    if (editingId.value) {
      await api.put('/api/announcements', { id: editingId.value, ...form.value })
    } else {
      await api.post('/api/announcements', form.value)
    }
    showForm.value = false
    await loadAnnouncements()
  } catch (e) {
    alert(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function deleteAnnouncement(id: string) {
  if (!confirm('Delete this announcement?')) return
  try {
    await api.delete(`/api/announcements?id=${id}`)
    await loadAnnouncements()
  } catch (e) {
    alert(e instanceof Error ? e.message : String(e))
  }
}

onMounted(loadAnnouncements)
</script>

<template>
  <div class="admin-page">
    <div class="toolbar">
      <button class="glass-btn glass-btn--primary glass-btn--sm" @click="showNewForm">
        + New Announcement
      </button>
      <div class="toolbar-spacer" />
      <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="loadAnnouncements">Refresh</button>
    </div>

    <!-- Form -->
    <transition name="slide">
      <div v-if="showForm" class="ann-form glass-card">
        <h3 class="form-title">{{ editingId ? 'Edit' : 'New' }} Announcement</h3>
        <div class="form-row">
          <div class="form-field">
            <label>Level</label>
            <select v-model="form.level" class="glass-select">
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div class="form-field" style="flex:1">
            <label>Title</label>
            <input v-model="form.title" class="glass-input" placeholder="Title" />
          </div>
          <div class="form-field">
            <label>Active</label>
            <label class="glass-toggle">
              <input type="checkbox" v-model="form.active" />
              <span class="glass-toggle__track"><span class="glass-toggle__thumb" /></span>
            </label>
          </div>
        </div>
        <div class="form-field">
          <label>Content</label>
          <textarea v-model="form.content" class="glass-input" rows="3" placeholder="Announcement content..." style="resize:vertical" />
        </div>
        <div class="form-actions">
          <button class="glass-btn glass-btn--primary glass-btn--sm" :disabled="loading" @click="saveAnnouncement">
            {{ loading ? 'Saving...' : 'Save' }}
          </button>
          <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="showForm = false">Cancel</button>
        </div>
      </div>
    </transition>

    <!-- List -->
    <div class="ann-list">
      <div v-for="ann in announcements" :key="ann.id" class="ann-item glass-card">
        <div class="ann-header">
          <span class="glass-badge" :class="{
            'glass-badge--blue': ann.level === 'info',
            'glass-badge--yellow': ann.level === 'warning',
            'glass-badge--red': ann.level === 'error',
          }">{{ ann.level }}</span>
          <span class="ann-title">{{ ann.title || '(no title)' }}</span>
          <span v-if="!ann.active" class="glass-badge glass-badge--purple">Inactive</span>
          <div class="ann-spacer" />
          <button class="icon-btn" @click="editAnnouncement(ann)">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
          </button>
          <button class="icon-btn" @click="deleteAnnouncement(ann.id)">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
          </button>
        </div>
        <p class="ann-content">{{ ann.content }}</p>
      </div>
      <div v-if="announcements.length === 0" class="empty-state">
        <p>No announcements yet. Click "New Announcement" to create one.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-page {
  padding: 20px;
  max-width: 900px;
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
}

.toolbar-spacer { flex: 1; }

.ann-form {
  padding: 20px;
  margin-bottom: 16px;
}

.form-title {
  font-size: 1rem;
  margin-bottom: 14px;
}

.form-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-field label {
  font-size: 0.78rem;
  color: var(--text-muted);
  font-weight: 500;
}

.form-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.ann-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ann-item {
  padding: 14px;
}

.ann-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.ann-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.ann-spacer { flex: 1; }

.ann-content {
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.5;
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

.empty-state {
  text-align: center;
  padding: 40px;
  color: var(--text-muted);
}
</style>
