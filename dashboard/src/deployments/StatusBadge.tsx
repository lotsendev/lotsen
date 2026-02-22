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
  return (
    <Badge variant={STATUS_VARIANTS[status]} className="uppercase tracking-wide">
      {status}
    </Badge>
  )
}
