import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor - attach auth token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('veer_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
)

// Response interceptor for error handling and 401 redirect
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('veer_token')
      window.location.href = '/login'
      return Promise.reject(new Error('未授权，请重新登录'))
    }
    const message = error.response?.data?.error || error.message || '请求失败'
    return Promise.reject(new Error(message))
  }
)

// ===== CDN Nodes API =====
export const nodesApi = {
  /** Get all CDN nodes */
  list: () => api.get('/nodes'),

  /** Create a new CDN node */
  create: (data) => api.post('/nodes', data),

  /** Update an existing CDN node */
  update: (id, data) => api.put(`/nodes/${id}`, data),

  /** Delete a CDN node */
  delete: (id) => api.delete(`/nodes/${id}`),

  /** Batch delete CDN nodes */
  batchDelete: (ids) => api.delete('/nodes/batch', { data: { ids } }),

  /** Batch update node status */
  batchUpdateStatus: (ids, status) => api.put('/nodes/batch/status', { ids, status }),

  /** Test node health (measures latency) */
  test: (id) => api.post(`/nodes/${id}/test`),
}

// ===== Redirect Rules API =====
export const rulesApi = {
  /** Get all redirect rules (optional filter by rule_type/enabled) */
  list: (params = {}) => api.get('/rules', { params }),

  /** Create a new redirect rule */
  create: (data) => api.post('/rules', data),

  /** Update an existing redirect rule */
  update: (id, data) => api.put(`/rules/${id}`, data),

  /** Delete a redirect rule */
  delete: (id) => api.delete(`/rules/${id}`),

  /** Batch delete redirect rules */
  batchDelete: (ids) => api.delete('/rules/batch', { data: { ids } }),

  /** Batch enable/disable rules */
  batchToggle: (ids, enabled) => api.put('/rules/batch/toggle', { ids, enabled }),

  /** Reorder rules by priority */
  reorder: (ids, ruleType) => api.put('/rules/reorder', { ids, rule_type: ruleType }),
}

// ===== Statistics API =====
export const statsApi = {
  /** Get dashboard overview stats */
  overview: () => api.get('/stats/overview'),

  /** Get paginated access logs */
  logs: (params = {}) => api.get('/stats/logs', { params }),

  /** Get 7-day traffic trend */
  traffic: () => api.get('/stats/traffic'),
}

// ===== Clusters API =====
export const clustersApi = {
  list: (params = {}) => api.get('/clusters', { params }),
  create: (data) => api.post('/clusters', data),
  update: (id, data) => api.put(`/clusters/${id}`, data),
  delete: (id) => api.delete(`/clusters/${id}`),
  get: (id) => api.get(`/clusters/${id}`),
  listNodes: (id, params = {}) => api.get(`/clusters/${id}/nodes`, { params }),
  setNodes: (id, nodeIds) => api.put(`/clusters/${id}/nodes`, { node_ids: nodeIds }),
  listRules: (id) => api.get(`/clusters/${id}/rules`),
  stats: (id) => api.get(`/clusters/${id}/stats`),
}

// ===== Views API =====
export const viewsApi = {
  topology: () => api.get('/views/topology'),
  healthMatrix: () => api.get('/views/health-matrix'),
  trafficDistribution: () => api.get('/views/traffic-distribution'),
}

export default api
