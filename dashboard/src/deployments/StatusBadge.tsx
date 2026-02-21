import type { DeploymentStatus } from '../lib/api'

const STATUS_STYLES: Record<DeploymentStatus, string> = {
  idle: 'bg-gray-100 text-gray-600',
  deploying: 'bg-blue-100 text-blue-700',
  healthy: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
}

type Props = {
  status: DeploymentStatus
  error?: string
}

export function StatusBadge({ status, error }: Props) {
  return (
    <>
      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLES[status]}`}>
        {status}
      </span>
      {status === 'failed' && error && (
        <p className="mt-1 text-xs text-red-600">{error}</p>
      )}
    </>
  )
}
