export type DeploymentStatus = 'idle' | 'deploying' | 'healthy' | 'failed'

export type Deployment = {
  id: string
  name: string
  image: string
  envs: Record<string, string>
  ports: string[]
  volumes: string[]
  domain: string
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

export async function getVersionInfo(): Promise<VersionInfo> {
  const res = await fetch('/api/version')
  if (!res.ok) throw new Error('Failed to fetch version info')
  return res.json()
}

export async function triggerUpgrade(): Promise<void> {
  const res = await fetch('/api/upgrade', { method: 'POST' })
  if (res.status === 409) throw new Error('Upgrade already in progress')
  if (!res.ok) throw new Error('Failed to start upgrade')
}


export type ProxyAccessLogEntry = {
  timestamp: string
  method: string
  path: string
  statusCode: number
  upstreamTarget?: string
  durationMs: number
  clientIp?: string
  host?: string
}

export type ProxySecurityConfig = {
  profile: string
  suspiciousWindowSeconds: number
  suspiciousThreshold: number
  suspiciousBlockForSeconds: number
}

export async function getProxyAccessLogs(limit = 200): Promise<ProxyAccessLogEntry[]> {
  const res = await fetch(`/api/access-logs?limit=${limit}`)
  if (!res.ok) throw new Error('Failed to fetch proxy access logs')
  return res.json()
}

export async function getProxySecurityConfig(): Promise<ProxySecurityConfig> {
  const res = await fetch('/api/security-config')
  if (!res.ok) throw new Error('Failed to fetch proxy security config')
  return res.json()
}
