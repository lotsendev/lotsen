import { useNavigate, useSearch } from '@tanstack/react-router'
import { Fingerprint, Rocket } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { usePasskeyRegister } from '../auth/usePasskeyRegister'
import { validateInvite } from '../lib/api'

export function JoinPage() {
  const { token } = useSearch({ strict: false }) as { token?: string }
  const navigate = useNavigate()
  const [username, setUsername] = useState('')

  const validateQuery = useQuery({
    queryKey: ['auth', 'invite', token],
    queryFn: () => validateInvite(token!),
    enabled: !!token,
    retry: false,
    staleTime: 60_000,
  })

  const registerMutation = usePasskeyRegister(() => {
    navigate({ to: '/deployments' })
  })

  useEffect(() => {
    if (!token) {
      navigate({ to: '/login', search: { redirect: undefined } })
    }
  }, [token, navigate])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const name = username.trim()
    if (!name || !token) return
    registerMutation.mutate({ mode: 'invite', token, username: name })
  }

  const isInvalid = validateQuery.data && !validateQuery.data.valid

  return (
    <div className="chart-grid-overlay flex min-h-screen items-center justify-center bg-background px-4 py-10">
      <div className="w-full max-w-sm space-y-8 rounded-2xl border border-border/70 bg-card/92 p-6 shadow-sm backdrop-blur sm:p-7">
        <div className="flex flex-col items-center gap-3">
          <div className="grid h-10 w-10 place-items-center rounded-xl border border-primary/30 bg-primary/12 text-primary">
            <Rocket className="h-5 w-5" />
          </div>
          <h1 className="font-[family-name:var(--font-display)] text-2xl font-semibold italic tracking-tight">
            lotsen
          </h1>
          <p className="font-mono text-[11px] uppercase tracking-[0.13em] text-muted-foreground">
            Create your account
          </p>
        </div>

        {isInvalid ? (
          <div className="rounded-lg border border-destructive/35 bg-destructive/10 p-4 text-sm text-destructive">
            <p className="font-medium">Invalid or expired invite link</p>
            <p className="mt-1 text-destructive/80">
              {validateQuery.data?.reason ?? 'This invite link is no longer valid.'}
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Choose a username</Label>
              <Input
                id="username"
                type="text"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                placeholder="your name"
              />
            </div>
            {registerMutation.isError && (
              <p className="text-sm text-destructive">
                {registerMutation.error instanceof Error
                  ? registerMutation.error.message
                  : 'Registration failed. Please try again.'}
              </p>
            )}
            <Button
              type="submit"
              className="w-full"
              disabled={registerMutation.isPending || validateQuery.isLoading}
            >
              <Fingerprint className="mr-2 h-4 w-4" />
              {registerMutation.isPending ? 'Creating account…' : 'Create account with passkey'}
            </Button>
          </form>
        )}
      </div>
    </div>
  )
}
