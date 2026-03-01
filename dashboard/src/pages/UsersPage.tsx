import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../auth/useAuth'
import { createUser, deleteUser, getUsers, updateUserPassword } from '../lib/api'
import { CreateUserPanel } from '../users/CreateUserPanel'
import { DeleteUserDialog } from '../users/DeleteUserDialog'
import { PasswordResetDialog } from '../users/PasswordResetDialog'
import { UsersRosterPanel } from '../users/UsersRosterPanel'
import { useUsersPageState } from '../users/useUsersPageState'

export function UsersPage() {
  const { isAuthDisabled } = useAuth()
  const queryClient = useQueryClient()
  const state = useUsersPageState()

  const usersQuery = useQuery({
    queryKey: ['users'],
    queryFn: getUsers,
    enabled: !isAuthDisabled,
  })

  const createUserMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) => createUser(username, password),
    onSuccess: () => {
      state.resetCreateForm()
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const updatePasswordMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) => updateUserPassword(username, password),
    onSuccess: () => {
      state.closePasswordReset()
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: (username: string) => deleteUser(username),
    onSuccess: () => {
      state.closeDeleteDialog()
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const sortedUsers = useMemo(
    () => [...(usersQuery.data ?? [])].sort((a, b) => a.username.localeCompare(b.username)),
    [usersQuery.data],
  )

  const filteredUsers = useMemo(() => {
    if (!state.normalizedSearch) {
      return sortedUsers
    }

    return sortedUsers.filter(user => user.username.toLowerCase().includes(state.normalizedSearch))
  }, [sortedUsers, state.normalizedSearch])

  const createError = createUserMutation.error instanceof Error ? createUserMutation.error.message : null
  const passwordError = updatePasswordMutation.error instanceof Error ? updatePasswordMutation.error.message : null
  const deleteError = deleteUserMutation.error instanceof Error ? deleteUserMutation.error.message : null

  if (isAuthDisabled) {
    return (
      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Authentication disabled</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Operator controls unavailable</h2>
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
      <CreateUserPanel
        username={state.newUsername}
        password={state.newPassword}
        onUsernameChange={state.setNewUsername}
        onPasswordChange={state.setNewPassword}
        onSubmit={() => createUserMutation.mutate({ username: state.newUsername.trim(), password: state.newPassword.trim() })}
        isSubmitting={createUserMutation.isPending}
        error={createError}
      />

      <UsersRosterPanel
        users={filteredUsers}
        totalUsers={sortedUsers.length}
        search={state.search}
        onSearchChange={state.setSearch}
        isLoading={usersQuery.isLoading}
        isError={usersQuery.isError}
        onRetry={() => {
          void usersQuery.refetch()
        }}
        onResetPassword={(username) => {
          updatePasswordMutation.reset()
          state.openPasswordReset(username)
        }}
        onDeleteUser={(username) => {
          deleteUserMutation.reset()
          state.openDeleteDialog(username)
        }}
      />

      <PasswordResetDialog
        username={state.passwordResetUser}
        value={state.passwordResetValue}
        onValueChange={state.setPasswordResetValue}
        onClose={state.closePasswordReset}
        onSubmit={() => {
          if (!state.passwordResetUser) return
          updatePasswordMutation.mutate({ username: state.passwordResetUser, password: state.passwordResetValue.trim() })
        }}
        isSubmitting={updatePasswordMutation.isPending}
        error={passwordError}
      />

      <DeleteUserDialog
        username={state.deleteUserName}
        confirmation={state.deleteConfirmationInput}
        onConfirmationChange={state.setDeleteConfirmationInput}
        onClose={state.closeDeleteDialog}
        onDelete={() => {
          if (!state.deleteUserName) return
          deleteUserMutation.mutate(state.deleteUserName)
        }}
        canDelete={state.canConfirmDelete}
        isDeleting={deleteUserMutation.isPending}
        error={deleteError}
      />
    </div>
  )
}
