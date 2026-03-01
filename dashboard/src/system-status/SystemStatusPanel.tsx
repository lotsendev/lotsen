import { AlertTriangle, Check, CheckCircle2, CircleHelp, CircleSlash2, X } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import type { HostMetricSystemStatus, SystemStatusState } from '../lib/api'
import { useSystemStatus } from './useSystemStatus'

type BadgeVariant = 'secondary' | 'info' | 'success' | 'destructive' | 'warning'
type CheckValue = boolean | undefined
const SERVICE_CHECK_ROWS = 3

const STATE_VARIANT: Record<SystemStatusState, BadgeVariant> = {
  healthy: 'success',
  degraded: 'warning',
  unavailable: 'secondary',
}

const STATE_TONE: Record<SystemStatusState, string> = {
  healthy: 'text-emerald-700 dark:text-emerald-300',
  degraded: 'text-amber-700 dark:text-amber-300',
  unavailable: 'text-muted-foreground',
}

const ICON_TONE: Record<SystemStatusState, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  unavailable: 'text-muted-foreground',
}

const DEGRADED_PRESSURE_THRESHOLD = 80

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

function formatCount(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '--'
  }
  return value.toLocaleString()
}

function formatBlockedUntil(timestamp?: string) {
  if (!timestamp) {
    return 'Unknown unblock time'
  }

  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown unblock time'
  }

  if (date.getTime() <= Date.now()) {
    return 'Unblock due now'
  }

  return `Blocked until ${date.toLocaleTimeString()}`
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
  return 'text-muted-foreground'
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
  const placeholders = Math.max(0, SERVICE_CHECK_ROWS - checks.length)

  return (
    <div className="mt-3 rounded-md border border-border/60 bg-background p-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Checks</p>
      <ul className="mt-2 space-y-1.5 text-xs">
        {checks.map((check, index) => (
          <li key={check.label} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
            <span className="truncate text-foreground" title={check.label}>
              {check.label}
            </span>
            <Badge variant={checkVariant(check.value)} className="min-w-13 justify-center">
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
        {Array.from({ length: placeholders }).map((_, index) => (
          <li key={`placeholder-${serviceId}-${index}`} aria-hidden className="h-[22px] rounded bg-background/40" />
        ))}
      </ul>
    </div>
  )
}

export function SystemStatusPanel() {
  const { status, isLoading, isError } = useSystemStatus()

  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading system status…</p>
  }

  if (isError || !status) {
    return <p className="text-sm text-destructive">Unable to fetch system status right now.</p>
  }

  const serviceStates = [status.api.state, status.orchestrator.state, status.docker.state, status.loadBalancer.state]
  const serviceSummary = {
    total: serviceStates.length,
    healthy: serviceStates.filter(state => state === 'healthy').length,
    degraded: serviceStates.filter(state => state === 'degraded').length,
    unavailable: serviceStates.filter(state => state === 'unavailable').length,
  }

  const statusCards = [
    {
      id: 'api',
      title: 'API signal',
      description: 'Control plane availability.',
      state: status.api.state,
      lastUpdatedLabel: `Last updated: ${formatTimestamp(status.api.lastUpdated)}`,
      testId: 'api-status-icon',
      checks: [
        { label: 'Process running', value: status.api.checks?.processRunning },
        { label: 'Dashboard reachability', value: status.api.checks?.dashboardReachable },
        { label: 'Store access', value: status.api.checks?.storeAccessible },
      ],
      notes: [] as string[],
    },
    {
      id: 'orchestrator',
      title: 'Orchestrator',
      description: 'Agent liveness signal.',
      state: status.orchestrator.state,
      lastUpdatedLabel: `Last heartbeat: ${formatTimestamp(status.orchestrator.lastUpdated)}`,
      testId: 'orchestrator-status-icon',
      checks: [
        { label: 'Process running', value: status.orchestrator.checks?.processRunning },
        { label: 'Docker daemon reachability', value: status.orchestrator.checks?.dockerReachable },
        { label: 'Store access', value: status.orchestrator.checks?.storeAccessible },
      ],
      notes: [`Freshness: ${formatFreshness(status.orchestrator.lastUpdated)}`, 'Orchestrator heartbeat stream is active'],
    },
    {
      id: 'docker',
      title: 'Docker connectivity',
      description: 'Container runtime reachability.',
      state: status.docker.state,
      lastUpdatedLabel: `Last checked: ${formatTimestamp(status.docker.lastUpdated)}`,
      testId: 'docker-status-icon',
      checks: [{ label: 'Daemon healthy', value: status.docker.checks?.daemonHealthy }],
      notes: [
        `Signal age: ${formatFreshness(status.docker.lastUpdated)}`,
        status.docker.state === 'healthy'
          ? 'Docker is reachable from orchestrator'
          : status.docker.state === 'degraded'
            ? 'Docker check failed at last probe'
            : 'No Docker connectivity telemetry yet',
      ],
    },
    {
      id: 'load-balancer',
      title: 'Load balancer',
      description: 'Reverse proxy health signal.',
      state: status.loadBalancer.state,
      lastUpdatedLabel: `Last checked: ${formatTimestamp(status.loadBalancer.lastUpdated)}`,
      testId: 'load-balancer-status-icon',
      checks: [
        { label: 'Process running', value: status.loadBalancer.checks?.processRunning },
        { label: 'Healthcheck response', value: status.loadBalancer.checks?.healthcheckResponding },
      ],
      notes: [
        `Signal age: ${formatFreshness(status.loadBalancer.lastUpdated)}`,
        status.loadBalancer.state === 'healthy'
          ? 'Load balancer healthcheck is responding'
          : status.loadBalancer.state === 'degraded'
            ? 'Load balancer healthcheck failed at last probe'
            : 'No load balancer telemetry yet',
      ],
    },
  ]

  statusCards[0].notes = [`Signal age: ${formatFreshness(status.api.lastUpdated)}`, 'API control plane endpoint is responding']

  return (
    <section className="space-y-5">
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Signal grid</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">
          Control plane and runtime health
        </h2>
        <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <article className="rounded-lg border border-border/60 bg-background/70 p-3">
            <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Services tracked</p>
            <p className="mt-1 text-lg font-semibold text-foreground">{serviceSummary.total}</p>
          </article>
          <article className="rounded-lg border border-border/60 bg-background/70 p-3">
            <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Healthy</p>
            <p className="mt-1 text-lg font-semibold text-foreground">{serviceSummary.healthy}</p>
          </article>
          <article className="rounded-lg border border-border/60 bg-background/70 p-3">
            <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Degraded</p>
            <p className="mt-1 text-lg font-semibold text-foreground">{serviceSummary.degraded}</p>
          </article>
          <article className="rounded-lg border border-border/60 bg-background/70 p-3">
            <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Unavailable</p>
            <p className="mt-1 text-lg font-semibold text-foreground">{serviceSummary.unavailable}</p>
          </article>
        </div>
      </section>

      <section className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Services</p>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {statusCards.map(card => {
            const Icon = statusIcon(card.state)
            return (
              <article key={card.id} className="flex h-full flex-col rounded-lg border border-border/60 bg-card p-4 text-sm">
                <div className="flex min-h-20 items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">{card.title}</p>
                    <p className="mt-1 text-xs text-muted-foreground">{card.description}</p>
                  </div>
                  <div className="rounded-md border border-border/60 bg-background/70 p-2">
                    <Icon data-testid={card.testId} className={`h-5 w-5 ${ICON_TONE[card.state]}`} />
                  </div>
                </div>

                <div className="mt-3 flex items-center justify-between gap-3 rounded-md border border-border/60 bg-background/70 px-3 py-2">
                  <p className="text-[11px] uppercase tracking-[0.13em] text-muted-foreground">State</p>
                  <Badge variant={STATE_VARIANT[card.state]} className={`min-w-24 justify-center ${STATE_TONE[card.state]}`}>
                    {card.state}
                  </Badge>
                </div>

                <ServiceChecks serviceId={card.id} checks={card.checks} />

                {card.id === 'load-balancer' && status.loadBalancer.traffic && (
                  <div className="mt-3 rounded-md border border-border/60 bg-background/70 p-3">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Traffic and security</p>
                    <div className="mt-2 grid grid-cols-2 gap-2 text-xs text-foreground">
                      <p>Total requests: {formatCount(status.loadBalancer.traffic.totalRequests)}</p>
                      <p>Suspicious: {formatCount(status.loadBalancer.traffic.suspiciousRequests)}</p>
                      <p>Blocked requests: {formatCount(status.loadBalancer.traffic.blockedRequests)}</p>
                      <p>Active blocked IPs: {formatCount(status.loadBalancer.traffic.activeBlockedIps)}</p>
                      <p>WAF blocked: {formatCount(status.loadBalancer.traffic.wafBlockedRequests)}</p>
                      <p>UA blocked: {formatCount(status.loadBalancer.traffic.uaBlockedRequests)}</p>
                    </div>
                    {status.loadBalancer.traffic.blockedIps && status.loadBalancer.traffic.blockedIps.length > 0 ? (
                      <ul className="mt-2 max-h-32 space-y-1.5 overflow-auto text-xs">
                        {status.loadBalancer.traffic.blockedIps.map(ip => (
                          <li
                            key={ip.ip}
                            className="flex items-center justify-between gap-2 rounded border border-border/50 bg-background px-2.5 py-1.5"
                          >
                            <span className="font-mono text-[11px] text-foreground">{ip.ip}</span>
                            <span className="text-muted-foreground">{formatBlockedUntil(ip.blockedUntil)}</span>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-2 text-xs text-muted-foreground">No blocked IPs currently.</p>
                    )}
                  </div>
                )}

                <div className="mt-auto border-t border-border/50 pt-3 text-xs text-muted-foreground">
                  <p>{card.lastUpdatedLabel}</p>
                  <div className="mt-1 space-y-1">
                    {card.notes.map(note => (
                      <p key={note}>{note}</p>
                    ))}
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      </section>

      <section className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Host metrics</p>
        <div className="grid gap-3 sm:grid-cols-2">
          <article className="rounded-lg border border-border/60 bg-card p-4 text-sm">
            <p className="font-semibold text-foreground">CPU usage</p>
            <div className="mt-3 flex items-end justify-between rounded-md border border-border/60 bg-background/70 px-3 py-2">
              <p className="font-mono text-2xl font-semibold text-foreground">{formatUsageValue(status.host.cpu)}</p>
              <p className="text-[11px] uppercase tracking-[0.13em] text-muted-foreground">host load</p>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <span className="text-xs text-muted-foreground">Pressure</span>
              <Badge variant={STATE_VARIANT[pressureState(status.host.cpu)]}>{pressureLabel(status.host.cpu)}</Badge>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">Reading: {formatUsagePercent(status.host.cpu)}</p>
            <p className="mt-1 text-xs text-muted-foreground">Last updated: {formatTimestamp(status.host.cpu.lastUpdated)}</p>
          </article>

          <article className="rounded-lg border border-border/60 bg-card p-4 text-sm">
            <p className="font-semibold text-foreground">RAM usage</p>
            <div className="mt-3 flex items-end justify-between rounded-md border border-border/60 bg-background/70 px-3 py-2">
              <p className="font-mono text-2xl font-semibold text-foreground">{formatUsageValue(status.host.ram)}</p>
              <p className="text-[11px] uppercase tracking-[0.13em] text-muted-foreground">memory load</p>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <span className="text-xs text-muted-foreground">Pressure</span>
              <Badge variant={STATE_VARIANT[pressureState(status.host.ram)]}>{pressureLabel(status.host.ram)}</Badge>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">Reading: {formatUsagePercent(status.host.ram)}</p>
            <p className="mt-1 text-xs text-muted-foreground">Last updated: {formatTimestamp(status.host.ram.lastUpdated)}</p>
          </article>
        </div>
      </section>
    </section>
  )
}
