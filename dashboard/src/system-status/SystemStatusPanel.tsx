import { useSystemStatus } from './useSystemStatus'
import { Badge } from '../components/ui/badge'
import type { SystemStatusState } from '../lib/api'

type BadgeVariant = 'secondary' | 'info' | 'success' | 'destructive' | 'warning'

const STATE_VARIANT: Record<SystemStatusState, BadgeVariant> = {
  healthy: 'success',
  degraded: 'warning',
  stale: 'destructive',
  unavailable: 'secondary',
}

function formatTimestamp(timestamp?: string) {
  if (!timestamp) {
    return 'No signal yet'
  }

  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }
  return date.toLocaleString()
}

function formatFreshness(timestamp?: string) {
  if (!timestamp) {
    return 'No heartbeat observed'
  }

  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }

  const diffSeconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000))
  if (diffSeconds < 60) return 'just now'
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`
  if (diffSeconds < 86400) return `${Math.floor(diffSeconds / 3600)}h ago`
  return `${Math.floor(diffSeconds / 86400)}d ago`
}

export function SystemStatusPanel() {
  const { status, isLoading, isError } = useSystemStatus()

  return (
    <section className="space-y-4">
      {isLoading && <p className="text-sm text-muted-foreground">Loading system status…</p>}

      {isError && (
        <p className="text-sm text-destructive">Unable to fetch system status right now.</p>
      )}

      {status && !isLoading && !isError && (
        <div className="divide-y rounded-lg bg-muted/20 text-sm text-muted-foreground">
          <section className="space-y-2 p-4">
            <p className="font-semibold text-foreground">API signal</p>
            <p className="text-xs text-muted-foreground/90">Control plane availability and response health.</p>
            <p>
              State: <Badge variant={STATE_VARIANT[status.api.state]}>{status.api.state}</Badge>
            </p>
            <p>
              Last updated:{' '}
              <span className="font-medium text-foreground">{formatTimestamp(status.api.lastUpdated)}</span>
            </p>
          </section>

          <section className="space-y-2 p-4">
            <p className="font-semibold text-foreground">Orchestrator liveness</p>
            <p className="text-xs text-muted-foreground/90">
              Worker heartbeat and Docker connectivity statuses from the host agent.
            </p>
            <ul className="space-y-1.5">
              <li>
                State: <Badge variant={STATE_VARIANT[status.orchestrator.state]}>{status.orchestrator.state}</Badge>
              </li>
              <li>
                Last heartbeat:{' '}
                <span className="font-medium text-foreground">
                  {formatTimestamp(status.orchestrator.lastUpdated)}
                </span>
              </li>
              <li>
                Freshness:{' '}
                <span className="font-medium text-foreground">
                  {formatFreshness(status.orchestrator.lastUpdated)}
                </span>
              </li>
              <li className="pt-1">
                Docker state: <Badge variant={STATE_VARIANT[status.docker.state]}>{status.docker.state}</Badge>
              </li>
              <li>
                Last checked:{' '}
                <span className="font-medium text-foreground">{formatTimestamp(status.docker.lastUpdated)}</span>
              </li>
              <li>
                Signal:{' '}
                <span className="font-medium text-foreground">
                  {status.docker.state === 'healthy' && 'Docker is reachable from orchestrator'}
                  {status.docker.state === 'degraded' && 'Docker check failed at last probe'}
                  {status.docker.state === 'stale' && 'Docker signal is stale'}
                  {status.docker.state === 'unavailable' && 'No Docker connectivity telemetry yet'}
                </span>
              </li>
            </ul>
          </section>
        </div>
      )}
    </section>
  )
}
