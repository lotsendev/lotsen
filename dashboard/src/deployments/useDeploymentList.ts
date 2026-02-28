import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getDeployments, deleteDeployment } from '../lib/api'

export function useDeploymentList() {
  const queryClient = useQueryClient()

  const { data: deployments, isLoading, isError, refetch } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
    refetchInterval: 30_000,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDeployment,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['deployments'] }),
  })

  return { deployments, isLoading, isError, deleteMutation, refetch }
}
