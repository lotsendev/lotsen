import { useNavigate, useSearch } from '@tanstack/react-router'
import { Fingerprint, Rocket } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
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

  const completeAuthRedirect = useCallback(() => {
    const fallback = '/deployments'
    const target = sanitizeRedirectTarget(redirect)
    if (!target) {
      navigate({ to: fallback })
      return
    }

    if (target.startsWith('/')) {
      navigate({ to: target as never })
      return
    }

    window.location.assign(target)
  }, [navigate, redirect])

  const setupQuery = useQuery({
    queryKey: ['auth', 'setup-available'],
    queryFn: getSetupAvailable,
    retry: false,
    staleTime: 30_000,
  })

  const isFirstRun = setupQuery.data === true

  useEffect(() => {
    if (!isLoading && (isAuthenticated || isAuthDisabled)) {
      completeAuthRedirect()
    }
  }, [isLoading, isAuthenticated, isAuthDisabled, completeAuthRedirect])

  const loginMutation = usePasskeyLogin(() => {
    completeAuthRedirect()
  })

  const registerMutation = usePasskeyRegister(() => {
    completeAuthRedirect()
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

function sanitizeRedirectTarget(redirect?: string): string | undefined {
  const raw = redirect?.trim()
  if (!raw) return undefined
  if (raw.startsWith('/')) return raw

  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    return undefined
  }

  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
    return undefined
  }

  if (typeof window === 'undefined') {
    return undefined
  }

  const currentHost = window.location.hostname.toLowerCase()
  const allowedSuffix = currentHost.split('.').slice(1).join('.')
  const targetHost = parsed.hostname.toLowerCase()
  if (!allowedSuffix || (targetHost !== currentHost && !targetHost.endsWith(`.${allowedSuffix}`))) {
    return undefined
  }

  return parsed.toString()
}
