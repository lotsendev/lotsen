import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { useAuth } from '../auth/useAuth'
import { createUser, deleteUser, getUsers, updateUserPassword } from '../lib/api'

export function UsersPage() {
  const { isAuthDisabled } = useAuth()
  const queryClient = useQueryClient()
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [passwordResetUser, setPasswordResetUser] = useState<string | null>(null)
  const [passwordResetValue, setPasswordResetValue] = useState('')
  const [deleteUserName, setDeleteUserName] = useState<string | null>(null)
  const [deleteConfirmationInput, setDeleteConfirmationInput] = useState('')

  const usersQuery = useQuery({
    queryKey: ['users'],
    queryFn: getUsers,
    enabled: !isAuthDisabled,
  })

  const createUserMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) => createUser(username, password),
    onSuccess: () => {
      setNewUsername('')
      setNewPassword('')
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const updatePasswordMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) => updateUserPassword(username, password),
    onSuccess: () => {
      setPasswordResetUser(null)
      setPasswordResetValue('')
      queryClient.invalidateQueries({ queryKey: ['users'] })
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

  const sortedUsers = useMemo(
    () => [...(usersQuery.data ?? [])].sort((a, b) => a.username.localeCompare(b.username)),
    [usersQuery.data],
  )

  const createError = createUserMutation.error instanceof Error ? createUserMutation.error.message : null
  const passwordError = updatePasswordMutation.error instanceof Error ? updatePasswordMutation.error.message : null
  const deleteError = deleteUserMutation.error instanceof Error ? deleteUserMutation.error.message : null
  const canConfirmDelete = deleteUserName !== null && deleteConfirmationInput === deleteUserName

  if (isAuthDisabled) {
    return (
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Authentication disabled</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">User management unavailable</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          User management APIs are only available when authentication is enabled. In local dev, set
          {' '}<code>LOTSEN_JWT_SECRET</code>{' '}
          (and optionally
          {' '}<code>LOTSEN_AUTH_USER</code>{' '}
          +
          {' '}<code>LOTSEN_AUTH_PASSWORD</code>{' '}
          for first-user bootstrap) in the API process.
        </p>
      </section>
    )
  }

  return (
    <div className="space-y-5">
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Create user</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Add dashboard user</h2>
        <p className="mt-1 text-sm text-muted-foreground">Create a user account that can sign in to the dashboard.</p>

        <form
          className="mt-4 grid gap-3 sm:grid-cols-[1fr_1fr_auto]"
          onSubmit={(event) => {
            event.preventDefault()
            createUserMutation.mutate({ username: newUsername.trim(), password: newPassword.trim() })
          }}
        >
          <label className="space-y-1">
            <span className="text-xs text-muted-foreground">Username</span>
            <Input value={newUsername} onChange={event => setNewUsername(event.target.value)} required autoComplete="off" />
          </label>
          <label className="space-y-1">
            <span className="text-xs text-muted-foreground">Password</span>
            <Input type="password" value={newPassword} onChange={event => setNewPassword(event.target.value)} required autoComplete="new-password" />
          </label>
          <Button type="submit" className="mt-auto" disabled={createUserMutation.isPending}>
            {createUserMutation.isPending ? 'Creating...' : 'Add user'}
          </Button>
        </form>

        {createError && <p className="mt-3 text-sm text-destructive">{createError}</p>}
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">User accounts</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Manage existing users</h2>
        <p className="mt-1 text-sm text-muted-foreground">Reset passwords or remove dashboard access.</p>

        <div className="mt-4 rounded-lg border border-border/60 bg-background/70 p-3">
          {usersQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading users...</p>
          ) : usersQuery.isError ? (
            <p className="text-sm text-destructive">Unable to load users right now.</p>
          ) : sortedUsers.length === 0 ? (
            <p className="text-sm text-muted-foreground">No users found.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead className="w-[240px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedUsers.map(user => (
                  <TableRow key={user.username}>
                    <TableCell className="font-medium">{user.username}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setPasswordResetUser(user.username)
                            setPasswordResetValue('')
                            updatePasswordMutation.reset()
                          }}
                        >
                          Reset password
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="destructive"
                          onClick={() => {
                            setDeleteUserName(user.username)
                            setDeleteConfirmationInput('')
                            deleteUserMutation.reset()
                          }}
                        >
                          Delete
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </section>

      <Dialog
        open={passwordResetUser !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPasswordResetUser(null)
            setPasswordResetValue('')
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Reset password</DialogTitle>
            <DialogDescription>
              {passwordResetUser ? `Set a new password for ${passwordResetUser}.` : 'Set a new password.'}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault()
              if (!passwordResetUser) return
              updatePasswordMutation.mutate({ username: passwordResetUser, password: passwordResetValue.trim() })
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="reset-password">New password</Label>
              <Input
                id="reset-password"
                type="password"
                autoComplete="new-password"
                value={passwordResetValue}
                onChange={event => setPasswordResetValue(event.target.value)}
                required
              />
            </div>
            {passwordError && <p className="text-sm text-destructive">{passwordError}</p>}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setPasswordResetUser(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={updatePasswordMutation.isPending || !passwordResetUser}>
                {updatePasswordMutation.isPending ? 'Updating...' : 'Update password'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={deleteUserName !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteUserName(null)
            setDeleteConfirmationInput('')
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete user</DialogTitle>
            <DialogDescription>
              {deleteUserName ? `Type ${deleteUserName} to confirm account removal.` : 'Confirm account removal.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="delete-user-confirm">Confirm username</Label>
              <Input
                id="delete-user-confirm"
                autoComplete="off"
                value={deleteConfirmationInput}
                onChange={event => setDeleteConfirmationInput(event.target.value)}
              />
            </div>
            {deleteError && <p className="text-sm text-destructive">{deleteError}</p>}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDeleteUserName(null)}>
                Cancel
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={!canConfirmDelete || deleteUserMutation.isPending || !deleteUserName}
                onClick={() => {
                  if (!deleteUserName) return
                  deleteUserMutation.mutate(deleteUserName)
                }}
              >
                {deleteUserMutation.isPending ? 'Deleting...' : 'Delete user'}
              </Button>
            </DialogFooter>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
