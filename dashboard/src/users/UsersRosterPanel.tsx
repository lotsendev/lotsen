import { KeyRound, Search, Trash2, UserRound } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import type { DashboardUser } from '../lib/api'

type UsersRosterPanelProps = {
  users: DashboardUser[]
  totalUsers: number
  search: string
  onSearchChange: (value: string) => void
  isLoading: boolean
  isError: boolean
  onRetry: () => void
  onResetPassword: (username: string) => void
  onDeleteUser: (username: string) => void
}

export function UsersRosterPanel({
  users,
  totalUsers,
  search,
  onSearchChange,
  isLoading,
  isError,
  onRetry,
  onResetPassword,
  onDeleteUser,
}: UsersRosterPanelProps) {
  return (
    <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
      <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Access roster</p>
      <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Manage operators</h2>
      <p className="mt-1 text-sm text-muted-foreground">Rotate credentials or revoke dashboard access without shell access.</p>

      <div className="mt-4 grid gap-2 sm:grid-cols-3">
        <article className="rounded-lg border border-border/60 bg-background/70 p-3">
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Total operators</p>
          <p className="mt-1 text-lg font-semibold text-foreground">{totalUsers}</p>
        </article>
        <article className="rounded-lg border border-border/60 bg-background/70 p-3">
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Shown</p>
          <p className="mt-1 text-lg font-semibold text-foreground">{users.length}</p>
        </article>
        <article className="rounded-lg border border-border/60 bg-background/70 p-3">
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Filters</p>
          <p className="mt-1 text-sm font-medium text-foreground">{search.trim() ? 'Search active' : 'All operators'}</p>
        </article>
      </div>

      <label className="mt-4 block space-y-1.5">
        <span className="text-xs text-muted-foreground">Search by username</span>
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={event => onSearchChange(event.target.value)} placeholder="alice, deploy-bot" className="pl-9" autoComplete="off" />
        </div>
      </label>

      <div className="mt-4 rounded-lg border border-border/60 bg-background/70 p-3">
        {isLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, index) => (
              <div key={index} className="h-[74px] rounded-lg border border-border/50 bg-background/70" />
            ))}
          </div>
        ) : isError ? (
          <div className="rounded-lg border border-destructive/35 bg-destructive/10 p-3 text-sm text-destructive">
            <p>Unable to load operators right now.</p>
            <Button type="button" size="sm" variant="outline" className="mt-3" onClick={onRetry}>
              Retry
            </Button>
          </div>
        ) : users.length === 0 ? (
          <div className="rounded-lg border border-border/50 bg-background/80 p-4 text-sm text-muted-foreground">
            {totalUsers === 0 ? 'No operators created yet. Add one above to unlock dashboard login.' : 'No operators match this search.'}
          </div>
        ) : (
          <div className="space-y-2">
            {users.map(user => (
              <article key={user.username} className="grid gap-3 rounded-lg border border-border/60 bg-background/80 p-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <div className="grid h-7 w-7 place-items-center rounded-md border border-border/60 bg-card">
                      <UserRound className="h-3.5 w-3.5 text-muted-foreground" />
                    </div>
                    <p className="truncate font-medium text-foreground">{user.username}</p>
                    <Badge variant="secondary">active</Badge>
                  </div>
                  <p className="mt-2 font-mono text-[11px] uppercase tracking-[0.08em] text-muted-foreground">Credential authority: local lotsen store</p>
                </div>

                <div className="flex flex-wrap justify-start gap-2 lg:justify-end">
                  <Button type="button" size="sm" variant="outline" onClick={() => onResetPassword(user.username)}>
                    <KeyRound className="h-3.5 w-3.5" />
                    Reset password
                  </Button>
                  <Button type="button" size="sm" variant="destructive" onClick={() => onDeleteUser(user.username)}>
                    <Trash2 className="h-3.5 w-3.5" />
                    Delete
                  </Button>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  )
}
