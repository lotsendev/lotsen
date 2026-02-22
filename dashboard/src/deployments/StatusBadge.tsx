import type { DeploymentStatus } from '../lib/api'
import { Badge } from '../components/ui/badge'

const STATUS_VARIANTS: Record<DeploymentStatus, 'secondary' | 'info' | 'success' | 'destructive'> = {
  idle: 'secondary',
  deploying: 'info',
  healthy: 'success',
  failed: 'destructive',
}

type Props = {
  status: DeploymentStatus
  error?: string
}

export function StatusBadge({ status, error }: Props) {
  return (
    <>
      <Badge variant={STATUS_VARIANTS[status]}>
        {status}
      </Badge>
      {status === 'failed' && error && (
        <p className="mt-1 text-xs text-destructive">{error}</p>
      )}
    </>
  )
}
