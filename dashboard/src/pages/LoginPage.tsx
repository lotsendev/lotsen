import { useNavigate, useSearch } from '@tanstack/react-router'
import { Fingerprint, Rocket } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { useAuth } from '../auth/useAuth'
import { usePasskeyLogin } from '../auth/usePasskeyLogin'
import { usePasskeyRegister } from '../auth/usePasskeyRegister'
import { getSetupAvailable } from '../lib/api'

export function LoginPage() {
  const { redirect } = useSearch({ strict: false }) as { redirect?: string }
  const navigate = useNavigate()
  const { isLoading, isAuthenticated, isAuthDisabled } = useAuth()
  const [username, setUsername] = useState('')

  const setupQuery = useQuery({
    queryKey: ['auth', 'setup-available'],
    queryFn: getSetupAvailable,
    retry: false,
    staleTime: 30_000,
  })

  const isFirstRun = setupQuery.data === true

  useEffect(() => {
    if (!isLoading && (isAuthenticated || isAuthDisabled)) {
      navigate({ to: (redirect as never) ?? '/deployments' })
    }
  }, [isLoading, isAuthenticated, isAuthDisabled, redirect, navigate])

  const loginMutation = usePasskeyLogin(() => {
    navigate({ to: (redirect as never) ?? '/deployments' })
  })

  const registerMutation = usePasskeyRegister(() => {
    navigate({ to: (redirect as never) ?? '/deployments' })
  })

  const pending = loginMutation.isPending || registerMutation.isPending
  const error = loginMutation.error ?? registerMutation.error

  const handleSetup = (e: React.FormEvent) => {
    e.preventDefault()
    const name = username.trim()
    if (!name) return
    registerMutation.mutate({ mode: 'setup', username: name })
  }

  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault()
    loginMutation.mutate(username.trim() || undefined)
  }

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
            {isFirstRun ? 'Create your account to get started' : 'Sign in to continue'}
          </p>
        </div>

        {isFirstRun ? (
          <form onSubmit={handleSetup} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
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
            {error && (
              <p className="text-sm text-destructive">{error instanceof Error ? error.message : 'Setup failed. Please try again.'}</p>
            )}
            <Button type="submit" className="w-full" disabled={pending}>
              <Fingerprint className="mr-2 h-4 w-4" />
              {pending ? 'Setting up…' : 'Set up with passkey'}
            </Button>
          </form>
        ) : (
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                type="text"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="optional hint"
              />
              <p className="text-[11px] text-muted-foreground">
                Leave blank to use a discoverable passkey
              </p>
            </div>
            {error && (
              <p className="text-sm text-destructive">{error instanceof Error ? error.message : 'Login failed. Please try again.'}</p>
            )}
            <Button type="submit" className="w-full" disabled={pending}>
              <Fingerprint className="mr-2 h-4 w-4" />
              {pending ? 'Signing in…' : 'Sign in with passkey'}
            </Button>
          </form>
        )}
      </div>
    </div>
  )
}
