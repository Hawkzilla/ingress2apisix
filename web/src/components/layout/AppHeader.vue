<script setup lang="ts">
import { useRoute } from 'vue-router'
import ThemeToggle from './ThemeToggle.vue'

const route = useRoute()

const navItems = [
  { name: 'convert', label: 'Convert', path: '/convert', icon: '⚙' },
  { name: 'check', label: 'Check', path: '/check', icon: '☑' },
  { name: 'docs', label: 'Docs', path: '/docs', icon: '📖' },
]
</script>

<template>
  <header class="app-header">
    <div class="header-inner">
      <div class="header-left">
        <h1 class="logo-title">
          ingress-nginx
          <span class="logo-arrow">&#8594;</span>
          APISIX
        </h1>
      </div>

      <nav class="header-nav">
        <router-link
          v-for="item in navItems"
          :key="item.name"
          :to="item.path"
          class="nav-tab"
          :class="{ active: route.path.startsWith(item.path) }"
        >
          <span class="tab-icon">{{ item.icon }}</span>
          {{ item.label }}
        </router-link>
      </nav>

      <div class="header-right">
        <span class="header-meta">Migration Tool</span>
        <span class="header-divider" />
        <span class="header-meta header-credit">
          by <img src="/static/logo.png" alt="Easystack" class="logo-img" />&nbsp;&copy; ECNS Team
        </span>
        <ThemeToggle />
      </div>
    </div>
    <div class="header-gradient-bar" />
  </header>
</template>

<style scoped>
.app-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  min-height: 56px;
  background: rgba(28,28,30,0.65);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-bottom: 0.5px solid var(--border);
  box-shadow: 0 0.5px 0 0 rgba(255,255,255,0.04), 0 4px 24px rgba(0,0,0,0.2);
}

[data-theme="light"] .app-header {
  background: rgba(255,255,255,0.82);
  box-shadow: 0 0.5px 0 0 rgba(0,0,0,0.04), 0 4px 24px rgba(0,0,0,0.06);
}

.header-inner {
  max-width: 1600px;
  margin: 0 auto;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 28px;
  gap: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.logo-title {
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.02em;
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.logo-arrow {
  font-size: 18px;
  margin: 0 4px;
  -webkit-text-fill-color: var(--accent);
}

.header-nav {
  display: flex;
  gap: 3px;
  background: rgba(255,255,255,0.05);
  padding: 3px;
  border-radius: 22px;
}

[data-theme="light"] .header-nav {
  background: rgba(0,0,0,0.04);
}

.nav-tab {
  padding: 7px 16px;
  border-radius: 22px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  border: 0.5px solid transparent;
  text-decoration: none;
  white-space: nowrap;
}

.nav-tab:hover {
  color: var(--text);
  background: rgba(255,255,255,0.06);
}

[data-theme="light"] .nav-tab:hover {
  background: rgba(0,0,0,0.06);
  color: #1d1d1f;
}

.nav-tab.active {
  color: #f5f5f7;
  font-weight: 600;
  background: rgba(255,255,255,0.15);
  border-color: rgba(255,255,255,0.1);
  box-shadow: 0 1px 4px rgba(0,0,0,0.2);
}

[data-theme="light"] .nav-tab.active {
  color: #1d1d1f;
  background: #ffffff;
  border-color: rgba(0,0,0,0.1);
  box-shadow: 0 1px 4px rgba(0,0,0,0.08), 0 0.5px 1px rgba(0,0,0,0.04);
}

[data-theme="light"] .nav-tab {
  color: #48484a;
}

.tab-icon {
  font-size: 15px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.header-meta {
  font-size: 11px;
  color: var(--text-muted);
  letter-spacing: 0;
}

.header-divider {
  width: 1px;
  height: 16px;
  background: rgba(255,255,255,0.12);
}

[data-theme="light"] .header-divider {
  background: rgba(0,0,0,0.1);
}

.header-credit {
  display: flex;
  align-items: center;
}

.logo-img {
  height: 14px;
  vertical-align: middle;
  filter: brightness(0) invert(1);
}

[data-theme="light"] .logo-img {
  filter: none;
}

.header-gradient-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 0.5px;
  background: linear-gradient(90deg, transparent, var(--accent-glow), transparent);
}

@media (max-width: 768px) {
  .header-nav {
    display: none;
  }
  .header-meta, .header-divider, .header-credit {
    display: none;
  }
}
</style>
