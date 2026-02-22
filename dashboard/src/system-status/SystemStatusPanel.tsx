import { useSystemStatus } from './useSystemStatus'
import { Badge } from '../components/ui/badge'
import { AlertTriangle, CheckCircle2, CircleSlash2, Clock3 } from 'lucide-react'
import type { HostMetricSystemStatus, SystemStatusState } from '../lib/api'

type BadgeVariant = 'secondary' | 'info' | 'success' | 'destructive' | 'warning'

const STATE_VARIANT: Record<SystemStatusState, BadgeVariant> = {
  healthy: 'success',
  degraded: 'warning',
  stale: 'destructive',
  unavailable: 'secondary',
}

const DEGRADED_PRESSURE_THRESHOLD = 80

const CARD_TONE: Record<SystemStatusState, string> = {
  healthy: 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-900/60 dark:bg-emerald-950/30',
  degraded: 'border-amber-200 bg-amber-50/60 dark:border-amber-900/60 dark:bg-amber-950/30',
  stale: 'border-rose-200 bg-rose-50/60 dark:border-rose-900/60 dark:bg-rose-950/30',
  unavailable: 'border-slate-200 bg-slate-50/70 dark:border-slate-800 dark:bg-slate-900/40',
}

const ICON_TONE: Record<SystemStatusState, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  stale: 'text-rose-600 dark:text-rose-400',
  unavailable: 'text-slate-500 dark:text-slate-400',
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

function formatUsagePercent(metric: HostMetricSystemStatus) {
  if (typeof metric.usagePercent !== 'number') {
    return 'Unavailable'
  }

  return `${metric.usagePercent.toFixed(1)}%`
}

function pressureState(metric: HostMetricSystemStatus): SystemStatusState {
  if (metric.state === 'unavailable' || typeof metric.usagePercent !== 'number') {
    return 'unavailable'
  }

  if (metric.state === 'stale') {
    return 'stale'
  }

  if (metric.state === 'degraded' || metric.usagePercent >= DEGRADED_PRESSURE_THRESHOLD) {
    return 'degraded'
  }

  return 'healthy'
}

function pressureLabel(metric: HostMetricSystemStatus) {
  const state = pressureState(metric)
  if (state === 'healthy') return 'healthy pressure'
  if (state === 'degraded') return 'degraded pressure'
  if (state === 'stale') return 'stale telemetry'
  return 'unavailable telemetry'
}

function formatUsageValue(metric: HostMetricSystemStatus) {
  if (typeof metric.usagePercent !== 'number') {
    return '--'
  }

  return `${Math.round(metric.usagePercent)}%`
}

function statusIcon(state: SystemStatusState) {
  if (state === 'healthy') return CheckCircle2
  if (state === 'degraded') return AlertTriangle
  if (state === 'stale') return Clock3
  return CircleSlash2
}

export function SystemStatusPanel() {
  const { status, isLoading, isError } = useSystemStatus()

  return (
    <section className="space-y-6">
      {isLoading && <p className="text-sm text-muted-foreground">Loading system status…</p>}

      {isError && (
        <p className="text-sm text-destructive">Unable to fetch system status right now.</p>
      )}

      {status && !isLoading && !isError && (
        <div className="space-y-8">
          <section className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Services</p>
            <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
              <article className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[status.api.state]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">API signal</p>
                    <p className="mt-1 text-xs">Control plane availability.</p>
                  </div>
                  {(() => {
                    const Icon = statusIcon(status.api.state)
                    return (
                      <div className="rounded-md bg-background/70 p-2">
                        <Icon data-testid="api-status-icon" className={`h-8 w-8 ${ICON_TONE[status.api.state]}`} />
                      </div>
                    )
                  })()}
                </div>
                <div className="mt-4 flex items-center justify-between gap-3">
                  <p className="text-xs uppercase tracking-wide">State</p>
                  <Badge variant={STATE_VARIANT[status.api.state]}>{status.api.state}</Badge>
                </div>
                <p className="mt-3 border-t border-border/50 pt-3 text-xs">Last updated: {formatTimestamp(status.api.lastUpdated)}</p>
              </article>

              <article className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[status.orchestrator.state]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">Orchestrator</p>
                    <p className="mt-1 text-xs">Agent liveness signal.</p>
                  </div>
                  {(() => {
                    const Icon = statusIcon(status.orchestrator.state)
                    return (
                      <div className="rounded-md bg-background/70 p-2">
                        <Icon
                          data-testid="orchestrator-status-icon"
                          className={`h-8 w-8 ${ICON_TONE[status.orchestrator.state]}`}
                        />
                      </div>
                    )
                  })()}
                </div>
                <div className="mt-4 flex items-center justify-between gap-3">
                  <p className="text-xs uppercase tracking-wide">State</p>
                  <Badge variant={STATE_VARIANT[status.orchestrator.state]}>{status.orchestrator.state}</Badge>
                </div>
                <div className="mt-3 space-y-1 border-t border-border/50 pt-3 text-xs">
                  <p>Last heartbeat: {formatTimestamp(status.orchestrator.lastUpdated)}</p>
                  <p>Freshness: {formatFreshness(status.orchestrator.lastUpdated)}</p>
                </div>
              </article>

              <article className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[status.docker.state]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">Docker connectivity</p>
                    <p className="mt-1 text-xs">Container runtime reachability.</p>
                  </div>
                  {(() => {
                    const Icon = statusIcon(status.docker.state)
                    return (
                      <div className="rounded-md bg-background/70 p-2">
                        <Icon data-testid="docker-status-icon" className={`h-8 w-8 ${ICON_TONE[status.docker.state]}`} />
                      </div>
                    )
                  })()}
                </div>
                <div className="mt-4 flex items-center justify-between gap-3">
                  <p className="text-xs uppercase tracking-wide">State</p>
                  <Badge variant={STATE_VARIANT[status.docker.state]}>{status.docker.state}</Badge>
                </div>
                <p className="mt-3 border-t border-border/50 pt-3 text-xs">Last checked: {formatTimestamp(status.docker.lastUpdated)}</p>
                <p className="mt-1 text-xs">
                  {status.docker.state === 'healthy' && 'Docker is reachable from orchestrator'}
                  {status.docker.state === 'degraded' && 'Docker check failed at last probe'}
                  {status.docker.state === 'stale' && 'Docker signal is stale'}
                  {status.docker.state === 'unavailable' && 'No Docker connectivity telemetry yet'}
                </p>
              </article>
            </div>
          </section>

          <section className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Host metrics</p>
            <div className="grid gap-5 sm:grid-cols-2">
              <article
                className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[pressureState(status.host.cpu)]}`}
              >
                <p className="font-semibold text-foreground">CPU usage</p>
                <div className="mt-3 flex items-end justify-between">
                  <p className="text-3xl font-bold text-foreground">{formatUsageValue(status.host.cpu)}</p>
                  <p className="text-xs uppercase tracking-wide">host load</p>
                </div>
                <p className="mt-3">
                  Pressure{' '}
                  <Badge variant={STATE_VARIANT[pressureState(status.host.cpu)]}>{pressureLabel(status.host.cpu)}</Badge>
                </p>
                <p className="mt-2 text-xs">Reading: {formatUsagePercent(status.host.cpu)}</p>
                <p className="mt-1 text-xs">Last updated: {formatTimestamp(status.host.cpu.lastUpdated)}</p>
              </article>

              <article
                className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[pressureState(status.host.ram)]}`}
              >
                <p className="font-semibold text-foreground">RAM usage</p>
                <div className="mt-3 flex items-end justify-between">
                  <p className="text-3xl font-bold text-foreground">{formatUsageValue(status.host.ram)}</p>
                  <p className="text-xs uppercase tracking-wide">memory load</p>
                </div>
                <p className="mt-3">
                  Pressure{' '}
                  <Badge variant={STATE_VARIANT[pressureState(status.host.ram)]}>{pressureLabel(status.host.ram)}</Badge>
                </p>
                <p className="mt-2 text-xs">Reading: {formatUsagePercent(status.host.ram)}</p>
                <p className="mt-1 text-xs">Last updated: {formatTimestamp(status.host.ram.lastUpdated)}</p>
              </article>
            </div>
          </section>
        </div>
      )}
    </section>
  )
}
