// API response types
export interface ConvertRequest {
  yaml: string
  target: 'APISIX' | 'GatewayAPI'
  sslRedirect: boolean
}

export interface ConvertResponse {
  output: string
  warnings: string[]
  stats: ConvertStats
}

export interface ConvertStats {
  ingressCount: number
  routeCount: number
  upstreamCount: number
  pluginConfigCount: number
  tlsCount: number
}

export interface CheckRequest {
  yaml: string
}

export interface CheckResponse {
  report: AnnotationFinding[]
  warnings: string[]
}

export interface AnnotationFinding {
  ingress: string
  annotation: string
  value: string
  status: 'auto' | 'manual' | 'plugin' | 'aic-native' | 'unsupported' | 'unknown'
  note: string
}

export interface DocItem {
  name: string
  title: string
  description: string
  size: number
  modTime: string
}

export interface DocContent {
  name: string
  title: string
  content: string
}

export interface Announcement {
  id: string
  title: string
  content: string
  level: 'info' | 'warning' | 'error'
  active: boolean
  createdAt: string
  updatedAt: string
}

export interface Feedback {
  id: string
  category: string
  title: string
  content: string
  contact: string
  createdAt: string
}

// Document manager types
export interface ManagedDocument {
  id: string
  name: string
  filename: string
  size: number
  uploadedAt: string
  ingressCount: number
  status: 'pending' | 'converted' | 'error'
  tags: string[]
}

// Plugin template types
export interface PluginTemplate {
  id: string
  name: string
  pluginName: string
  description: string
  config: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface ConfigTemplate {
  id: string
  name: string
  description: string
  yaml: string
  createdAt: string
  updatedAt: string
}

// Auth types
export interface User {
  username: string
  role: 'admin' | 'user'
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}
