export type DeploymentStatus = 'idle' | 'deploying' | 'healthy' | 'failed'

export type BasicAuthUser = {
  username: string
  password: string
}

export type BasicAuthConfig = {
  users: BasicAuthUser[]
}

export type Deployment = {
  id: string
  name: string
  image: string
  envs: Record<string, string>
  ports: string[]
  volumes: string[]
  domain: string
  basic_auth?: BasicAuthConfig
  status: DeploymentStatus
  error?: string
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
    activeBlockedIps: number
    blockedIps?: Array<{
      ip: string
      blockedUntil?: string
    }>
  }
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

export async function getDeployments(): Promise<Deployment[]> {
  const res = await fetch('/api/deployments')
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
  const res = await fetch('/api/deployments', {
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
}

export async function updateDeployment(id: string, data: UpdateDeploymentInput): Promise<Deployment> {
  const res = await fetch(`/api/deployments/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to update deployment')
  return res.json()
}

export async function deleteDeployment(id: string): Promise<void> {
  const res = await fetch(`/api/deployments/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to delete deployment')
}

export async function getSystemStatus(): Promise<SystemStatusSnapshot> {
  const res = await fetch('/api/system-status')
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

  const res = await fetch(`/api/load-balancer/access-logs?${params.toString()}`)
  if (!res.ok) throw new Error('Failed to fetch load balancer access logs')
  return res.json()
}

export async function getVersionInfo(): Promise<VersionInfo> {
  const res = await fetch('/api/version')
  if (!res.ok) throw new Error('Failed to fetch version info')
  return res.json()
}

export async function getVersionReleases(limit = 25): Promise<VersionRelease[]> {
  const res = await fetch(`/api/version/releases?limit=${limit}`)
  if (!res.ok) throw new Error('Failed to fetch version releases')
  return res.json()
}

export async function triggerUpgrade(targetVersion?: string): Promise<void> {
  const body = targetVersion ? JSON.stringify({ targetVersion }) : undefined
  const res = await fetch('/api/upgrade', {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body,
  })
  if (res.status === 409) throw new Error('Upgrade already in progress')
  if (!res.ok) throw new Error('Failed to start upgrade')
}
