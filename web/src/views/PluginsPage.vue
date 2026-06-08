<script setup lang="ts">
import { ref } from 'vue'
import type { PluginTemplate, ConfigTemplate } from '@/types/api'

const activeTab = ref<'plugin' | 'config'>('plugin')

const pluginTemplates = ref<PluginTemplate[]>([])
const configTemplates = ref<ConfigTemplate[]>([])
const showPluginForm = ref(false)
const showConfigForm = ref(false)

const pluginForm = ref({
  name: '',
  pluginName: '',
  description: '',
  configYaml: '',
})

const configForm = ref({
  name: '',
  description: '',
  yaml: '',
})

function showNewPlugin() {
  pluginForm.value = { name: '', pluginName: '', description: '', configYaml: '' }
  showPluginForm.value = true
}

function showNewConfig() {
  configForm.value = { name: '', description: '', yaml: '' }
  showConfigForm.value = true
}

function savePlugin() {
  const t: PluginTemplate = {
    id: Date.now().toString(),
    name: pluginForm.value.name,
    pluginName: pluginForm.value.pluginName,
    description: pluginForm.value.description,
    config: {},
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  pluginTemplates.value.push(t)
  showPluginForm.value = false
}

function saveConfig() {
  const t: ConfigTemplate = {
    id: Date.now().toString(),
    name: configForm.value.name,
    description: configForm.value.description,
    yaml: configForm.value.yaml,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  configTemplates.value.push(t)
  showConfigForm.value = false
}

function deletePlugin(id: string) {
  pluginTemplates.value = pluginTemplates.value.filter((t) => t.id !== id)
}

function deleteConfig(id: string) {
  configTemplates.value = configTemplates.value.filter((t) => t.id !== id)
}
</script>

<template>
  <div class="plugins-page">
    <!-- Tab switcher -->
    <div class="tab-bar">
      <button
        class="tab-item"
        :class="{ active: activeTab === 'plugin' }"
        @click="activeTab = 'plugin'"
      >Plugin Templates</button>
      <button
        class="tab-item"
        :class="{ active: activeTab === 'config' }"
        @click="activeTab = 'config'"
      >Config Templates</button>
    </div>

    <!-- Plugin Templates -->
    <div v-show="activeTab === 'plugin'" class="tab-body">
      <div class="toolbar">
        <button class="glass-btn glass-btn--primary glass-btn--sm" @click="showNewPlugin">+ New Plugin Template</button>
      </div>

      <transition name="slide">
        <div v-if="showPluginForm" class="form-card glass-card">
          <h3>New Plugin Template</h3>
          <div class="form-row">
            <div class="form-field" style="flex:1">
              <label>Name</label>
              <input v-model="pluginForm.name" class="glass-input" placeholder="e.g. CORS with credentials" />
            </div>
            <div class="form-field" style="flex:1">
              <label>Plugin Name</label>
              <input v-model="pluginForm.pluginName" class="glass-input" placeholder="e.g. cors" />
            </div>
          </div>
          <div class="form-field">
            <label>Description</label>
            <input v-model="pluginForm.description" class="glass-input" placeholder="Brief description..." />
          </div>
          <div class="form-field">
            <label>Config (YAML)</label>
            <textarea v-model="pluginForm.configYaml" class="glass-input code-textarea" rows="8" placeholder="allow_credential: true
allow_origins:
  - '*'
allow_methods:
  - GET
  - POST" />
          </div>
          <div class="form-actions">
            <button class="glass-btn glass-btn--primary glass-btn--sm" @click="savePlugin">Save</button>
            <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="showPluginForm = false">Cancel</button>
          </div>
        </div>
      </transition>

      <div class="template-grid">
        <div v-for="t in pluginTemplates" :key="t.id" class="template-card glass-card">
          <div class="template-header">
            <span class="template-name">{{ t.name }}</span>
            <button class="icon-btn" @click="deletePlugin(t.id)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
            </button>
          </div>
          <code class="template-plugin">{{ t.pluginName }}</code>
          <p class="template-desc">{{ t.description }}</p>
        </div>
      </div>
      <div v-if="pluginTemplates.length === 0 && !showPluginForm" class="empty-state">
        <p>No plugin templates yet. Create one to reuse common plugin configurations.</p>
      </div>
    </div>

    <!-- Config Templates -->
    <div v-show="activeTab === 'config'" class="tab-body">
      <div class="toolbar">
        <button class="glass-btn glass-btn--primary glass-btn--sm" @click="showNewConfig">+ New Config Template</button>
      </div>

      <transition name="slide">
        <div v-if="showConfigForm" class="form-card glass-card">
          <h3>New Config Template</h3>
          <div class="form-row">
            <div class="form-field" style="flex:1">
              <label>Name</label>
              <input v-model="configForm.name" class="glass-input" placeholder="e.g. Standard Ingress Config" />
            </div>
          </div>
          <div class="form-field">
            <label>Description</label>
            <input v-model="configForm.description" class="glass-input" placeholder="Brief description..." />
          </div>
          <div class="form-field">
            <label>Config (YAML)</label>
            <textarea v-model="configForm.yaml" class="glass-input code-textarea" rows="10" placeholder="Full Ingress YAML template..." />
          </div>
          <div class="form-actions">
            <button class="glass-btn glass-btn--primary glass-btn--sm" @click="saveConfig">Save</button>
            <button class="glass-btn glass-btn--ghost glass-btn--sm" @click="showConfigForm = false">Cancel</button>
          </div>
        </div>
      </transition>

      <div class="template-grid">
        <div v-for="t in configTemplates" :key="t.id" class="template-card glass-card">
          <div class="template-header">
            <span class="template-name">{{ t.name }}</span>
            <button class="icon-btn" @click="deleteConfig(t.id)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
            </button>
          </div>
          <p class="template-desc">{{ t.description }}</p>
        </div>
      </div>
      <div v-if="configTemplates.length === 0 && !showConfigForm" class="empty-state">
        <p>No config templates yet. Create one to save common Ingress configurations.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plugins-page {
  padding: 20px;
  max-width: 1000px;
  margin: 0 auto;
}

.tab-bar {
  display: flex;
  gap: 2px;
  background: var(--bg-hover);
  padding: 3px;
  border-radius: 10px;
  margin-bottom: 16px;
  width: fit-content;
}

.tab-item {
  padding: 7px 18px;
  border-radius: 8px;
  border: none;
  background: none;
  color: var(--text-muted);
  font-family: var(--font-sans);
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.tab-item:hover { color: var(--text); }
.tab-item.active { background: var(--bg-card); color: var(--text); box-shadow: 0 1px 3px rgba(0,0,0,0.12); }

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.form-card {
  padding: 20px;
  margin-bottom: 16px;
}

.form-card h3 { font-size: 1rem; margin-bottom: 14px; }

.form-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 10px;
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

.code-textarea {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.5;
  resize: vertical;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 10px;
}

.template-card {
  padding: 14px;
  transition: transform 0.15s;
}

.template-card:hover { transform: translateY(-1px); }

.template-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.template-name { font-weight: 600; font-size: 0.9rem; flex: 1; }
.template-plugin { font-size: 0.78rem; color: var(--accent); }
.template-desc { font-size: 0.8rem; color: var(--text-muted); line-height: 1.4; margin-top: 4px; }

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

.icon-btn:hover { background: var(--bg-hover); color: var(--red); }

.empty-state {
  text-align: center;
  padding: 40px;
  color: var(--text-muted);
}
</style>
