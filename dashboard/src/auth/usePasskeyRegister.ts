import { useMutation, useQueryClient } from '@tanstack/react-query'
import { startRegistration } from '@simplewebauthn/browser'
import {
  passkeySetupBegin,
  passkeySetupFinish,
  passkeyInviteBegin,
  passkeyInviteFinish,
} from '../lib/api'
import { authQueryKey } from './useAuth'

type SetupArgs = { mode: 'setup'; username: string }
type InviteArgs = { mode: 'invite'; token: string; username: string }

export function usePasskeyRegister(onSuccess: () => void) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (args: SetupArgs | InviteArgs) => {
      let options: object

      if (args.mode === 'setup') {
        options = await passkeySetupBegin(args.username)
      } else {
        options = await passkeyInviteBegin(args.token, args.username)
      }

      // startRegistration handles ArrayBuffer ↔ base64url conversion.
      const response = await startRegistration({ optionsJSON: options as never })

      if (args.mode === 'setup') {
        await passkeySetupFinish(args.username, response)
      } else {
        await passkeyInviteFinish(args.token, args.username, response)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authQueryKey })
      onSuccess()
    },
  })
}
