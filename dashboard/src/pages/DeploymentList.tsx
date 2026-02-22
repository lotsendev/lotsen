import { useState } from 'react'
import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'
import type { Deployment } from '../lib/api'

export default function DeploymentList() {
  useDeploymentSSE()
  const listState = useDeploymentList()
  const [editingDeployment, setEditingDeployment] = useState<Deployment | null>(null)

  return (
    <main className="max-w-4xl mx-auto px-6 py-10">
      <h1 className="text-2xl font-semibold text-gray-900 mb-8">Deployments</h1>
      {editingDeployment
        ? <EditDeploymentForm key={editingDeployment.id} deployment={editingDeployment} onClose={() => setEditingDeployment(null)} />
        : <CreateDeploymentForm />}
      <DeploymentTable {...listState} onEdit={setEditingDeployment} />
    </main>
  )
}
