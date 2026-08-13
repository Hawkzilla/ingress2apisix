import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/docs',
  },
  {
    path: '/convert',
    name: 'convert',
    component: () => import('@/views/ConvertPage.vue'),
    meta: { title: 'Convert' },
  },
  {
    path: '/check',
    name: 'check',
    component: () => import('@/views/CheckPage.vue'),
    meta: { title: 'Check Annotations' },
  },
  {
    path: '/docs',
    name: 'docs',
    component: () => import('@/views/DocsPage.vue'),
    meta: { title: 'Documentation' },
  },
  {
    path: '/docs/:name(.*)',
    name: 'doc-detail',
    component: () => import('@/views/DocsPage.vue'),
    meta: { title: 'Documentation' },
  },
  {
    path: '/documents',
    name: 'documents',
    component: () => import('@/views/DocumentsPage.vue'),
    meta: { title: 'Document Manager' },
  },
  {
    path: '/admin',
    name: 'admin',
    component: () => import('@/views/AdminPage.vue'),
    meta: { title: 'Admin' },
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginPage.vue'),
    meta: { title: 'Login' },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/docs',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Support legacy hash URLs: /#/docs/xxx → /docs/xxx, /#convert → /convert
// Also handles: /#docs/xxx, /#/docs/xxx#section, /#convert#anchor, etc.
router.beforeEach((to, _from, next) => {
  const fullHash = window.location.hash
  if (!fullHash) {
    next()
    return
  }

  const rawHash = fullHash.slice(1) // remove leading #
  if (!rawHash) {
    next()
    return
  }

  let hashPath = ''
  let hashAnchor = ''

  if (rawHash.startsWith('/')) {
    // Pattern: #/docs/xxx or #/docs/xxx#section
    const anchorIdx = rawHash.indexOf('#', 1)
    if (anchorIdx !== -1) {
      hashPath = rawHash.slice(0, anchorIdx)
      hashAnchor = rawHash.slice(anchorIdx)
    } else {
      hashPath = rawHash
    }
  } else {
    // Pattern: #docs/xxx, #convert, #check, or with anchor: #docs#section
    const anchorIdx = rawHash.indexOf('#', 1)
    const segment = anchorIdx !== -1 ? rawHash.slice(0, anchorIdx) : rawHash
    hashPath = '/' + segment
    if (anchorIdx !== -1) {
      hashAnchor = rawHash.slice(anchorIdx)
    }
  }

  if (hashPath && hashPath !== '/' && hashPath !== to.path) {
    window.location.hash = ''
    next({ path: hashPath + hashAnchor, replace: true })
    return
  }

  next()
})

router.afterEach((to) => {
  const title = to.meta.title as string | undefined
  if (title) {
    document.title = `${title} — ingress2apisix`
  }
})

export default router
