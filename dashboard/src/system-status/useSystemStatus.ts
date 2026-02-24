import { useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getSystemStatus, type SystemStatusSnapshot } from '../lib/api'

export function useSystemStatus() {
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['system-status'],
    queryFn: getSystemStatus,
    retry: false,
  })

  useEffect(() => {
    if (typeof EventSource === 'undefined') {
      return
    }

    const es = new EventSource('/api/system-status/events')

    es.onmessage = (event: MessageEvent) => {
      try {
        const snapshot: SystemStatusSnapshot = JSON.parse(event.data)
        queryClient.setQueryData(['system-status'], snapshot)
      } catch {
        // ignore malformed events
      }
    }

    return () => es.close()
  }, [queryClient])

  return {
    status: query.data,
    isLoading: query.isLoading && !query.data,
    isError: query.isError && !query.data,
  }
}
