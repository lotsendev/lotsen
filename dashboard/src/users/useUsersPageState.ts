import { useMemo, useState } from 'react'

export function useUsersPageState() {
  const [search, setSearch] = useState('')
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [passwordResetUser, setPasswordResetUser] = useState<string | null>(null)
  const [passwordResetValue, setPasswordResetValue] = useState('')
  const [deleteUserName, setDeleteUserName] = useState<string | null>(null)
  const [deleteConfirmationInput, setDeleteConfirmationInput] = useState('')

  const normalizedSearch = useMemo(() => search.trim().toLowerCase(), [search])
  const canConfirmDelete = deleteUserName !== null && deleteConfirmationInput === deleteUserName

  const resetCreateForm = () => {
    setNewUsername('')
    setNewPassword('')
  }

  const openPasswordReset = (username: string) => {
    setPasswordResetUser(username)
    setPasswordResetValue('')
  }

  const closePasswordReset = () => {
    setPasswordResetUser(null)
    setPasswordResetValue('')
  }

  const openDeleteDialog = (username: string) => {
    setDeleteUserName(username)
    setDeleteConfirmationInput('')
  }

  const closeDeleteDialog = () => {
    setDeleteUserName(null)
    setDeleteConfirmationInput('')
  }

  return {
    search,
    setSearch,
    normalizedSearch,
    newUsername,
    setNewUsername,
    newPassword,
    setNewPassword,
    resetCreateForm,
    passwordResetUser,
    passwordResetValue,
    setPasswordResetValue,
    openPasswordReset,
    closePasswordReset,
    deleteUserName,
    deleteConfirmationInput,
    setDeleteConfirmationInput,
    canConfirmDelete,
    openDeleteDialog,
    closeDeleteDialog,
  }
}
