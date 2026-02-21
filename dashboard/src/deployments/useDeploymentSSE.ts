import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { Deployment, StatusEvent } from '../lib/api'

export function useDeploymentSSE() {
  const queryClient = useQueryClient()

  useEffect(() => {
    const es = new EventSource('/api/deployments/events')

    es.onmessage = (event: MessageEvent) => {
      try {
        const { deploymentId, status, error }: StatusEvent = JSON.parse(event.data)
        queryClient.setQueryData<Deployment[]>(['deployments'], prev =>
          prev?.map(d => (d.id === deploymentId ? { ...d, status, error } : d))
        )
      } catch {
        // ignore malformed events
      }
    }

    return () => es.close()
  }, [queryClient])
}
