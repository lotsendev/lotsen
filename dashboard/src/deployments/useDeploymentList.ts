import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getDeployments, deleteDeployment } from '../lib/api'

export function useDeploymentList() {
  const queryClient = useQueryClient()

  const { data: deployments, isLoading, isError } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDeployment,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['deployments'] }),
  })

  return { deployments, isLoading, isError, deleteMutation }
}
