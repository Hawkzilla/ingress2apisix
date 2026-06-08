import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Announcement } from '@/types/api'
import { api } from '@/api/client'

export const useAppStore = defineStore('app', () => {
  const version = ref('')
  const theme = ref<'dark' | 'light'>('light')
  const announcements = ref<Announcement[]>([])
  const loading = ref(false)

  function initTheme() {
    const saved = localStorage.getItem('ingress2apisix-theme') as 'dark' | 'light' | null
    theme.value = saved || 'light'
    applyTheme()
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem('ingress2apisix-theme', theme.value)
    applyTheme()
  }

  function applyTheme() {
    if (theme.value === 'dark') {
      document.documentElement.removeAttribute('data-theme')
    } else {
      document.documentElement.setAttribute('data-theme', 'light')
    }
  }

  async function loadVersion() {
    // Version is injected via the HTML; we can fetch it from an API or parse from page
    // For now, placeholder
    version.value = ''
  }

  async function loadAnnouncements() {
    try {
      const data = await api.get<Announcement[]>('/api/announcements')
      announcements.value = data.filter((a) => a.active)
    } catch {
      // Silently fail - announcements are non-critical
    }
  }

  initTheme()

  return {
    version,
    theme,
    announcements,
    loading,
    toggleTheme,
    loadVersion,
    loadAnnouncements,
  }
})
