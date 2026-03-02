import { useMutation, useQueryClient } from '@tanstack/react-query'
import { startAuthentication } from '@simplewebauthn/browser'
import { passkeyLoginBegin, passkeyLoginFinish } from '../lib/api'
import { authQueryKey } from './useAuth'

export function usePasskeyLogin(onSuccess: () => void) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (username?: string) => {
      const options = await passkeyLoginBegin(username)
      // startAuthentication handles ArrayBuffer ↔ base64url conversion.
      const response = await startAuthentication({ optionsJSON: options as never })
      await passkeyLoginFinish(response)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authQueryKey })
      onSuccess()
    },
  })
}
