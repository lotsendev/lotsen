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

export type SystemStatusState = 'healthy' | 'unavailable'

export type APISystemStatus = {
  state: SystemStatusState
  lastUpdated: string
}

export type SystemStatusSnapshot = {
  api: APISystemStatus
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
