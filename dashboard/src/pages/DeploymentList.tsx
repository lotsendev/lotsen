import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { getDeployments, deleteDeployment, type DeploymentStatus } from '../lib/api'
import CreateDeploymentForm from './CreateDeploymentForm'

const STATUS_STYLES: Record<DeploymentStatus, string> = {
  idle: 'bg-gray-100 text-gray-600',
  deploying: 'bg-blue-100 text-blue-700',
  healthy: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
}

export default function DeploymentList() {
  const queryClient = useQueryClient()

  const { data: deployments, isLoading, isError } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDeployment,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['deployments'] }),
  })

  return (
    <main className="max-w-4xl mx-auto px-6 py-10">
      <h1 className="text-2xl font-semibold text-gray-900 mb-8">Deployments</h1>

      <CreateDeploymentForm />

      {/* Deployment table */}
      {isLoading && <p className="text-sm text-gray-500">Loading deployments…</p>}
      {isError && <p className="text-sm text-red-600">Failed to load deployments.</p>}
      {deployments && deployments.length === 0 && (
        <p className="text-sm text-gray-500">No deployments yet.</p>
      )}
      {deployments && deployments.length > 0 && (
        <div className="border border-gray-200 rounded-lg overflow-hidden shadow-sm">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Image</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {deployments.map(d => (
                <tr key={d.id} className="bg-white hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-gray-900">{d.name}</td>
                  <td className="px-4 py-3 text-gray-600 font-mono text-xs">{d.image}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLES[d.status]}`}>
                      {d.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => deleteMutation.mutate(d.id)}
                      disabled={deleteMutation.isPending}
                      aria-label={`Delete ${d.name}`}
                      className="p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 disabled:opacity-40"
                    >
                      <Trash2 size={15} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
