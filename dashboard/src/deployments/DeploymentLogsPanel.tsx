import { useEffect, useRef } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { useDeploymentLogsSSE } from './useDeploymentLogsSSE'

type Props = {
  deploymentId: string
}

export function DeploymentLogsPanel({ deploymentId }: Props) {
  const { lines } = useDeploymentLogsSSE(deploymentId)
  const logContainerRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    if (!logContainerRef.current) return
    logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
  }, [lines])

  return (
    <Card>
      <CardHeader>
        <CardTitle>Live logs</CardTitle>
        <CardDescription>Last 100 lines are shown immediately, then new lines stream live.</CardDescription>
      </CardHeader>
      <CardContent>
        <pre
          ref={logContainerRef}
          className="h-80 overflow-y-auto rounded-lg border bg-muted/30 p-4 font-mono text-xs leading-5 text-foreground"
        >
          {lines.length ? lines.join('\n') : 'Waiting for log output...'}
        </pre>
      </CardContent>
    </Card>
  )
}
