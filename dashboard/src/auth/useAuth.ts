import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { getMe, logout } from '../lib/api'

export const authQueryKey = ['auth', 'me'] as const

export function useAuth() {
  const { data, isLoading } = useQuery({
    queryKey: authQueryKey,
    queryFn: getMe,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })

  return {
    isLoading,
    isAuthDisabled: data?.status === 'disabled',
    isAuthenticated: data?.status === 'authenticated' || data?.status === 'disabled',
    username: data?.username,
  }
}

export function useLogout() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  return useMutation({
    mutationFn: logout,
    onSettled: () => {
      queryClient.setQueryData(authQueryKey, { status: 'unauthenticated' })
      navigate({ to: '/login', search: { redirect: undefined } })
    },
  })
}
