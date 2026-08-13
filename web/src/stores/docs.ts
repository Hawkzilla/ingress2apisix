import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'

interface DocInfo {
  name: string
  title: string
  file: string
}

interface DocTreeNode {
  name: string
  path: string
  title: string
  isDir: boolean
  children?: DocTreeNode[]
}

export const useDocsStore = defineStore('docs', () => {
  const docs = ref<DocInfo[]>([])
  const docTree = ref<DocTreeNode[]>([])
  const currentDocName = ref('')
  const currentDocTitle = ref('')
  const currentDocContent = ref('')
  const loading = ref(false)
  const sidebarCollapsed = ref(false)
  const sidebarWidth = ref(280)
  const expandedDirs = ref<Set<string>>(new Set())

  async function loadDocs() {
    try {
      const res = await api.get<{ success: boolean; data: DocInfo[]; tree: DocTreeNode[] }>('/api/docs/list')
      docs.value = res.data ?? []
      docTree.value = res.tree ?? []
    } catch {
      docs.value = []
      docTree.value = []
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

      // Auto-expand parent directories
      const parts = name.split('/')
      if (parts.length > 1) {
        for (let i = 1; i < parts.length; i++) {
          expandedDirs.value.add(parts.slice(0, i).join('/'))
        }
      }
    } catch {
      currentDocName.value = name
      currentDocTitle.value = name
      currentDocContent.value = ''
    } finally {
      loading.value = false
    }
  }

  function toggleDir(path: string) {
    if (expandedDirs.value.has(path)) {
      expandedDirs.value.delete(path)
    } else {
      expandedDirs.value.add(path)
    }
  }

  function isDirExpanded(path: string): boolean {
    return expandedDirs.value.has(path)
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
    docTree,
    currentDocName,
    currentDocTitle,
    currentDocContent,
    loading,
    sidebarCollapsed,
    sidebarWidth,
    expandedDirs,
    loadDocs,
    loadDoc,
    toggleDir,
    isDirExpanded,
    toggleSidebar,
    setSidebarWidth,
  }
})
