import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'

export default function DeploymentList() {
  useDeploymentSSE()
  const listState = useDeploymentList()

  return (
    <main className="max-w-4xl mx-auto px-6 py-10">
      <h1 className="text-2xl font-semibold text-gray-900 mb-8">Deployments</h1>
      <CreateDeploymentForm />
      <DeploymentTable {...listState} />
    </main>
  )
}
