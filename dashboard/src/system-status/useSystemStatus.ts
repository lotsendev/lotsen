import { useQuery } from '@tanstack/react-query'
import { getSystemStatus } from '../lib/api'

export function useSystemStatus() {
  const query = useQuery({
    queryKey: ['system-status'],
    queryFn: getSystemStatus,
  })

  return {
    status: query.data,
    isLoading: query.isLoading,
    isError: query.isError,
  }
}
