import { useCallback, useEffect, useState } from 'react'
import {
  getLoadBalancerAccessLogs,
  type LoadBalancerAccessLogEntry,
  type LoadBalancerAccessLogFilters,
} from '../lib/api'

const PAGE_SIZE = 50

export function useLoadBalancerAccessLogs(filters: LoadBalancerAccessLogFilters) {
  const [items, setItems] = useState<LoadBalancerAccessLogEntry[]>([])
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined)
  const [hasMore, setHasMore] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [isError, setIsError] = useState(false)

  const loadPage = useCallback(async (cursor?: string, append = false) => {
    setIsLoading(true)
    setIsError(false)
    try {
      const page = await getLoadBalancerAccessLogs(cursor, PAGE_SIZE, filters)
      setItems(prev => (append ? [...prev, ...page.items] : page.items))
      setNextCursor(page.nextCursor)
      setHasMore(page.hasMore)
    } catch {
      setIsError(true)
    } finally {
      setIsLoading(false)
    }
  }, [filters])

  useEffect(() => {
    loadPage(undefined, false)
  }, [loadPage])

  const loadOlder = useCallback(() => {
    if (isLoading || !nextCursor) {
      return
    }
    void loadPage(nextCursor, true)
  }, [isLoading, nextCursor, loadPage])

  return { items, hasMore, isLoading, isError, loadOlder }
}
