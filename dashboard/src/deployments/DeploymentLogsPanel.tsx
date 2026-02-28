import { useEffect, useRef } from 'react'
import type { DeploymentStatus } from '../lib/api'
import { useDeploymentLogsSSE } from './useDeploymentLogsSSE'

type Props = {
  deploymentId: string
  status: DeploymentStatus
  error?: string
}

export function DeploymentLogsPanel({ deploymentId, status, error }: Props) {
  const { lines, connected } = useDeploymentLogsSSE(deploymentId)
  const logContainerRef = useRef<HTMLPreElement | null>(null)

  const emptyState =
    status === 'failed'
      ? `No container logs were captured for this failed deployment.${error ? ` Last error: ${error}` : ''}`
      : 'Waiting for log output...'

  useEffect(() => {
    if (!logContainerRef.current) return
    logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
  }, [lines])

  return (
    <div className="rounded-xl border border-border/60 bg-card p-4">
      <div className="mb-3 flex items-center justify-between">
        <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">Live logs</p>
        <div className="flex items-center gap-3">
          {lines.length > 0 && (
            <span className="font-mono text-[11px] text-muted-foreground/40">{lines.length} lines</span>
          )}
          <div className="flex items-center gap-1.5">
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                connected ? 'animate-pulse bg-emerald-500' : 'bg-muted-foreground/30'
              }`}
            />
            <span className="text-[11px] text-muted-foreground/50">{connected ? 'Streaming' : 'Connecting'}</span>
          </div>
        </div>
      </div>
      <pre
        ref={logContainerRef}
        className="h-[420px] overflow-y-auto rounded-lg border border-border/40 bg-background/70 p-4 font-mono text-xs leading-5 text-foreground/90 whitespace-pre-wrap break-all"
      >
        {lines.length ? (
          lines.join('\n')
        ) : (
          <span className="text-muted-foreground/50">{emptyState}</span>
        )}
      </pre>
    </div>
  )
}
