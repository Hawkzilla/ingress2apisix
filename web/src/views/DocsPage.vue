<script setup lang="ts">
import { ref, onMounted, watch, computed, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDocsStore } from '@/stores/docs'
import mermaid from 'mermaid'

// Initialize mermaid (startOnLoad: false — we render manually via mermaid.run)
let mermaidCounter = 0
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
})

const route = useRoute()
const router = useRouter()
const docs = useDocsStore()

const searchQuery = ref('')
const contentEl = ref<HTMLDivElement | null>(null)
const isResizing = ref(false)

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fff\s-]/g, '') // keep word chars, Chinese chars, spaces, hyphens
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim()
}

// Normalize anchor ID for matching (handles spaces, slashes, etc.)
function normalizeAnchorId(text: string): string {
  return text
    .toLowerCase()
    .replace(/[\s/]+/g, '-') // replace spaces and slashes with hyphens
    .replace(/[^\w\u4e00-\u9fff-]/g, '') // remove special chars except hyphens
    .replace(/-+/g, '-') // collapse multiple hyphens
    .trim()
}

// Filter tree based on search query
const filteredTree = computed(() => {
  if (!searchQuery.value.trim()) return docs.docTree
  const q = searchQuery.value.toLowerCase()

  function filterNode(node: any): any | null {
    if (node.isDir) {
      const filteredChildren = node.children
        ?.map(filterNode)
        .filter(Boolean) ?? []
      if (filteredChildren.length > 0) {
        return { ...node, children: filteredChildren }
      }
      return null
    }
    // File node
    if (node.title?.toLowerCase().includes(q) || node.name?.toLowerCase().includes(q)) {
      return node
    }
    return null
  }

  return docs.docTree
    .map(filterNode)
    .filter(Boolean)
})

interface TocItem {
  level: number
  text: string
  id: string
}

const tocItems = computed<TocItem[]>(() => {
  if (!docs.currentDocContent) return []
  const items: TocItem[] = []
  const regex = /^(#{1,3})\s+(.+)$/gm
  let match
  while ((match = regex.exec(docs.currentDocContent)) !== null) {
    const level = match[1].length
    const text = match[2].trim()
    items.push({ level, text, id: slugify(text) })
  }
  return items
})

function selectDoc(name: string) {
  router.push(`/docs/${name}`)
}

function showCatalog() {
  router.push('/docs')
}

// Resize sidebar
function startResize(e: MouseEvent) {
  isResizing.value = true
  const startX = e.clientX
  const startWidth = docs.sidebarWidth

  const onMove = (ev: MouseEvent) => {
    const dx = ev.clientX - startX
    docs.setSidebarWidth(Math.min(500, Math.max(180, startWidth + dx)))
  }

  const onUp = () => {
    isResizing.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

// Load docs on mount
onMounted(() => {
  docs.loadDocs()
  handleRoute()
})

watch(() => route.params.name, handleRoute)

// Scroll to hash after content loads and render mermaid diagrams
watch(() => docs.currentDocContent, async () => {
  if (docs.currentDocContent && route.hash) {
    const id = route.hash.slice(1) // remove leading #
    // Wait for DOM to render
    setTimeout(() => {
      const el = document.getElementById(id)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'start' })
      }
    }, 100)
  }
  // Render mermaid diagrams after content loads
  await renderMermaidDiagrams()
})

function handleRoute() {
  const name = route.params.name as string | undefined
  if (name) {
    docs.loadDoc(name)
  } else {
    docs.currentDocContent = ''
    docs.currentDocName = ''
  }
}

// Syntax highlighting for code blocks
function highlightCode(code: string, lang: string): string {
  let html = escapeHtml(code)
  if (!lang) return html

  const isYaml = /^(ya?ml|yml)$/i.test(lang)
  const isJson = /^json$/i.test(lang)
  const isShell = /^(bash|sh|shell|zsh)$/i.test(lang)
  const isGo = /^go$/i.test(lang)
  const isK8s = isYaml || /^(k8s|kubernetes|ingress|apisix)$/i.test(lang)

  if (isYaml || isK8s) {
    // Comments
    html = html.replace(/(#[^\n]*)/g, '<span class="hl-comment">$1</span>')
    // Keys
    html = html.replace(/^(\s*)([\w][\w\s./-]*?)(:)/gm, '$1<span class="hl-key">$2</span>$3')
    // Strings
    html = html.replace(/(:\s*)([|>]?['"][^'"]*['"])/g, '$1<span class="hl-string">$2</span>')
    // Booleans / null
    html = html.replace(/(:\s*)(true|false|null|~)(?=\s|$)/gm, '$1<span class="hl-bool">$2</span>')
    // Numbers
    html = html.replace(/(:\s*)(\d[\d.]*)(?=\s|$)/gm, '$1<span class="hl-number">$2</span>')
    // List dashes
    html = html.replace(/^(\s*)(- )/gm, '$1<span class="hl-list">$2</span>')
    // Anchors / aliases
    html = html.replace(/(&amp;\w[\w-]*)/g, '<span class="hl-anchor">$1</span>')
    html = html.replace(/(\*\w[\w-]*)/g, '<span class="hl-anchor">$1</span>')
    // Separators
    html = html.replace(/^(---)/gm, '<span class="hl-sep">$1</span>')
  } else if (isJson) {
    // Keys
    html = html.replace(/(&quot;[\w][\w-]*&quot;)(\s*:)/g, '<span class="hl-key">$1</span>$2')
    // String values
    html = html.replace(/(:\s*)(&quot;[^&]*&quot;)/g, '$1<span class="hl-string">$2</span>')
    // Booleans / null
    html = html.replace(/(:\s*)(true|false|null)(?=[,\s}\]])/g, '$1<span class="hl-bool">$2</span>')
    // Numbers
    html = html.replace(/(:\s*)(-?\d[\d.]*)(?=[,\s}\]])/g, '$1<span class="hl-number">$2</span>')
  } else if (isShell) {
    // Comments
    html = html.replace(/(#[^\n]*)/g, '<span class="hl-comment">$1</span>')
    // Strings
    html = html.replace(/('[^']*')/g, '<span class="hl-string">$1</span>')
    html = html.replace(/(&quot;[^&]*&quot;)/g, '<span class="hl-string">$1</span>')
    // Commands
    html = html.replace(/^(\$\s)/gm, '<span class="hl-list">$1</span>')
    // Flags
    html = html.replace(/(\s)(--?\w[\w-]*)/g, '$1<span class="hl-key">$2</span>')
  } else if (isGo) {
    // Comments
    html = html.replace(/(\/\/[^\n]*)/g, '<span class="hl-comment">$1</span>')
    // Strings
    html = html.replace(/(&quot;[^&]*&quot;)/g, '<span class="hl-string">$1</span>')
    // Keywords
    html = html.replace(/\b(func|var|const|if|else|for|range|return|switch|case|type|struct|interface|map|chan|go|defer|package|import)\b/g, '<span class="hl-bool">$1</span>')
    // Types
    html = html.replace(/\b(string|int|int64|float64|bool|error|byte)\b/g, '<span class="hl-key">$1</span>')
  }
  return html
}

// Enhanced markdown rendering with rich styling
function renderMd(md: string): string {
  if (!md) return ''
  let html = md

  // Fenced code blocks with syntax highlighting
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
    // Mermaid code blocks - generate placeholder for client-side rendering
    if (lang === 'mermaid') {
      mermaidCounter++
      const id = `mermaid-diagram-${mermaidCounter}`
      return `<div class="mermaid-placeholder" data-mermaid-id="${id}">${escapeHtml(code.trim())}</div>`
    }
    const highlighted = highlightCode(code.trim(), lang)
    return `<pre class="code-block"><code class="lang-${lang}">${highlighted}</code></pre>`
  })

  // Headers
  html = html.replace(/^######\s+(.+)$/gm, (_, t) => `<h6 id="${slugify(t)}">${t}</h6>`)
  html = html.replace(/^#####\s+(.+)$/gm, (_, t) => `<h5 id="${slugify(t)}">${t}</h5>`)
  html = html.replace(/^####\s+(.+)$/gm, (_, t) => `<h4 id="${slugify(t)}">${t}</h4>`)
  html = html.replace(/^###\s+(.+)$/gm, (_, t) => `<h3 id="${slugify(t)}">${t}</h3>`)
  html = html.replace(/^##\s+(.+)$/gm, (_, t) => `<h2 id="${slugify(t)}">${t}</h2>`)
  html = html.replace(/^#\s+(.+)$/gm, (_, t) => `<h1 id="${slugify(t)}">${t}</h1>`)

  // Horizontal rules
  html = html.replace(/^(?:---|\*\*\*|___)\s*$/gm, '<hr class="doc-hr" />')

  // Blockquotes
  html = html.replace(/^>\s*(.+)$/gm, '<blockquote class="doc-quote">$1</blockquote>')
  // Merge consecutive blockquotes
  html = html.replace(/<\/blockquote>\n<blockquote class="doc-quote">/g, '\n')

  // Unordered lists
  html = html.replace(/^(\s*)[-*+]\s+(.+)$/gm, (_, indent, content) => {
    const level = Math.floor(indent.length / 2)
    return `<li class="doc-li" data-level="${level}">${content}</li>`
  })

  // Ordered lists
  html = html.replace(/^(\s*)\d+\.\s+(.+)$/gm, (_, indent, content) => {
    const level = Math.floor(indent.length / 2)
    return `<li class="doc-li doc-li-ordered" data-level="${level}">${content}</li>`
  })

  // Bold, italic, inline code
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')
  html = html.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>')

  // Links
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" class="doc-link">$1</a>')

  // Images
  html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" class="doc-img" />')

  // Tables
  html = html.replace(
    /^\|(.+)\|\n\|[-| :]+\|\n((?:\|.+\|\n?)*)/gm,
    (_, header, rows) => {
      const ths = header
        .split('|')
        .filter((c: string) => c.trim())
        .map((c: string) => `<th>${c.trim()}</th>`)
        .join('')
      const trs = rows
        .trim()
        .split('\n')
        .map((row: string, i: number) => {
          const tds = row
            .split('|')
            .filter((c: string) => c.trim())
            .map((c: string) => `<td>${c.trim()}</td>`)
            .join('')
          return `<tr class="${i % 2 === 0 ? 'row-even' : 'row-odd'}">${tds}</tr>`
        })
        .join('')
      return `<table class="doc-table"><thead><tr>${ths}</tr></thead><tbody>${trs}</tbody></table>`
    }
  )

  // Paragraphs
  html = html.replace(/\n\n/g, '</p><p>')
  html = '<p>' + html + '</p>'
  html = html.replace(/<p>\s*<(h[1-6]|pre|table|ul|ol|blockquote|hr|img)/g, '<$1')
  html = html.replace(/<\/(h[1-6]|pre|table|ul|ol|blockquote|hr|img)>\s*<\/p>/g, '</$1>')
  html = html.replace(/<p>\s*<\/p>/g, '')

  // Wrap consecutive li elements in ul
  html = html.replace(/(<li class="doc-li"[^>]*>.*?<\/li>\n?)+/g, (match) => {
    const isOrdered = match.includes('doc-li-ordered')
    return `<${isOrdered ? 'ol' : 'ul'} class="doc-list">${match}</${isOrdered ? 'ol' : 'ul'}>`
  })

  return html
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

// Render mermaid diagrams after content is loaded
async function renderMermaidDiagrams() {
  await nextTick()
  // Small delay to ensure DOM is fully rendered
  await new Promise(resolve => setTimeout(resolve, 100))

  const mermaidElements = document.querySelectorAll('.mermaid-placeholder[data-mermaid-id]')
  const nodesToRender: HTMLElement[] = []

  for (const el of mermaidElements) {
    const code = el.textContent || ''
    if (!code.trim()) continue

    // Replace placeholder with a proper .mermaid div
    const container = document.createElement('div')
    container.className = 'mermaid'
    container.innerHTML = code
    el.parentNode?.replaceChild(container, el)
    nodesToRender.push(container)
  }

  if (nodesToRender.length === 0) return

  try {
    // Use mermaid.run() with nodes parameter — handles entity decoding, rendering, and innerHTML update
    await mermaid.run({ nodes: nodesToRender, suppressErrors: false })
  } catch (err) {
    console.error('Mermaid render error:', err)
    // Mark failed diagrams with error display
    for (const node of nodesToRender) {
      if (!node.querySelector('svg')) {
        const errorDiv = document.createElement('div')
        errorDiv.className = 'mermaid-error'
        errorDiv.textContent = `Mermaid Error: ${err instanceof Error ? err.message : String(err)}\n\n${node.textContent || ''}`
        node.parentNode?.replaceChild(errorDiv, node)
      }
    }
  }
}

function handleContentClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const link = target.closest('a.doc-link') as HTMLAnchorElement | null
  if (!link) return
  const href = link.getAttribute('href')
  if (!href) return

  // External links — open in new tab
  if (href.startsWith('http://') || href.startsWith('https://')) {
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
    return
  }

  e.preventDefault()

  // Anchor link within current doc
  if (href.startsWith('#')) {
    const rawId = decodeURIComponent(href.slice(1))
    // Try exact match first, then normalized match
    let el = document.getElementById(rawId)
    if (!el) {
      // Try matching with normalized IDs (handles spaces, slashes, etc.)
      const normalized = normalizeAnchorId(rawId)
      const headings = document.querySelectorAll('h1, h2, h3, h4, h5, h6')
      for (const heading of headings) {
        if (normalizeAnchorId(heading.textContent || '') === normalized) {
          el = heading as HTMLElement
          break
        }
      }
    }
    if (el) {
      history.replaceState(null, '', `#${el.id}`)
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
    return
  }

  // Internal doc link (e.g. docs/xxx or /docs/xxx)
  if (href.startsWith('/docs/') || href.startsWith('docs/')) {
    const path = href.startsWith('/') ? href : '/' + href
    router.push(path)
  }
}

function scrollToHeading(id: string) {
  // id is already slugified from tocItems
  history.replaceState(null, '', `#${id}`)
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}
</script>

<template>
  <div class="docs-page" :class="{ resizing: isResizing }">
    <!-- Sidebar -->
    <aside
      v-show="!docs.sidebarCollapsed"
      class="docs-sidebar glass-card"
      :style="{ width: docs.sidebarWidth + 'px' }"
    >
      <div class="sidebar-header">
        <span class="sidebar-title">Documentation</span>
        <button class="icon-btn" @click="docs.toggleSidebar()">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="11 17 6 12 11 7" /><polyline points="18 17 13 12 18 7" /></svg>
        </button>
      </div>

      <div class="sidebar-search">
        <input
          v-model="searchQuery"
          class="glass-input"
          placeholder="Search docs..."
          style="padding: 6px 10px; font-size: 0.82rem"
        />
      </div>

      <div class="sidebar-list">
        <button
          class="doc-item"
          :class="{ active: !route.params.name }"
          @click="showCatalog"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="3" y="14" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /></svg>
          <span>Catalog</span>
        </button>
        <template v-for="node in filteredTree" :key="node.path || node.name">
          <!-- Directory node -->
          <div v-if="node.isDir" class="dir-node">
            <button
              class="doc-item dir-item"
              @click="docs.toggleDir(node.path)"
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="dir-icon"
                :class="{ expanded: docs.isDirExpanded(node.path) }"
              >
                <polyline points="9 18 15 12 9 6" />
              </svg>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
              </svg>
              <span>{{ node.name }}</span>
            </button>
            <div v-if="docs.isDirExpanded(node.path)" class="dir-children">
              <button
                v-for="child in node.children"
                :key="child.path || child.name"
                class="doc-item"
                :class="{ active: route.params.name === child.path }"
                @click="child.isDir ? docs.toggleDir(child.path) : selectDoc(child.path)"
              >
                <template v-if="child.isDir">
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    class="dir-icon"
                    :class="{ expanded: docs.isDirExpanded(child.path) }"
                  >
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                  </svg>
                  <span>{{ child.name }}</span>
                </template>
                <template v-else>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>
                  <span>{{ child.title || child.name }}</span>
                </template>
              </button>
            </div>
          </div>
          <!-- File node at root level -->
          <button
            v-else
            class="doc-item"
            :class="{ active: route.params.name === node.path }"
            @click="selectDoc(node.path)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>
            <span>{{ node.title || node.name }}</span>
          </button>
        </template>
      </div>

      <!-- Resize handle -->
      <div class="resize-handle" @mousedown="startResize" />
    </aside>

    <!-- Collapse toggle -->
    <button v-show="docs.sidebarCollapsed" class="expand-btn icon-btn" @click="docs.toggleSidebar()">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="13 17 18 12 13 7" /><polyline points="6 17 11 12 6 7" /></svg>
    </button>

    <!-- Content -->
    <div ref="contentEl" class="docs-content" @click="handleContentClick">
      <div v-if="docs.loading" class="loading-state">
        <span class="spinner" /> Loading...
      </div>

      <div v-else-if="docs.currentDocContent" class="doc-rendered">
        <div v-html="renderMd(docs.currentDocContent)" />
      </div>

      <!-- Catalog view -->
      <div v-else class="catalog">
        <h1 class="catalog-title">Documentation Library</h1>
        <p class="catalog-subtitle">Browse migration guides, annotation references, and conversion examples.</p>
        <div class="catalog-grid">
          <template v-for="node in docs.docTree" :key="node.path || node.name">
            <!-- Directory card -->
            <div v-if="node.isDir" class="catalog-dir glass-card">
              <div class="catalog-dir-header" @click="docs.toggleDir(node.path)">
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  class="dir-icon"
                  :class="{ expanded: docs.isDirExpanded(node.path) }"
                >
                  <polyline points="9 18 15 12 9 6" />
                </svg>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                </svg>
                <h3>{{ node.name }}</h3>
              </div>
              <div v-if="docs.isDirExpanded(node.path)" class="catalog-dir-children">
                <button
                  v-for="child in node.children"
                  :key="child.path || child.name"
                  class="catalog-card glass-card"
                  @click="selectDoc(child.path)"
                >
                  <h3>{{ child.title || child.name }}</h3>
                  <p class="doc-filename">{{ child.path }}</p>
                </button>
              </div>
            </div>
            <!-- File card -->
            <button
              v-else
              class="catalog-card glass-card"
              @click="selectDoc(node.path)"
            >
              <h3>{{ node.title || node.name }}</h3>
              <p class="doc-filename">{{ node.path }}</p>
            </button>
          </template>
        </div>
      </div>
    </div>

    <!-- TOC sidebar -->
    <aside v-if="tocItems.length > 0" class="toc-sidebar">
      <div class="toc-title">On this page</div>
      <nav class="toc-list">
        <a
          v-for="(item, i) in tocItems"
          :key="i"
          class="toc-item"
          :class="'toc-level-' + item.level"
          :href="'#' + item.id"
          @click.prevent="scrollToHeading(item.id)"
        >
          {{ item.text }}
        </a>
      </nav>
    </aside>
  </div>
</template>

<style scoped>
.docs-page {
  display: flex;
  height: calc(100vh - 56px);
  overflow-x: hidden;
}

.docs-page.resizing {
  cursor: col-resize;
  user-select: none;
}

.docs-sidebar {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  border-radius: 0;
  border-right: 0.5px solid var(--border);
  border-top: none;
  border-bottom: none;
  border-left: none;
  position: relative;
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 0.5px solid var(--border);
  flex-shrink: 0;
}

.sidebar-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.sidebar-search {
  padding: 8px 10px;
  flex-shrink: 0;
}

.sidebar-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 6px;
}

.dir-node {
  margin-bottom: 2px;
}

.dir-item {
  font-weight: 500;
  color: var(--text);
}

.dir-icon {
  transition: transform 0.15s ease;
}

.dir-icon.expanded {
  transform: rotate(90deg);
}

.dir-children {
  padding-left: 12px;
  border-left: 1px solid var(--border);
  margin-left: 18px;
}

.doc-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  background: none;
  color: var(--text-muted);
  font-family: var(--font-sans);
  font-size: 0.82rem;
  text-align: left;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.12s;
}

.doc-item:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.doc-item.active {
  background: var(--bg-hover);
  color: var(--accent);
  font-weight: 500;
}

.resize-handle {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 4px;
  cursor: col-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: var(--accent);
}

.expand-btn {
  position: absolute;
  left: 8px;
  top: 64px;
  z-index: 10;
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
  transition: all 0.12s;
}

.icon-btn:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.docs-content {
  flex: 1;
  overflow-y: auto;
  min-width: 0;
  padding: 24px 32px;
}

.doc-rendered {
  max-width: 860px;
  font-size: 0.92rem;
  line-height: 1.75;
}

/* h1 — subtle gradient, less saturated */
.doc-rendered :deep(h1) {
  font-size: 1.8rem;
  font-weight: 700;
  margin: 0 0 20px;
  padding-bottom: 12px;
  border-bottom: 2px solid var(--border);
  color: var(--text);
}

.doc-rendered :deep(h2) {
  font-size: 1.4rem;
  font-weight: 700;
  margin: 36px 0 14px;
  padding-bottom: 8px;
  padding-left: 12px;
  border-bottom: 0.5px solid var(--border);
  border-left: 3px solid var(--text-muted);
  color: var(--text);
}

.doc-rendered :deep(h3) {
  font-size: 1.15rem;
  font-weight: 600;
  margin: 24px 0 10px;
  color: var(--text);
}

.doc-rendered :deep(h4) {
  font-size: 1rem;
  font-weight: 600;
  margin: 18px 0 8px;
  color: var(--text-muted);
}

/* Paragraphs */
.doc-rendered :deep(p) {
  margin: 10px 0;
  line-height: 1.8;
  color: var(--text);
}

/* Bold */
.doc-rendered :deep(strong) {
  font-weight: 600;
  color: var(--text);
}

/* Italic */
.doc-rendered :deep(em) {
  color: var(--text-muted);
  font-style: italic;
}

/* Inline code - muted, warm tone */
.doc-rendered :deep(.inline-code) {
  font-family: var(--font-mono);
  font-size: 0.84em;
  padding: 2px 7px;
  border-radius: 5px;
  background: rgba(128,128,128,0.08);
  color: var(--text);
  border: 0.5px solid rgba(128,128,128,0.12);
  font-weight: 500;
}

[data-theme="light"] .doc-rendered :deep(.inline-code) {
  background: rgba(0,0,0,0.04);
  color: var(--text);
  border-color: rgba(0,0,0,0.08);
}

/* Code blocks - dark background with syntax feel */
.doc-rendered :deep(.code-block) {
  margin: 16px 0;
  padding: 16px 18px;
  background: #1a1a2e;
  border: 0.5px solid rgba(255,255,255,0.06);
  border-radius: 10px;
  overflow-x: auto;
  font-size: 0.84rem;
  line-height: 1.65;
}

[data-theme="light"] .doc-rendered :deep(.code-block) {
  background: #f8f9fb;
  border-color: rgba(0,0,0,0.08);
}

.doc-rendered :deep(.code-block code) {
  color: #e0e0e6;
  font-family: var(--font-mono);
}

[data-theme="light"] .doc-rendered :deep(.code-block code) {
  color: #2d3436;
}

/* Syntax highlighting in code blocks */
.doc-rendered :deep(.hl-key) {
  color: #c792ea;
  font-weight: 500;
}
[data-theme="light"] .doc-rendered :deep(.hl-key) {
  color: #8e44ad;
}

.doc-rendered :deep(.hl-string) {
  color: #c3e88d;
}
[data-theme="light"] .doc-rendered :deep(.hl-string) {
  color: #27ae60;
}

.doc-rendered :deep(.hl-comment) {
  color: #697098;
  font-style: italic;
}
[data-theme="light"] .doc-rendered :deep(.hl-comment) {
  color: #95a5a6;
}

.doc-rendered :deep(.hl-bool) {
  color: #f78c6c;
  font-weight: 500;
}
[data-theme="light"] .doc-rendered :deep(.hl-bool) {
  color: #d35400;
}

.doc-rendered :deep(.hl-number) {
  color: #89ddff;
  font-weight: 500;
}
[data-theme="light"] .doc-rendered :deep(.hl-number) {
  color: #0984e3;
}

.doc-rendered :deep(.hl-list) {
  color: #ffcb6b;
}
[data-theme="light"] .doc-rendered :deep(.hl-list) {
  color: #e67e22;
}

.doc-rendered :deep(.hl-anchor) {
  color: #82aaff;
  text-decoration: underline;
}
[data-theme="light"] .doc-rendered :deep(.hl-anchor) {
  color: #3498db;
}

.doc-rendered :deep(.hl-sep) {
  color: #697098;
  font-weight: 700;
}

/* Tables - alternating rows with color */
.doc-rendered :deep(.doc-table) {
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0;
  font-size: 0.86rem;
  border-radius: 8px;
  overflow: hidden;
  border: 0.5px solid var(--border);
}

.doc-rendered :deep(th) {
  padding: 10px 14px;
  background: rgba(128,128,128,0.08);
  color: var(--text-muted);
  font-weight: 600;
  font-size: 0.82rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
  border-bottom: 1px solid var(--border);
  text-align: left;
}

[data-theme="light"] .doc-rendered :deep(th) {
  background: rgba(0,0,0,0.03);
  color: var(--text-muted);
}

.doc-rendered :deep(td) {
  padding: 9px 14px;
  border-bottom: 0.5px solid var(--border);
  text-align: left;
}

.doc-rendered :deep(.row-even) {
  background: transparent;
}

.doc-rendered :deep(.row-odd) {
  background: var(--bg-hover);
}

.doc-rendered :deep(tr:hover td) {
  background: rgba(128,128,128,0.04);
}

/* Blockquotes - left border accent */
.doc-rendered :deep(.doc-quote) {
  margin: 14px 0;
  padding: 12px 16px;
  border-left: 3px solid var(--orange);
  background: rgba(255,159,10,0.05);
  border-radius: 0 8px 8px 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

[data-theme="light"] .doc-rendered :deep(.doc-quote) {
  background: rgba(255,149,0,0.04);
}

/* Lists */
.doc-rendered :deep(.doc-list) {
  margin: 10px 0;
  padding-left: 20px;
  list-style: none;
}

.doc-rendered :deep(.doc-li) {
  position: relative;
  padding: 3px 0 3px 14px;
  line-height: 1.7;
}

.doc-rendered :deep(.doc-li::before) {
  content: '';
  position: absolute;
  left: 0;
  top: 12px;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--accent);
}

.doc-rendered :deep(.doc-li-ordered) {
  counter-increment: list-counter;
}

.doc-rendered :deep(.doc-li-ordered::before) {
  content: counter(list-counter) '.';
  background: none;
  width: auto;
  height: auto;
  border-radius: 0;
  color: var(--accent);
  font-weight: 600;
  font-size: 0.84em;
  top: 4px;
}

/* Horizontal rule */
.doc-rendered :deep(.doc-hr) {
  border: none;
  height: 1px;
  margin: 28px 0;
  background: linear-gradient(90deg, transparent, var(--border), var(--accent-glow), var(--border), transparent);
}

/* Images */
.doc-rendered :deep(.doc-img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 12px 0;
  border: 0.5px solid var(--border);
}

/* Links */
.doc-rendered :deep(a) {
  color: var(--accent);
  text-decoration: none;
  border-bottom: 1px solid transparent;
  transition: border-color 0.15s;
}

.doc-rendered :deep(a:hover) {
  border-bottom-color: var(--accent);
}

/* Catalog */
.catalog {
  max-width: 900px;
  margin: 0 auto;
}

.catalog-title {
  font-size: 2rem;
  font-weight: 700;
  margin-bottom: 8px;
}

.catalog-subtitle {
  color: var(--text-muted);
  font-size: 1rem;
  margin-bottom: 28px;
}

.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.catalog-card {
  padding: 18px;
  text-align: left;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
  font-family: var(--font-sans);
  color: var(--text);
}

.catalog-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 32px rgba(0,0,0,0.15);
}

.catalog-card h3 {
  font-size: 1rem;
  margin-bottom: 6px;
}

.catalog-card p {
  font-size: 0.82rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.catalog-dir {
  grid-column: 1 / -1;
  padding: 0;
  overflow: hidden;
}

.catalog-dir-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 18px;
  cursor: pointer;
  transition: background 0.15s;
}

.catalog-dir-header:hover {
  background: var(--bg-hover);
}

.catalog-dir-header h3 {
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
}

.catalog-dir-children {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 10px;
  padding: 0 18px 14px 40px;
}

.doc-size {
  display: inline-block;
  margin-top: 8px;
  font-size: 0.72rem;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  padding: 40px;
}

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

/* TOC sidebar */
.toc-sidebar {
  width: 220px;
  flex-shrink: 0;
  padding: 20px 14px;
  overflow-y: auto;
  border-left: 0.5px solid var(--border);
}

.toc-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 12px;
}

.toc-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.toc-item {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-decoration: none;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.12s;
  line-height: 1.4;
  border-left: 2px solid transparent;
}

.toc-item:hover {
  color: var(--text);
  background: var(--bg-hover);
}

.toc-level-2 {
  padding-left: 16px;
}

.toc-level-3 {
  padding-left: 28px;
  font-size: 0.76rem;
}

/* Mermaid diagrams */
.doc-rendered :deep(.mermaid-placeholder) {
  margin: 16px 0;
  text-align: center;
  overflow-x: auto;
  padding: 16px;
  background: var(--bg-hover);
  border-radius: 10px;
  border: 0.5px solid var(--border);
  font-family: var(--font-mono);
  font-size: 0.85rem;
  white-space: pre-wrap;
}

.doc-rendered :deep(.mermaid) {
  margin: 16px 0;
  text-align: center;
  overflow-x: auto;
  padding: 16px;
  background: var(--bg-hover);
  border-radius: 10px;
  border: 0.5px solid var(--border);
}

.doc-rendered :deep(.mermaid svg) {
  max-width: 100%;
  height: auto;
}

.doc-rendered :deep(.mermaid-error) {
  color: #e74c3c;
  background: rgba(231, 76, 60, 0.05);
  padding: 12px;
  border-radius: 8px;
  font-size: 0.85rem;
  font-family: var(--font-mono);
  white-space: pre-wrap;
  word-break: break-word;
  margin: 16px 0;
}
</style>
