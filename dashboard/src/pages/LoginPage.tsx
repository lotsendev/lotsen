import { useNavigate, useSearch } from '@tanstack/react-router'
import { Rocket } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { useLoginForm } from '../auth/useLoginForm'
import { UnauthorizedError } from '../lib/api'

export function LoginPage() {
  const { redirect } = useSearch({ strict: false }) as { redirect?: string }
  const navigate = useNavigate()

  const { username, setUsername, password, setPassword, mutation } = useLoginForm(() => {
    navigate({ to: (redirect as never) ?? '/deployments' })
  })

  const errorMessage = mutation.isError
    ? mutation.error instanceof UnauthorizedError
      ? 'Invalid username or password.'
      : 'Login failed. Please try again.'
    : null

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-8">
        <div className="flex flex-col items-center gap-3">
          <div className="grid h-10 w-10 place-items-center rounded-xl bg-primary text-primary-foreground">
            <Rocket className="h-5 w-5" />
          </div>
          <h1 className="font-[family-name:var(--font-display)] text-2xl font-bold tracking-tight">
            lotsen
          </h1>
          <p className="text-sm text-muted-foreground">Sign in to continue</p>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            mutation.mutate()
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              type="text"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          {errorMessage && (
            <p className="text-sm text-destructive">{errorMessage}</p>
          )}
          <Button type="submit" className="w-full" disabled={mutation.isPending}>
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  )
}
