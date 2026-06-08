<script setup lang="ts">
import { useAppStore } from '@/stores/app'

const app = useAppStore()
</script>

<template>
  <button
    class="theme-toggle"
    :title="app.theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
    @click="app.toggleTheme()"
  >
    <span class="toggle-track" :class="{ dark: app.theme === 'dark' }">
      <span class="toggle-icon sun">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5" /><line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" /><line x1="4.22" y1="4.22" x2="5.64" y2="5.64" /><line x1="18.36" y1="18.36" x2="19.78" y2="19.78" /><line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" /><line x1="4.22" y1="19.78" x2="5.64" y2="18.36" /><line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
        </svg>
      </span>
      <span class="toggle-thumb" />
      <span class="toggle-icon moon">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      </span>
    </span>
  </button>
</template>

<style scoped>
.theme-toggle {
  display: flex;
  align-items: center;
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
}

.toggle-track {
  position: relative;
  width: 48px;
  height: 26px;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: 13px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 5px;
  transition: background 0.25s, border-color 0.25s;
}

.toggle-track.dark {
  background: var(--accent);
  border-color: var(--accent);
}

.toggle-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  background: #fff;
  border-radius: 50%;
  box-shadow: 0 1px 4px rgba(0,0,0,0.2);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.toggle-track.dark .toggle-thumb {
  transform: translateX(22px);
}

.toggle-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1;
  transition: opacity 0.25s;
}

.sun {
  color: var(--yellow);
  opacity: 1;
}

.toggle-track.dark .sun {
  opacity: 0;
}

.moon {
  color: rgba(255,255,255,0.7);
  opacity: 0;
}

.toggle-track.dark .moon {
  opacity: 1;
}
</style>
