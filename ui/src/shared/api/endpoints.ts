/**
 * Single source of truth cho tất cả API endpoints.
 * Tất cả feature API modules import từ đây.
 */
export const ENDPOINTS = {
  // ─── Auth ──────────────────────────────────────────────────────────────
  auth: {
    login:         '/api/v1/auth/login',
    refresh:       '/api/v1/auth/refresh',
    logout:        '/api/v1/auth/logout',
    me:            '/api/v1/auth/me',
    mfaSetup:      '/api/v1/auth/mfa/setup',
    mfaConfirm:    '/api/v1/auth/mfa/confirm',
    oauthGoogle:   '/api/v1/auth/oauth/google',
    oauthGitHub:   '/api/v1/auth/oauth/github',
    oauthCallback: '/api/v1/auth/callback',
  },

  // ─── Dashboard ─────────────────────────────────────────────────────────
  dashboard: {
    metrics: '/api/v1/dashboard',
    sla:     '/api/v1/dashboard/sla',
  },

  // ─── Notifications ─────────────────────────────────────────────────────
  notifications: {
    stream:      '/api/v1/notifications/stream',
    list:        '/api/v1/notifications',
    unreadCount: '/api/v1/notifications/unread-count',
    markRead:    (id: string) => `/api/v1/notifications/${id}/read`,
    markAllRead: '/api/v1/notifications/mark-all-read',
  },

  // ─── CVE Intelligence (v2) ─────────────────────────────────────────────
  cve: {
    search:       '/api/v2/cves/search',
    semantic:     '/api/v2/cves/search/semantic',
    detail:       (id: string) => `/api/v2/cves/${id}`,
    aggregations: '/api/v2/cves/aggregations',
    export:       '/api/v2/cves/export',
  },
  kev: {
    list:       '/api/v2/kev',
    stats:      '/api/v2/kev/stats',
    ransomware: '/api/v2/kev/ransomware',
  },
  epss: {
    byCve:        (cveId: string) => `/api/v2/epss/${cveId}`,
    top:          '/api/v2/epss/top',
    distribution: '/api/v2/epss/distribution',
  },
  cwe: {
    list:   '/api/v2/cwe',
    detail: (id: string) => `/api/v2/cwe/${id}`,
  },
  capec: {
    list:   '/api/v2/capec',
    detail: (id: string) => `/api/v2/capec/${id}`,
  },
  vendors: '/api/v2/vendors',
  browse: {
    root:      '/api/v2/browse',
    byVendor:  (vendor: string) => `/api/v2/browse/${encodeURIComponent(vendor)}`,
    byProduct: (vendor: string, product: string) =>
      `/api/v2/browse/${encodeURIComponent(vendor)}/${encodeURIComponent(product)}`,
  },
  dbinfo: '/api/v2/dbinfo',

  // ─── Scans (v1) ────────────────────────────────────────────────────────
  scans: {
    list:      '/api/v1/scans',
    create:    '/api/v1/scans',
    history:   '/api/v1/scans/history',
    detail:    (id: string) => `/api/v1/scans/${id}`,
    stream:    (id: string) => `/api/v1/scans/${id}/stream`,
    cancel:    (id: string) => `/api/v1/scans/${id}/cancel`,
    nmap:      (id: string) => `/api/v1/scans/${id}/results/nmap`,
    zap:       (id: string) => `/api/v1/scans/${id}/results/zap`,
    scheduled: '/api/v1/scans/scheduled',
    import:    '/api/v1/scans/import',
  },

  // ─── Findings (v1) ─────────────────────────────────────────────────────
  findings: {
    list:       '/api/v1/findings',
    stats:      '/api/v1/findings/stats',
    detail:     (id: string) => `/api/v1/findings/${id}`,
    patch:      (id: string) => `/api/v1/findings/${id}`,
    notes:      (id: string) => `/api/v1/findings/${id}/notes`,
    audit:      (id: string) => `/api/v1/findings/${id}/audit`,
    bulkClose:  '/api/v1/findings/bulk/close',
    bulkReopen: '/api/v1/findings/bulk/reopen',
    bulkAssign: '/api/v1/findings/bulk/assign',
  },
  riskAcceptances: {
    list:   '/api/v1/risk-acceptances',
    create: '/api/v1/risk-acceptances',
    delete: (id: string) => `/api/v1/risk-acceptances/${id}`,
  },
  sla: {
    config:   '/api/v1/sla/config',
    overview: '/api/v1/sla/overview',
  },

  // ─── Assets (v1) ───────────────────────────────────────────────────────
  assets: {
    list:     '/api/v1/assets',
    detail:   (id: string) => `/api/v1/assets/${id}`,
    findings: (id: string) => `/api/v1/assets/${id}/findings`,
    patch:    (id: string) => `/api/v1/assets/${id}`,
    tags:     '/api/v1/assets/tags',
  },

  // ─── Products (v1) ─────────────────────────────────────────────────────
  products: {
    list:        '/api/v1/products',
    create:      '/api/v1/products',
    detail:      (id: string) => `/api/v1/products/${id}`,
    patch:       (id: string) => `/api/v1/products/${id}`,
    engagements: (id: string) => `/api/v1/products/${id}/engagements`,
    grades:      '/api/v1/products/grades',
    types:       '/api/v1/products/types',
  },
  engagements: {
    tests: (engId: string) => `/api/v1/engagements/${engId}/tests`,
  },

  // ─── AI (v1) ───────────────────────────────────────────────────────────
  ai: {
    triage:        (findingId: string) => `/api/v1/ai/triage/${findingId}`,
    triageReview:  (findingId: string) => `/api/v1/ai/triage/${findingId}/review`,
    triageQueue:   '/api/v1/ai/triage/queue',
    enrichment:    '/api/v1/ai/enrichment',
    enrichTrigger: '/api/v1/ai/enrichment/trigger',
    enrichByCve:   (cveId: string) => `/api/v1/ai/enrichment/${cveId}`,
    insights:      '/api/v1/ai/insights',
  },

  // ─── Reports (v1) ──────────────────────────────────────────────────────
  reports: {
    list:      '/api/v1/reports',
    templates: '/api/v1/reports/templates',
    create:    '/api/v1/reports',
    detail:    (id: string) => `/api/v1/reports/${id}`,
    download:  (id: string) => `/api/v1/reports/${id}/download`,
    delete:    (id: string) => `/api/v1/reports/${id}`,
  },

  // ─── Webhooks (v1) ─────────────────────────────────────────────────────
  webhooks: {
    list:          '/api/v1/webhooks',
    create:        '/api/v1/webhooks',
    delete:        (id: string) => `/api/v1/webhooks/${id}`,
    test:          (id: string) => `/api/v1/webhooks/${id}/test`,
    deliveries:    '/api/v1/webhooks/deliveries',
    deliveryStats: '/api/v1/webhooks/stats/hourly',
    retryDelivery: (id: string) => `/api/v1/webhooks/deliveries/${id}/retry`,
  },

  // ─── API Keys (v1) ─────────────────────────────────────────────────────
  apiKeys: {
    list:   '/api/v1/api-keys',
    create: '/api/v1/api-keys',
    revoke: (id: string) => `/api/v1/api-keys/${id}`,
  },

  // ─── JIRA / Integrations (v1) ────────────────────────────────────────────
  jira: {
    config: '/api/v1/jira/config',
    test:   '/api/v1/jira/config/test',
  },
  integrations: {
    // Actual path used by JiraConfig.tsx component (distinct from /jira/config above)
    jira: '/api/v1/integrations/jira',
  },

  // ─── Profile (v1) ──────────────────────────────────────────────────────
  profile: {
    get:                  '/api/v1/profile',
    patch:                '/api/v1/profile',
    changePassword:       '/api/v1/profile/change-password',
    sessions:             '/api/v1/profile/sessions',
    notificationSettings: '/api/v1/profile/notifications/settings',
  },

  // ─── Search (v1) ───────────────────────────────────────────────────────
  search: {
    recent:      '/api/v1/search/recent',
    suggested:   '/api/v1/search/suggested',
    semanticSuggestions: '/api/v2/cves/search/semantic/suggestions',
  },

  // ─── Admin (v1) ────────────────────────────────────────────────────────
  admin: {
    users:      '/api/v1/admin/users',
    userDetail: (id: string) => `/api/v1/admin/users/${id}`,
    userInvite: '/api/v1/admin/users/invite',
    userUnlock: (id: string) => `/api/v1/admin/users/${id}/unlock`,
    userReset:  (id: string) => `/api/v1/admin/users/${id}/reset-password`,
    roles:      '/api/v1/admin/roles',
    health:     '/api/v1/admin/health',
    settings:   '/api/v1/admin/settings',
  },

  // ─── Audit (v1) ────────────────────────────────────────────────────────
  audit: {
    log: '/api/v1/audit-log',
  },
} as const;
