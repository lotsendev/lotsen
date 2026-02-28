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
}

export function StatusBadge({ status }: Props) {
  const label = status === 'idle' ? 'Idle' : status === 'deploying' ? 'Deploying' : status === 'healthy' ? 'Healthy' : 'Failed'

  return (
    <Badge variant={STATUS_VARIANTS[status]} className="min-w-20 justify-center font-semibold tracking-normal">
      {label}
    </Badge>
  )
}
