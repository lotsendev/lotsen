import { useState } from 'react'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { useLoadBalancerAccessLogs } from '../system-status/useLoadBalancerAccessLogs'
import { useSystemStatus } from '../system-status/useSystemStatus'

export function TrafficPage() {
  const { status, isLoading, isError } = useSystemStatus()
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

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Traffic and security posture</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading traffic telemetry...</p>
          ) : isError || !status ? (
            <p className="text-sm text-destructive">Unable to load traffic telemetry.</p>
          ) : !status.loadBalancer.traffic ? (
            <p className="text-sm text-muted-foreground">No traffic metrics reported yet.</p>
          ) : (
            <div className="space-y-3">
              <div className="grid gap-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
                <p>Total requests: {formatCount(status.loadBalancer.traffic.totalRequests)}</p>
                <p>Suspicious: {formatCount(status.loadBalancer.traffic.suspiciousRequests)}</p>
                <p>Blocked requests: {formatCount(status.loadBalancer.traffic.blockedRequests)}</p>
                <p>Active blocked IPs: {formatCount(status.loadBalancer.traffic.activeBlockedIps)}</p>
              </div>
              {status.loadBalancer.traffic.blockedIps && status.loadBalancer.traffic.blockedIps.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Currently blocked IPs</p>
                  <ul className="max-h-36 space-y-1 overflow-auto text-xs">
                    {status.loadBalancer.traffic.blockedIps.map(ip => (
                      <li key={ip.ip} className="flex items-center justify-between gap-2 rounded border border-border/50 px-2 py-1">
                        <span className="font-mono text-[11px] text-foreground">{ip.ip}</span>
                        <Badge variant="secondary">{formatBlockedUntil(ip.blockedUntil)}</Badge>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No blocked IPs currently.</p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Proxy access logs</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            <label className="text-xs text-muted-foreground">
              Method
              <select
                value={methodFilter}
                onChange={event => setMethodFilter(event.target.value)}
                className="mt-1 w-full rounded border border-border bg-background px-2 py-1 text-xs text-foreground"
              >
                <option value="">Any</option>
                {['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].map(method => (
                  <option key={method} value={method}>
                    {method}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-xs text-muted-foreground">
              Status
              <input
                type="number"
                min={100}
                max={599}
                value={statusFilter}
                onChange={event => setStatusFilter(event.target.value)}
                className="mt-1 w-full rounded border border-border bg-background px-2 py-1 text-xs text-foreground"
                placeholder="Any"
              />
            </label>
            <label className="text-xs text-muted-foreground">
              Host contains
              <input
                type="text"
                value={hostFilter}
                onChange={event => setHostFilter(event.target.value)}
                className="mt-1 w-full rounded border border-border bg-background px-2 py-1 text-xs text-foreground"
                placeholder="api.example.com"
              />
            </label>
            <label className="text-xs text-muted-foreground">
              IP contains
              <input
                type="text"
                value={ipFilter}
                onChange={event => setIPFilter(event.target.value)}
                className="mt-1 w-full rounded border border-border bg-background px-2 py-1 text-xs text-foreground"
                placeholder="203.0.113"
              />
            </label>
          </div>

          <div className="mt-2 flex items-center gap-2">
            <button
              type="button"
              onClick={applyFilters}
              className="rounded border border-border px-2 py-1 text-xs text-foreground"
            >
              Apply filters
            </button>
            <button
              type="button"
              onClick={clearFilters}
              className="rounded border border-border px-2 py-1 text-xs text-muted-foreground"
            >
              Clear
            </button>
          </div>

          {accessLogs.isError ? (
            <p className="text-sm text-destructive">Unable to load access logs.</p>
          ) : accessLogs.items.length === 0 && !accessLogs.isLoading ? (
            <p className="text-sm text-muted-foreground">No access logs yet.</p>
          ) : (
            <div className="mt-2 overflow-x-auto">
              <table className="w-full min-w-[640px] text-left text-xs">
                <thead>
                  <tr className="text-muted-foreground">
                    <th className="py-1 pr-2 font-medium">Time</th>
                    <th className="py-1 pr-2 font-medium">IP</th>
                    <th className="py-1 pr-2 font-medium">Request</th>
                    <th className="py-1 pr-2 font-medium">Status</th>
                    <th className="py-1 pr-2 font-medium">Duration</th>
                    <th className="py-1 pr-2 font-medium">Outcome</th>
                  </tr>
                </thead>
                <tbody>
                  {accessLogs.items.map((entry, idx) => (
                    <tr key={`${entry.timestamp}-${entry.clientIp}-${entry.path}-${idx}`} className="border-t border-border/30 align-top">
                      <td className="py-1 pr-2 whitespace-nowrap">{formatTimestamp(entry.timestamp)}</td>
                      <td className="py-1 pr-2 font-mono text-[11px]">{entry.clientIp || '-'}</td>
                      <td className="py-1 pr-2">
                        <div className="text-foreground">
                          {entry.method} {entry.path}
                          {entry.query ? `?${entry.query}` : ''}
                        </div>
                        {entry.headers && Object.keys(entry.headers).length > 0 && (
                          <details className="mt-1 text-[11px] text-muted-foreground">
                            <summary className="cursor-pointer select-none">Headers</summary>
                            <pre className="mt-1 whitespace-pre-wrap rounded border border-border/40 bg-background/70 p-2 text-[10px] text-foreground">
                              {JSON.stringify(entry.headers, null, 2)}
                            </pre>
                          </details>
                        )}
                      </td>
                      <td className="py-1 pr-2">{entry.status}</td>
                      <td className="py-1 pr-2">{entry.durationMs}ms</td>
                      <td className="py-1 pr-2">{entry.outcome}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="mt-2 flex items-center gap-2">
            <button
              type="button"
              onClick={accessLogs.loadOlder}
              disabled={!accessLogs.hasMore || accessLogs.isLoading}
              className="rounded border border-border px-2 py-1 text-xs text-foreground disabled:cursor-not-allowed disabled:opacity-60"
            >
              {accessLogs.isLoading ? 'Loading…' : accessLogs.hasMore ? 'Load older logs' : 'No older logs'}
            </button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
