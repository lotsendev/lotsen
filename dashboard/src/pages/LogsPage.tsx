import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '../components/ui/button'
import { useDeploymentLogsSSE } from '../deployments/useDeploymentLogsSSE'
import { type CoreService, getCoreServiceLogs, getDeploymentRecentLogs, getDeployments } from '../lib/api'

const CORE_SERVICES: Array<{ value: CoreService; label: string }> = [
  { value: 'api', label: 'API service' },
  { value: 'orchestrator', label: 'Orchestrator service' },
  { value: 'proxy', label: 'Proxy service' },
]

const TAIL_OPTIONS = [100, 200, 400, 800]
const DEPLOYMENT_RECENT_TAIL_OPTIONS = [100, 300, 500, 800]

export function LogsPage() {
  const [coreService, setCoreService] = useState<CoreService>('api')
  const [tail, setTail] = useState(200)
  const [autoRefreshCore, setAutoRefreshCore] = useState(true)
  const [selectedDeploymentID, setSelectedDeploymentID] = useState('')
  const [deploymentLogMode, setDeploymentLogMode] = useState<'live' | 'recent'>('live')
  const [deploymentRecentTail, setDeploymentRecentTail] = useState(300)
  const [reconnectToken, setReconnectToken] = useState(0)

  const coreLogs = useQuery({
    queryKey: ['core-service-logs', coreService, tail],
    queryFn: () => getCoreServiceLogs(coreService, tail),
    refetchInterval: autoRefreshCore ? 5000 : false,
    retry: false,
  })

  const deployments = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
    refetchInterval: 30_000,
  })

  const sortedDeployments = useMemo(
    () => [...(deployments.data ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
    [deployments.data]
  )

  useEffect(() => {
    if (sortedDeployments.length === 0) {
      setSelectedDeploymentID('')
      return
    }

    const selectedStillExists = sortedDeployments.some(deployment => deployment.id === selectedDeploymentID)
    if (!selectedStillExists) {
      setSelectedDeploymentID(sortedDeployments[0].id)
    }
  }, [selectedDeploymentID, sortedDeployments])

  const selectedDeployment = sortedDeployments.find(deployment => deployment.id === selectedDeploymentID)
  const deploymentLogs = useDeploymentLogsSSE(selectedDeploymentID, reconnectToken)
  const deploymentRecentLogs = useQuery({
    queryKey: ['deployment-recent-logs', selectedDeploymentID, deploymentRecentTail],
    queryFn: () => getDeploymentRecentLogs(selectedDeploymentID, deploymentRecentTail),
    enabled: deploymentLogMode === 'recent' && !!selectedDeploymentID,
    retry: false,
  })
  const deploymentLogContainerRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    if (!deploymentLogContainerRef.current) {
      return
    }
    deploymentLogContainerRef.current.scrollTop = deploymentLogContainerRef.current.scrollHeight
  }, [deploymentLogMode, deploymentLogs.lines, deploymentRecentLogs.data])

  return (
    <div className="space-y-5">
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Core services</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">API, orchestrator, and proxy logs</h2>
          </div>
          <div className="flex items-center gap-2">
            <Button type="button" size="sm" variant="outline" onClick={() => coreLogs.refetch()} disabled={coreLogs.isFetching}>
              {coreLogs.isFetching ? 'Refreshing...' : 'Refresh'}
            </Button>
          </div>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <label className="space-y-1">
            <span className="text-xs text-muted-foreground">Service</span>
            <select
              value={coreService}
              onChange={event => setCoreService(event.target.value as CoreService)}
              className="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1.5 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
            >
              {CORE_SERVICES.map(service => (
                <option key={service.value} value={service.value}>
                  {service.label}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-1">
            <span className="text-xs text-muted-foreground">History window</span>
            <select
              value={tail}
              onChange={event => setTail(Number(event.target.value))}
              className="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1.5 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
            >
              {TAIL_OPTIONS.map(option => (
                <option key={option} value={option}>
                  Last {option} lines
                </option>
              ))}
            </select>
          </label>

          <label className="flex items-center gap-2 pt-6 text-sm text-foreground">
            <input type="checkbox" checked={autoRefreshCore} onChange={event => setAutoRefreshCore(event.target.checked)} />
            Auto refresh every 5s
          </label>
        </div>

        <div className="mt-4 rounded-lg border border-border/60 bg-background/70 p-4">
          {coreLogs.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading service logs...</p>
          ) : coreLogs.isError ? (
            <p className="text-sm text-destructive">Unable to load service logs.</p>
          ) : coreLogs.data && coreLogs.data.lines.length > 0 ? (
            <pre className="h-[420px] overflow-y-auto font-mono text-xs leading-5 text-foreground/90 whitespace-pre-wrap break-all">{coreLogs.data.lines.join('\n')}</pre>
          ) : (
            <p className="text-sm text-muted-foreground">No log output was found for this service yet.</p>
          )}
        </div>
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Deployments</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Deployment run log stream</h2>
            <p className="mt-1 text-sm text-muted-foreground">Select a deployment to follow its live container output.</p>
          </div>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          <label className="space-y-1">
            <span className="text-xs text-muted-foreground">Deployment</span>
            <select
              value={selectedDeploymentID}
              onChange={event => setSelectedDeploymentID(event.target.value)}
              disabled={deployments.isLoading || deployments.isError || sortedDeployments.length === 0}
              className="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1.5 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {sortedDeployments.map(deployment => (
                <option key={deployment.id} value={deployment.id}>
                  {deployment.name}
                </option>
              ))}
            </select>
          </label>

          <div className="space-y-1">
            <span className="text-xs text-muted-foreground">View mode</span>
            <div className="flex h-9 items-center rounded-md border border-border/60 bg-background/70 p-1">
              <button
                type="button"
                className={`h-full flex-1 rounded px-2 text-xs font-medium transition-colors ${
                  deploymentLogMode === 'live' ? 'bg-card text-foreground' : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setDeploymentLogMode('live')}
              >
                Live stream
              </button>
              <button
                type="button"
                className={`h-full flex-1 rounded px-2 text-xs font-medium transition-colors ${
                  deploymentLogMode === 'recent' ? 'bg-card text-foreground' : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setDeploymentLogMode('recent')}
              >
                Recent snapshot
              </button>
            </div>
          </div>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          {deploymentLogMode === 'live' ? (
            <>
              <div className="flex h-8 items-center rounded-md border border-border/60 bg-background/70 px-3">
                <span className={`mr-2 h-1.5 w-1.5 rounded-full ${deploymentLogs.connected ? 'animate-pulse bg-emerald-500' : 'bg-muted-foreground/30'}`} />
                <span className="text-xs text-muted-foreground">
                  {selectedDeployment ? (deploymentLogs.connected ? 'Connected' : 'Connecting...') : 'No deployment selected'}
                </span>
              </div>
              <Button type="button" size="sm" variant="outline" onClick={() => setReconnectToken(prev => prev + 1)} disabled={!selectedDeploymentID}>
                Reconnect stream
              </Button>
            </>
          ) : (
            <>
              <select
                value={deploymentRecentTail}
                onChange={event => setDeploymentRecentTail(Number(event.target.value))}
                className="h-8 rounded-md border border-input bg-transparent px-2.5 text-xs text-foreground shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
              >
                {DEPLOYMENT_RECENT_TAIL_OPTIONS.map(option => (
                  <option key={option} value={option}>
                    Last {option} lines
                  </option>
                ))}
              </select>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => deploymentRecentLogs.refetch()}
                disabled={!selectedDeploymentID || deploymentRecentLogs.isFetching}
              >
                {deploymentRecentLogs.isFetching ? 'Refreshing...' : 'Refresh snapshot'}
              </Button>
            </>
          )}
        </div>

        <div className="mt-4 rounded-lg border border-border/60 bg-background/70 p-4">
          {deployments.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading deployments...</p>
          ) : deployments.isError ? (
            <p className="text-sm text-destructive">Unable to load deployments.</p>
          ) : !selectedDeployment ? (
            <p className="text-sm text-muted-foreground">No deployments available yet.</p>
          ) : deploymentLogMode === 'live' && deploymentLogs.lines.length > 0 ? (
            <pre ref={deploymentLogContainerRef} className="h-[420px] overflow-y-auto font-mono text-xs leading-5 text-foreground/90 whitespace-pre-wrap break-all">
              {deploymentLogs.lines.join('\n')}
            </pre>
          ) : deploymentLogMode === 'recent' && deploymentRecentLogs.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading recent deployment logs...</p>
          ) : deploymentLogMode === 'recent' && deploymentRecentLogs.isError ? (
            <p className="text-sm text-destructive">Unable to load recent deployment logs.</p>
          ) : deploymentLogMode === 'recent' && (deploymentRecentLogs.data?.lines.length ?? 0) > 0 ? (
            <pre ref={deploymentLogContainerRef} className="h-[420px] overflow-y-auto font-mono text-xs leading-5 text-foreground/90 whitespace-pre-wrap break-all">
              {deploymentRecentLogs.data?.lines.join('\n')}
            </pre>
          ) : (
            <p className="text-sm text-muted-foreground">
              {deploymentLogMode === 'live' ? 'Waiting for deployment log output...' : 'No recent logs captured for this deployment.'}
            </p>
          )}
        </div>
      </section>
    </div>
  )
}
