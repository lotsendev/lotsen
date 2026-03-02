import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Link2 } from 'lucide-react'
import { useAuth } from '../auth/useAuth'
import { createInvite, deleteUser, getUsers, type InviteLink } from '../lib/api'
import { DeleteUserDialog } from '../users/DeleteUserDialog'
import { UsersRosterPanel } from '../users/UsersRosterPanel'
import { Button } from '../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog'

export function UsersPage() {
  const { isAuthDisabled } = useAuth()
  const queryClient = useQueryClient()

  const [deleteUserName, setDeleteUserName] = useState<string | null>(null)
  const [deleteConfirmationInput, setDeleteConfirmationInput] = useState('')
  const [search, setSearch] = useState('')
  const [inviteLink, setInviteLink] = useState<InviteLink | null>(null)
  const [copied, setCopied] = useState(false)

  const usersQuery = useQuery({
    queryKey: ['users'],
    queryFn: getUsers,
    enabled: !isAuthDisabled,
  })

  const createInviteMutation = useMutation({
    mutationFn: createInvite,
    onSuccess: (link) => {
      setInviteLink(link)
      setCopied(false)
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: (username: string) => deleteUser(username),
    onSuccess: () => {
      setDeleteUserName(null)
      setDeleteConfirmationInput('')
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const normalizedSearch = search.trim().toLowerCase()
  const sortedUsers = [...(usersQuery.data ?? [])].sort((a, b) => a.username.localeCompare(b.username))
  const filteredUsers = normalizedSearch
    ? sortedUsers.filter(u => u.username.toLowerCase().includes(normalizedSearch))
    : sortedUsers

  const handleCopy = () => {
    if (!inviteLink) return
    navigator.clipboard.writeText(inviteLink.url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  if (isAuthDisabled) {
    return (
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Authentication disabled</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Operator controls unavailable</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          User management APIs are only available when authentication is enabled. Set{' '}
          <code>LOTSEN_JWT_SECRET</code> and <code>LOTSEN_RP_ID</code> in the API process.
        </p>
      </section>
    )
  }

  return (
    <div className="space-y-5">
      {/* Invite panel */}
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Crew onboarding</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Invite a new operator</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Generate a single-use invite link (valid 30 minutes). The recipient visits the link and registers their own passkey.
        </p>
        <div className="mt-4">
          <Button
            onClick={() => createInviteMutation.mutate()}
            disabled={createInviteMutation.isPending}
          >
            <Link2 className="mr-2 h-4 w-4" />
            {createInviteMutation.isPending ? 'Generating…' : 'Generate invite link'}
          </Button>
          {createInviteMutation.isError && (
            <p className="mt-2 text-sm text-destructive">
              {createInviteMutation.error instanceof Error
                ? createInviteMutation.error.message
                : 'Failed to create invite'}
            </p>
          )}
        </div>
      </section>

      <UsersRosterPanel
        users={filteredUsers}
        totalUsers={sortedUsers.length}
        search={search}
        onSearchChange={setSearch}
        isLoading={usersQuery.isLoading}
        isError={usersQuery.isError}
        onRetry={() => void usersQuery.refetch()}
        onDeleteUser={(username) => {
          deleteUserMutation.reset()
          setDeleteUserName(username)
          setDeleteConfirmationInput('')
        }}
      />

      <DeleteUserDialog
        username={deleteUserName}
        confirmation={deleteConfirmationInput}
        onConfirmationChange={setDeleteConfirmationInput}
        onClose={() => {
          setDeleteUserName(null)
          setDeleteConfirmationInput('')
        }}
        onDelete={() => {
          if (!deleteUserName) return
          deleteUserMutation.mutate(deleteUserName)
        }}
        canDelete={deleteUserName !== null && deleteConfirmationInput === deleteUserName}
        isDeleting={deleteUserMutation.isPending}
        error={deleteUserMutation.error instanceof Error ? deleteUserMutation.error.message : null}
      />

      {/* Invite link dialog */}
      <Dialog open={inviteLink !== null} onOpenChange={(open) => !open && setInviteLink(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Invite link ready</DialogTitle>
            <DialogDescription>
              Send this link to the new operator. It expires at{' '}
              {inviteLink ? new Date(inviteLink.expiresAt).toLocaleTimeString() : ''} and can only
              be used once.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-background/70 p-3">
            <code className="min-w-0 flex-1 truncate text-xs">{inviteLink?.url}</code>
            <Button type="button" size="sm" variant="outline" onClick={handleCopy} className="shrink-0">
              {copied ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Copy className="h-3.5 w-3.5" />}
              {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setInviteLink(null)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
