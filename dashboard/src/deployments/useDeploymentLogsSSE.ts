import { useEffect, useState } from 'react'
import type { DeploymentLogEvent } from '../lib/api'

export function useDeploymentLogsSSE(deploymentId: string) {
  const [lines, setLines] = useState<string[]>([])

  useEffect(() => {
    setLines([])

    const es = new EventSource(`/api/deployments/${deploymentId}/logs`)

    es.onmessage = (event: MessageEvent) => {
      try {
        const { line }: DeploymentLogEvent = JSON.parse(event.data)
        setLines(prev => [...prev, line])
      } catch {
        // ignore malformed events
      }
    }

    return () => es.close()
  }, [deploymentId])

  return { lines }
}
