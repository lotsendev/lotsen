import type { UseMutationResult } from '@tanstack/react-query'
import type { Deployment } from '../lib/api'
import { DeploymentRow } from './DeploymentRow'

type Props = {
  deployments: Deployment[] | undefined
  isLoading: boolean
  isError: boolean
  deleteMutation: UseMutationResult<void, Error, string>
}

export function DeploymentTable({ deployments, isLoading, isError, deleteMutation }: Props) {
  if (isLoading) return <p className="text-sm text-gray-500">Loading deployments…</p>
  if (isError) return <p className="text-sm text-red-600">Failed to load deployments.</p>
  if (!deployments?.length) return <p className="text-sm text-gray-500">No deployments yet.</p>

  return (
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
            <DeploymentRow
              key={d.id}
              deployment={d}
              onDelete={id => deleteMutation.mutate(id)}
              isDeleting={deleteMutation.isPending}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}
