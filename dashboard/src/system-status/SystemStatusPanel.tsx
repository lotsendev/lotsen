import { useSystemStatus } from './useSystemStatus'
import { Badge } from '../components/ui/badge'
import { AlertTriangle, Check, CheckCircle2, CircleHelp, CircleSlash2, X } from 'lucide-react'
import type { HostMetricSystemStatus, SystemStatusState } from '../lib/api'

type BadgeVariant = 'secondary' | 'info' | 'success' | 'destructive' | 'warning'
type CheckValue = boolean | undefined

const STATE_VARIANT: Record<SystemStatusState, BadgeVariant> = {
  healthy: 'success',
  degraded: 'warning',
  unavailable: 'secondary',
}

const DEGRADED_PRESSURE_THRESHOLD = 80

const CARD_TONE: Record<SystemStatusState, string> = {
  healthy: 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-900/60 dark:bg-emerald-950/30',
  degraded: 'border-amber-200 bg-amber-50/60 dark:border-amber-900/60 dark:bg-amber-950/30',
  unavailable: 'border-slate-200 bg-slate-50/70 dark:border-slate-800 dark:bg-slate-900/40',
}

const ICON_TONE: Record<SystemStatusState, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  degraded: 'text-amber-600 dark:text-amber-400',
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

  if (metric.state === 'degraded' || metric.usagePercent >= DEGRADED_PRESSURE_THRESHOLD) {
    return 'degraded'
  }

  return 'healthy'
}

function pressureLabel(metric: HostMetricSystemStatus) {
  const state = pressureState(metric)
  if (state === 'healthy') return 'healthy pressure'
  if (state === 'degraded') return 'degraded pressure'
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
  return CircleSlash2
}

function checkVariant(value: CheckValue): BadgeVariant {
  if (value === true) return 'success'
  if (value === false) return 'destructive'
  return 'secondary'
}

function checkIcon(value: CheckValue) {
  if (value === true) return Check
  if (value === false) return X
  return CircleHelp
}

function checkIconTone(value: CheckValue) {
  if (value === true) return 'text-emerald-600 dark:text-emerald-400'
  if (value === false) return 'text-rose-600 dark:text-rose-400'
  return 'text-slate-500 dark:text-slate-400'
}

function checkLabel(value: CheckValue) {
  if (value === true) return 'pass'
  if (value === false) return 'fail'
  return 'unknown'
}

function ServiceChecks({
  serviceId,
  checks,
}: {
  serviceId: string
  checks: Array<{ label: string; value: CheckValue }>
}) {
  return (
    <div className="mt-3 border-t border-border/50 pt-3">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Checks</p>
      <ul className="mt-2 space-y-1.5 text-xs">
        {checks.map((check, index) => (
          <li key={check.label} className="flex items-center justify-between gap-3">
            <span>{check.label}</span>
            <Badge variant={checkVariant(check.value)}>
              {(() => {
                const Icon = checkIcon(check.value)
                return (
                  <Icon
                    data-testid={`${serviceId}-check-${index}-${checkLabel(check.value)}`}
                    aria-label={checkLabel(check.value)}
                    className={`h-3.5 w-3.5 ${checkIconTone(check.value)}`}
                  />
                )
              })()}
            </Badge>
          </li>
        ))}
      </ul>
    </div>
  )
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
            <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
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
                <ServiceChecks
                  serviceId="api"
                  checks={[
                    { label: 'Process running', value: status.api.checks?.processRunning },
                    { label: 'Dashboard reachability', value: status.api.checks?.dashboardReachable },
                    { label: 'Store access', value: status.api.checks?.storeAccessible },
                  ]}
                />
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
                <ServiceChecks
                  serviceId="orchestrator"
                  checks={[
                    { label: 'Process running', value: status.orchestrator.checks?.processRunning },
                    { label: 'Docker daemon reachability', value: status.orchestrator.checks?.dockerReachable },
                    { label: 'Store access', value: status.orchestrator.checks?.storeAccessible },
                  ]}
                />
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
                <ServiceChecks
                  serviceId="docker"
                  checks={[{ label: 'Daemon healthy', value: status.docker.checks?.daemonHealthy }]}
                />
                <p className="mt-3 border-t border-border/50 pt-3 text-xs">Last checked: {formatTimestamp(status.docker.lastUpdated)}</p>
                <p className="mt-1 text-xs">
                  {status.docker.state === 'healthy' && 'Docker is reachable from orchestrator'}
                  {status.docker.state === 'degraded' && 'Docker check failed at last probe'}
                  {status.docker.state === 'unavailable' && 'No Docker connectivity telemetry yet'}
                </p>
              </article>

              <article className={`rounded-lg border p-5 text-sm text-muted-foreground ${CARD_TONE[status.loadBalancer.state]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">Load balancer</p>
                    <p className="mt-1 text-xs">Reverse proxy health signal.</p>
                  </div>
                  {(() => {
                    const Icon = statusIcon(status.loadBalancer.state)
                    return (
                      <div className="rounded-md bg-background/70 p-2">
                        <Icon
                          data-testid="load-balancer-status-icon"
                          className={`h-8 w-8 ${ICON_TONE[status.loadBalancer.state]}`}
                        />
                      </div>
                    )
                  })()}
                </div>
                <div className="mt-4 flex items-center justify-between gap-3">
                  <p className="text-xs uppercase tracking-wide">State</p>
                  <Badge variant={STATE_VARIANT[status.loadBalancer.state]}>{status.loadBalancer.state}</Badge>
                </div>
                <ServiceChecks
                  serviceId="load-balancer"
                  checks={[
                    { label: 'Process running', value: status.loadBalancer.checks?.processRunning },
                    { label: 'Healthcheck response', value: status.loadBalancer.checks?.healthcheckResponding },
                  ]}
                />
                <p className="mt-3 border-t border-border/50 pt-3 text-xs">
                  Last checked: {formatTimestamp(status.loadBalancer.lastUpdated)}
                </p>
                <p className="mt-1 text-xs">
                  {status.loadBalancer.state === 'healthy' && 'Load balancer healthcheck is responding'}
                  {status.loadBalancer.state === 'degraded' && 'Load balancer healthcheck failed at last probe'}
                  {status.loadBalancer.state === 'unavailable' && 'No load balancer telemetry yet'}
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
