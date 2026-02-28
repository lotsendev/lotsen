import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { getSecurityConfig } from '../lib/api'
import { useLoadBalancerAccessLogs } from '../system-status/useLoadBalancerAccessLogs'
import { useSystemStatus } from '../system-status/useSystemStatus'

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

function statusVariant(status?: number): 'secondary' | 'success' | 'warning' | 'destructive' {
  if (typeof status !== 'number') return 'secondary'
  if (status < 300) return 'success'
  if (status < 500) return 'warning'
  return 'destructive'
}

export function TrafficPage() {
  const { status, isLoading, isError } = useSystemStatus()
  const securityConfig = useQuery({
    queryKey: ['security-config'],
    queryFn: getSecurityConfig,
  })
  const [methodFilter, setMethodFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [hostFilter, setHostFilter] = useState('')
  const [ipFilter, setIPFilter] = useState('')
  const [appliedFilters, setAppliedFilters] = useState<{ method?: string; status?: number; host?: string; ip?: string }>({})
  const accessLogs = useLoadBalancerAccessLogs(appliedFilters)

  const applyFilters = () => {
    const next: { method?: string; status?: number; host?: string; ip?: string } = {}
    const method = methodFilter.trim().toUpperCase()
    const httpStatus = Number(statusFilter)
    const host = hostFilter.trim()
    const ip = ipFilter.trim()

    if (method) {
      next.method = method
    }
    if (statusFilter.trim() && Number.isFinite(httpStatus) && httpStatus > 0) {
      next.status = httpStatus
    }
    if (host) {
      next.host = host
    }
    if (ip) {
      next.ip = ip
    }

    setAppliedFilters(next)
  }

  const clearFilters = () => {
    setMethodFilter('')
    setStatusFilter('')
    setHostFilter('')
    setIPFilter('')
    setAppliedFilters({})
  }

  const formatCount = (value?: number) => {
    if (typeof value !== 'number' || Number.isNaN(value)) {
      return '--'
    }
    return value.toLocaleString()
  }

  const formatTimestamp = (timestamp?: string) => {
    if (!timestamp) {
      return 'Unknown'
    }

    const date = new Date(timestamp)
    if (Number.isNaN(date.getTime())) {
      return 'Unknown'
    }

    return date.toLocaleString()
  }

  const formatBlockedUntil = (timestamp?: string) => {
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

  const wafMode = securityConfig.data?.wafMode ?? 'off'

  const wafModeVariant = () => {
    if (wafMode === 'enforcement') {
      return 'destructive' as const
    }
    if (wafMode === 'detection') {
      return 'warning' as const
    }
    return 'secondary' as const
  }

  return (
    <div className="space-y-5">
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Traffic watch</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">
              Security posture and proxy pressure
            </h2>
          </div>
        </div>

        <div className="mt-4">
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading traffic telemetry...</p>
          ) : isError || !status ? (
            <p className="text-sm text-destructive">Unable to load traffic telemetry.</p>
          ) : !status.loadBalancer.traffic ? (
            <p className="text-sm text-muted-foreground">No traffic metrics reported yet.</p>
          ) : (
            <div className="space-y-4">
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Total requests</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.totalRequests)}
                  </p>
                </article>
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Suspicious</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.suspiciousRequests)}
                  </p>
                </article>
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Blocked requests</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.blockedRequests)}
                  </p>
                </article>
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Active blocked IPs</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.activeBlockedIps)}
                  </p>
                </article>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">WAF blocked</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.wafBlockedRequests)}
                  </p>
                </article>
                <article className="rounded-lg border border-border/60 bg-background/70 p-3">
                  <p className="text-[11px] uppercase tracking-wide text-muted-foreground">UA blocked</p>
                  <p className="mt-1 text-lg font-semibold text-foreground">
                    {formatCount(status.loadBalancer.traffic.uaBlockedRequests)}
                  </p>
                </article>
              </div>
              {status.loadBalancer.traffic.blockedIps && status.loadBalancer.traffic.blockedIps.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-[0.13em] text-muted-foreground">Currently blocked IPs</p>
                  <ul className="max-h-40 space-y-1.5 overflow-auto text-xs">
                    {status.loadBalancer.traffic.blockedIps.map(ip => (
                      <li
                        key={ip.ip}
                        className="flex items-center justify-between gap-2 rounded-md border border-border/60 bg-background/70 px-3 py-2"
                      >
                        <span className="font-mono text-[11px] text-foreground">{ip.ip}</span>
                        <Badge variant="warning">{formatBlockedUntil(ip.blockedUntil)}</Badge>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No blocked IPs currently.</p>
              )}
            </div>
          )}
        </div>
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Global security</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">
              Effective WAF and IP filtering mode
            </h2>
          </div>
          {securityConfig.isLoading ? null : securityConfig.isError ? null : (
            <Badge variant={wafModeVariant()} className="capitalize">
              WAF mode: {wafMode}
            </Badge>
          )}
        </div>

        {securityConfig.isLoading ? (
          <p className="mt-4 text-sm text-muted-foreground">Loading global security config...</p>
        ) : securityConfig.isError || !securityConfig.data ? (
          <p className="mt-4 text-sm text-destructive">Unable to load global security config.</p>
        ) : (
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Global IP denylist</p>
              {securityConfig.data.globalIpDenylist && securityConfig.data.globalIpDenylist.length > 0 ? (
                <ul className="mt-2 space-y-1 text-xs">
                  {securityConfig.data.globalIpDenylist.map(entry => (
                    <li key={`global-deny-${entry}`} className="font-mono text-foreground">
                      {entry}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-xs text-muted-foreground">No global denylist entries configured.</p>
              )}
            </article>
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Global IP allowlist</p>
              {securityConfig.data.globalIpAllowlist && securityConfig.data.globalIpAllowlist.length > 0 ? (
                <ul className="mt-2 space-y-1 text-xs">
                  {securityConfig.data.globalIpAllowlist.map(entry => (
                    <li key={`global-allow-${entry}`} className="font-mono text-foreground">
                      {entry}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-xs text-muted-foreground">No global allowlist entries configured.</p>
              )}
            </article>
          </div>
        )}
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Access ledger</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">
              Proxy access logs
            </h2>
          </div>
          <div className="flex items-center gap-2">
            <Button type="button" size="sm" variant="outline" onClick={clearFilters}>
              Clear
            </Button>
            <Button type="button" size="sm" onClick={applyFilters}>
              Apply filters
            </Button>
          </div>
        </div>

        <div className="mt-4 space-y-3 rounded-lg border border-border/60 bg-background/70 p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-muted-foreground">Method</span>
            <Button
              type="button"
              size="sm"
              variant={methodFilter ? 'outline' : 'default'}
              className="h-7 px-2.5 text-[11px]"
              onClick={() => setMethodFilter('')}
            >
              Any
            </Button>
            {HTTP_METHODS.map(method => (
              <Button
                key={method}
                type="button"
                size="sm"
                variant={methodFilter === method ? 'default' : 'outline'}
                className="h-7 px-2.5 text-[11px]"
                onClick={() => setMethodFilter(method)}
              >
                {method}
              </Button>
            ))}
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <label className="space-y-1">
              <span className="text-xs text-muted-foreground">HTTP status</span>
              <Input
                type="number"
                min={100}
                max={599}
                value={statusFilter}
                onChange={event => setStatusFilter(event.target.value)}
                placeholder="Any"
                className="h-8"
              />
            </label>
            <label className="space-y-1">
              <span className="text-xs text-muted-foreground">Host contains</span>
              <Input
                type="text"
                value={hostFilter}
                onChange={event => setHostFilter(event.target.value)}
                placeholder="api.example.com"
                className="h-8"
              />
            </label>
            <label className="space-y-1">
              <span className="text-xs text-muted-foreground">IP contains</span>
              <Input
                type="text"
                value={ipFilter}
                onChange={event => setIPFilter(event.target.value)}
                placeholder="203.0.113"
                className="h-8"
              />
            </label>
          </div>
        </div>

        {accessLogs.isError ? (
          <p className="mt-4 text-sm text-destructive">Unable to load access logs.</p>
        ) : accessLogs.items.length === 0 && !accessLogs.isLoading ? (
          <p className="mt-4 text-sm text-muted-foreground">No access logs yet.</p>
        ) : (
          <div className="mt-4 overflow-x-auto rounded-lg border border-border/60 bg-background/70">
            <table className="w-full min-w-[760px] text-left text-xs">
                <thead>
                  <tr className="border-b border-border/60 text-muted-foreground">
                    <th className="px-3 py-2 font-medium">Time</th>
                    <th className="px-3 py-2 font-medium">IP</th>
                    <th className="px-3 py-2 font-medium">Request</th>
                    <th className="px-3 py-2 font-medium">Status</th>
                    <th className="px-3 py-2 font-medium">Duration</th>
                    <th className="px-3 py-2 font-medium">Outcome</th>
                  </tr>
                </thead>
                <tbody>
                  {accessLogs.items.map((entry, idx) => (
                    <tr
                      key={`${entry.timestamp}-${entry.clientIp}-${entry.path}-${idx}`}
                      className="border-t border-border/40 align-top"
                    >
                      <td className="px-3 py-2 whitespace-nowrap">{formatTimestamp(entry.timestamp)}</td>
                      <td className="px-3 py-2 font-mono text-[11px]">{entry.clientIp || '-'}</td>
                      <td className="px-3 py-2">
                        <div className="text-foreground">
                          {entry.method} {entry.path}
                          {entry.query ? `?${entry.query}` : ''}
                        </div>
                        {entry.headers && Object.keys(entry.headers).length > 0 && (
                          <details className="mt-1 text-[11px] text-muted-foreground">
                            <summary className="cursor-pointer select-none">Headers</summary>
                            <pre className="mt-1 whitespace-pre-wrap rounded border border-border/50 bg-background p-2 text-[10px] text-foreground">
                              {JSON.stringify(entry.headers, null, 2)}
                            </pre>
                          </details>
                        )}
                      </td>
                      <td className="px-3 py-2">
                        <Badge variant={statusVariant(entry.status)}>{entry.status ?? 'n/a'}</Badge>
                      </td>
                      <td className="px-3 py-2">{entry.durationMs}ms</td>
                      <td className="px-3 py-2">
                        <Badge variant="outline" className="capitalize">
                          {entry.outcome}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
            </table>
          </div>
        )}

        <div className="mt-4 flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={accessLogs.loadOlder}
            disabled={!accessLogs.hasMore || accessLogs.isLoading}
          >
            {accessLogs.isLoading ? 'Loading…' : accessLogs.hasMore ? 'Load older logs' : 'No older logs'}
          </Button>
        </div>
      </section>
    </div>
  )
}
