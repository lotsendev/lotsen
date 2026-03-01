import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'

type CreateUserPanelProps = {
  username: string
  password: string
  onUsernameChange: (value: string) => void
  onPasswordChange: (value: string) => void
  onSubmit: () => void
  isSubmitting: boolean
  error: string | null
}

export function CreateUserPanel({
  username,
  password,
  onUsernameChange,
  onPasswordChange,
  onSubmit,
  isSubmitting,
  error,
}: CreateUserPanelProps) {
  return (
    <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
      <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Crew onboarding</p>
      <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Add dashboard operator</h2>
      <p className="mt-1 text-sm text-muted-foreground">Create credentials for someone who needs command deck access.</p>

      <form
        className="mt-4 rounded-lg border border-border/60 bg-background/70 p-3"
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        <div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <label className="space-y-1.5">
            <span className="text-xs text-muted-foreground">Username</span>
            <Input value={username} onChange={event => onUsernameChange(event.target.value)} required autoComplete="off" />
          </label>
          <label className="space-y-1.5">
            <span className="text-xs text-muted-foreground">Password</span>
            <Input type="password" value={password} onChange={event => onPasswordChange(event.target.value)} required autoComplete="new-password" />
          </label>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Creating...' : 'Add operator'}
          </Button>
        </div>
        {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
      </form>
    </section>
  )
}
