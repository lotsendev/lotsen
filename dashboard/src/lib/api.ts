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
}

export async function getDeployments(): Promise<Deployment[]> {
  const res = await fetch('/api/deployments')
  if (!res.ok) throw new Error('Failed to fetch deployments')
  return res.json()
}

export async function createDeployment(data: { name: string; image: string }): Promise<Deployment> {
  const res = await fetch('/api/deployments', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to create deployment')
  return res.json()
}

export async function deleteDeployment(id: string): Promise<void> {
  const res = await fetch(`/api/deployments/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to delete deployment')
}
