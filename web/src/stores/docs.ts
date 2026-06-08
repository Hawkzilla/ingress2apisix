import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/api/client'

interface DocInfo {
  name: string
  title: string
  file: string
}

interface DocCategory {
  key: string
  label: string
  icon: string // SVG path
  matcher: (doc: DocInfo) => boolean
}

export const useDocsStore = defineStore('docs', () => {
  const docs = ref<DocInfo[]>([])
  const currentDocName = ref('')
  const currentDocTitle = ref('')
  const currentDocContent = ref('')
  const loading = ref(false)
  const sidebarCollapsed = ref(false)
  const sidebarWidth = ref(280)

  // Category definitions
  const categories: DocCategory[] = [
    {
      key: 'getting-started',
      label: 'Getting Started',
      icon: 'M13 10V3L4 14h7v7l9-11h-7z',
      matcher: (d) => /^(usage|webui|migration|readme|getting-started)$/i.test(d.name),
    },
    {
      key: 'annotations',
      label: 'Annotations',
      icon: 'M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z',
      matcher: (d) => /^annotation/.test(d.name),
    },
    {
      key: 'analysis',
      label: 'Analysis',
      icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
      matcher: (d) => /analysis|report|hxk8s/.test(d.name),
    },
    {
      key: 'other',
      label: 'Other',
      icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10',
      matcher: () => true, // catch-all
    },
  ]

  const categorizedDocs = computed(() => {
    const result: { category: DocCategory; docs: DocInfo[] }[] = []
    const assigned = new Set<string>()

    for (const cat of categories) {
      const matched = docs.value.filter((d) => !assigned.has(d.name) && cat.matcher(d))
      if (matched.length > 0) {
        result.push({ category: cat, docs: matched })
        matched.forEach((d) => assigned.add(d.name))
      }
    }
    return result
  })

  async function loadDocs() {
    try {
      const res = await api.get<{ success: boolean; data: DocInfo[] }>('/api/docs/list')
      docs.value = res.data ?? []
    } catch {
      docs.value = []
    }
  }

  async function loadDoc(name: string) {
    loading.value = true
    try {
      const token = localStorage.getItem('ingress2apisix-token')
      const headers: Record<string, string> = {}
      if (token) headers['Authorization'] = `Bearer ${token}`
      const res = await fetch(`/api/docs/${name}`, { headers })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const text = await res.text()

      currentDocName.value = name
      currentDocContent.value = text

      const titleMatch = text.match(/^#\s+(.+)$/m)
      currentDocTitle.value = titleMatch ? titleMatch[1] : name
    } catch {
      currentDocName.value = name
      currentDocTitle.value = name
      currentDocContent.value = ''
    } finally {
      loading.value = false
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
    localStorage.setItem('ingress2apisix-docs-collapsed', String(sidebarCollapsed.value))
  }

  function setSidebarWidth(w: number) {
    sidebarWidth.value = w
    localStorage.setItem('ingress2apisix-docs-width', String(w))
  }

  const savedCollapsed = localStorage.getItem('ingress2apisix-docs-collapsed')
  if (savedCollapsed === 'true') sidebarCollapsed.value = true

  const savedWidth = localStorage.getItem('ingress2apisix-docs-width')
  if (savedWidth) sidebarWidth.value = Number(savedWidth) || 280

  return {
    docs,
    currentDocName,
    currentDocTitle,
    currentDocContent,
    loading,
    sidebarCollapsed,
    sidebarWidth,
    categorizedDocs,
    categories,
    loadDocs,
    loadDoc,
    toggleSidebar,
    setSidebarWidth,
  }
})
