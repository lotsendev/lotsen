import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { login } from '../lib/api'
import { authQueryKey } from './useAuth'

export function useLoginForm(onSuccess: () => void) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: () => login(username, password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authQueryKey })
      onSuccess()
    },
  })

  return { username, setUsername, password, setPassword, mutation }
}
