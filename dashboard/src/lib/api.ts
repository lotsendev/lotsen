export class UnauthorizedError extends Error {
  constructor() {
    super('Unauthorized')
    this.name = 'UnauthorizedError'
  }
}

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401) throw new UnauthorizedError()
  return res
}

export type AuthStatus = 'authenticated' | 'unauthenticated' | 'disabled'

export type MeResponse = {
  status: AuthStatus
  username?: string
}

export async function getMe(): Promise<MeResponse> {
  const res = await fetch('/auth/me')
  if (res.ok) return { status: 'authenticated', ...(await res.json()) }
  if (res.status === 401) return { status: 'unauthenticated' }
  if (res.status === 503) return { status: 'disabled' }
  throw new Error('Failed to check auth status')
}

export async function login(username: string, password: string): Promise<void> {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (res.status === 401) throw new UnauthorizedError()
  if (!res.ok) throw new Error('Login failed')
}

export async function logout(): Promise<void> {
  await fetch('/auth/logout', { method: 'POST' })
}

export type DeploymentStatus = 'idle' | 'deploying' | 'healthy' | 'failed'

export type BasicAuthUser = {
  username: string
  password: string
}

export type BasicAuthConfig = {
  users: BasicAuthUser[]
}

export type SecurityConfig = {
  waf_enabled: boolean
  waf_mode: 'detection' | 'enforcement'
  ip_denylist: string[]
  ip_allowlist: string[]
  custom_rules: string[]
}

export type Deployment = {
  id: string
  name: string
  image: string
  envs: Record<string, string>
  ports: string[]
  volumes: string[]
  domain: string
  public: boolean
  basic_auth?: BasicAuthConfig
  security?: SecurityConfig
  status: DeploymentStatus
  error?: string
  stats?: ContainerStats
}

export type ContainerStats = {
  cpuPercent: number
  memoryUsedBytes: number
  memoryLimitBytes: number
  memoryPercent: number
}

export type StatusEvent = {
  deploymentId: string
  status: DeploymentStatus
  error?: string
}

export type DeploymentLogEvent = {
  line: string
}

export type VersionInfo = {
  currentVersion: string
  latestVersion?: string
  releaseNotes?: string
  publishedAt?: string
  upgradeAvailable: boolean
  cachedAt?: string
}

export type VersionRelease = {
  version: string
  releaseNotes: string
  publishedAt?: string
}

export type UpgradeLogEvent = {
  line: string
}

export type SystemStatusState = 'healthy' | 'degraded' | 'unavailable'

export type APISystemStatus = {
  state: SystemStatusState
  lastUpdated: string
  checks?: {
    processRunning: boolean
    dashboardReachable: boolean
    storeAccessible: boolean
  }
}

export type OrchestratorSystemStatus = {
  state: SystemStatusState
  lastUpdated?: string
  checks?: {
    processRunning: boolean
    dockerReachable: boolean
    storeAccessible: boolean
  }
}

export type LoadBalancerSystemStatus = {
  state: SystemStatusState
  lastUpdated?: string
  checks?: {
    processRunning: boolean
    healthcheckResponding: boolean
  }
  traffic?: {
    totalRequests: number
    suspiciousRequests: number
    blockedRequests: number
    wafBlockedRequests?: number
    uaBlockedRequests?: number
    activeBlockedIps: number
    blockedIps?: Array<{
      ip: string
      blockedUntil?: string
    }>
  }
}

export type ProxySecurityConfig = {
  profile: string
  suspiciousWindowSeconds: number
  suspiciousThreshold: number
  suspiciousBlockForSeconds: number
  globalIpDenylist?: string[]
  globalIpAllowlist?: string[]
}

export type DockerSystemStatus = {
  state: SystemStatusState
  lastUpdated?: string
  checks?: {
    daemonHealthy: boolean
  }
}

export type HostMetricSystemStatus = {
  state: SystemStatusState
  usagePercent?: number
  lastUpdated?: string
}

export type HostSystemStatus = {
  cpu: HostMetricSystemStatus
  ram: HostMetricSystemStatus
}

export type SystemStatusSnapshot = {
  api: APISystemStatus
  orchestrator: OrchestratorSystemStatus
  loadBalancer: LoadBalancerSystemStatus
  docker: DockerSystemStatus
  host: HostSystemStatus
  error?: string
}

export type LoadBalancerAccessLogEntry = {
  timestamp: string
  clientIp: string
  host: string
  method: string
  path: string
  query?: string
  status: number
  durationMs: number
  bytesWritten: number
  outcome: string
  headers?: Record<string, string>
}

export type LoadBalancerAccessLogsPage = {
  items: LoadBalancerAccessLogEntry[]
  hasMore: boolean
  nextCursor?: string
}

export type LoadBalancerAccessLogFilters = {
  method?: string
  status?: number
  host?: string
  ip?: string
}

export type CoreService = 'api' | 'orchestrator' | 'proxy'

export type CoreServiceLogsResponse = {
  service: CoreService
  lines: string[]
}

export type DeploymentRecentLogsResponse = {
  deploymentId: string
  lines: string[]
}

export async function getDeployments(): Promise<Deployment[]> {
  const res = await apiFetch('/api/deployments')
  if (!res.ok) throw new Error('Failed to fetch deployments')
  return res.json()
}

export type CreateDeploymentInput = {
  name: string
  image: string
  envs: Record<string, string>
  ports: string[]
  volumes: string[]
  domain: string
  basic_auth?: BasicAuthConfig
}

export async function createDeployment(data: CreateDeploymentInput): Promise<Deployment> {
  const res = await apiFetch('/api/deployments', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to create deployment')
  return res.json()
}

export type UpdateDeploymentInput = {
  name: string
  image: string
  envs: Record<string, string>
  ports: string[]
  volumes: string[]
  domain: string
  basic_auth?: BasicAuthConfig
  security?: SecurityConfig
}

export async function updateDeployment(id: string, data: UpdateDeploymentInput): Promise<Deployment> {
  const res = await apiFetch(`/api/deployments/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to update deployment')
  return res.json()
}

export type PatchDeploymentInput = {
  image?: string
  envs?: Record<string, string>
  ports?: string[]
  volumes?: string[]
  domain?: string
  basic_auth?: BasicAuthConfig
  security?: SecurityConfig
}

export async function patchDeployment(id: string, data: PatchDeploymentInput): Promise<Deployment> {
  const res = await apiFetch(`/api/deployments/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to patch deployment')
  return res.json()
}

export async function deleteDeployment(id: string): Promise<void> {
  const res = await apiFetch(`/api/deployments/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to delete deployment')
}

export async function restartDeployment(id: string): Promise<Deployment> {
  const res = await apiFetch(`/api/deployments/${id}/restart`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to restart deployment')
  return res.json()
}

export async function getSystemStatus(): Promise<SystemStatusSnapshot> {
  const res = await apiFetch('/api/system-status')
  if (!res.ok) throw new Error('Failed to fetch system status')
  return res.json()
}

export async function getLoadBalancerAccessLogs(
  cursor?: string,
  limit = 100,
  filters?: LoadBalancerAccessLogFilters
): Promise<LoadBalancerAccessLogsPage> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (cursor) {
    params.set('cursor', cursor)
  }
  if (filters?.method) {
    params.set('method', filters.method)
  }
  if (typeof filters?.status === 'number') {
    params.set('status', String(filters.status))
  }
  if (filters?.host) {
    params.set('host', filters.host)
  }
  if (filters?.ip) {
    params.set('ip', filters.ip)
  }

  const res = await apiFetch(`/api/load-balancer/access-logs?${params.toString()}`)
  if (!res.ok) throw new Error('Failed to fetch load balancer access logs')
  return res.json()
}

export async function getCoreServiceLogs(service: CoreService, tail = 200): Promise<CoreServiceLogsResponse> {
  const params = new URLSearchParams({ service, tail: String(tail) })
  const res = await apiFetch(`/api/core-services/logs?${params.toString()}`)
  if (!res.ok) throw new Error('Failed to fetch core service logs')
  return res.json()
}

export async function getDeploymentRecentLogs(deploymentId: string, tail = 300): Promise<DeploymentRecentLogsResponse> {
  const params = new URLSearchParams({ tail: String(tail) })
  const res = await apiFetch(`/api/deployments/${deploymentId}/logs/recent?${params.toString()}`)
  if (!res.ok) throw new Error('Failed to fetch deployment recent logs')
  return res.json()
}

export async function getVersionInfo(options?: { forceRefresh?: boolean }): Promise<VersionInfo> {
  const suffix = options?.forceRefresh ? "?refresh=1" : ""
  const res = await apiFetch(`/api/version${suffix}`)
  if (!res.ok) throw new Error('Failed to fetch version info')
  return res.json()
}

export async function getSecurityConfig(): Promise<ProxySecurityConfig> {
  const res = await apiFetch('/api/security-config')
  if (!res.ok) throw new Error('Failed to fetch security config')
  return res.json()
}

export async function getVersionReleases(limit = 25): Promise<VersionRelease[]> {
  const res = await apiFetch(`/api/version/releases?limit=${limit}`)
  if (!res.ok) throw new Error('Failed to fetch version releases')
  return res.json()
}

export async function triggerUpgrade(targetVersion?: string): Promise<void> {
  const body = targetVersion ? JSON.stringify({ targetVersion }) : undefined
  const res = await apiFetch('/api/upgrade', {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body,
  })
  if (res.status === 409) throw new Error('Upgrade already in progress')
  if (!res.ok) throw new Error('Failed to start upgrade')
}
