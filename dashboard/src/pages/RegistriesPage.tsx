import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { createRegistry, deleteRegistry, getRegistries, updateRegistry, type RegistryCredential } from '../lib/api'

export function RegistriesPage() {
  const queryClient = useQueryClient()
  const [registryPrefix, setRegistryPrefix] = useState('')
  const [registryUsername, setRegistryUsername] = useState('')
  const [registryPassword, setRegistryPassword] = useState('')
  const [registryError, setRegistryError] = useState<string | null>(null)
  const [editingRegistryId, setEditingRegistryId] = useState<string | null>(null)

  const registriesQuery = useQuery({
    queryKey: ['registries'],
    queryFn: getRegistries,
  })

  const resetRegistryForm = () => {
    setRegistryPrefix('')
    setRegistryUsername('')
    setRegistryPassword('')
    setEditingRegistryId(null)
  }

  const upsertRegistryMutation = useMutation({
    mutationFn: async () => {
      if (editingRegistryId) {
        return updateRegistry(editingRegistryId, registryPrefix, registryUsername, registryPassword || undefined)
      }
      return createRegistry(registryPrefix, registryUsername, registryPassword)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registries'] })
      setRegistryError(null)
      resetRegistryForm()
    },
    onError: error => {
      setRegistryError(error instanceof Error ? error.message : 'Failed to save registry')
    },
  })

  const deleteRegistryMutation = useMutation({
    mutationFn: deleteRegistry,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registries'] })
      setRegistryError(null)
      if (editingRegistryId) {
        resetRegistryForm()
      }
    },
    onError: error => {
      setRegistryError(error instanceof Error ? error.message : 'Failed to delete registry')
    },
  })

  const beginEditRegistry = (registry: RegistryCredential) => {
    setEditingRegistryId(registry.id)
    setRegistryPrefix(registry.prefix)
    setRegistryUsername(registry.username)
    setRegistryPassword('')
    setRegistryError(null)
  }

  return (
    <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Registry credentials</p>
          <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Private image access</h2>
          <p className="mt-1 text-sm text-muted-foreground">Save registry prefixes once and Lotsen reuses credentials automatically for matching pulls.</p>
        </div>
        <Badge variant="outline" className="rounded-md border-border/70 bg-background/70 px-2.5 py-1 font-mono text-[11px]">
          {(registriesQuery.data ?? []).length} configured
        </Badge>
      </div>

      <form
        className="mt-4 grid gap-2 md:grid-cols-[1.4fr_1fr_1fr_auto]"
        onSubmit={event => {
          event.preventDefault()
          if (!registryPrefix.trim() || !registryUsername.trim() || (!editingRegistryId && !registryPassword.trim())) {
            setRegistryError('Prefix, username, and password are required for new registries')
            return
          }
          upsertRegistryMutation.mutate()
        }}
      >
        <Input value={registryPrefix} onChange={event => setRegistryPrefix(event.target.value)} placeholder="ghcr.io/myorg" aria-label="Registry prefix" />
        <Input value={registryUsername} onChange={event => setRegistryUsername(event.target.value)} placeholder="Username" aria-label="Registry username" />
        <Input
          type="password"
          value={registryPassword}
          onChange={event => setRegistryPassword(event.target.value)}
          placeholder={editingRegistryId ? 'New password (optional)' : 'Password or token'}
          aria-label="Registry password"
        />
        <div className="flex gap-2">
          <Button type="submit" disabled={upsertRegistryMutation.isPending}>
            {editingRegistryId ? 'Save' : 'Add'}
          </Button>
          {editingRegistryId && (
            <Button type="button" variant="outline" onClick={resetRegistryForm}>
              Cancel
            </Button>
          )}
        </div>
      </form>

      {registryError && <p className="mt-2 text-sm text-destructive">{registryError}</p>}

      <div className="mt-4 overflow-x-auto rounded-lg border border-border/60 bg-background/70">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border/60 text-left text-xs uppercase tracking-[0.13em] text-muted-foreground">
              <th className="px-3 py-2">Prefix</th>
              <th className="px-3 py-2">Username</th>
              <th className="px-3 py-2 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(registriesQuery.data ?? []).map(registry => (
              <tr key={registry.id} className="border-b border-border/40 last:border-0">
                <td className="px-3 py-2 font-mono text-xs text-foreground">{registry.prefix}</td>
                <td className="px-3 py-2 text-foreground">{registry.username}</td>
                <td className="px-3 py-2">
                  <div className="flex justify-end gap-2">
                    <Button type="button" size="sm" variant="outline" onClick={() => beginEditRegistry(registry)}>
                      Edit
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={deleteRegistryMutation.isPending}
                      onClick={() => deleteRegistryMutation.mutate(registry.id)}
                    >
                      Delete
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {registriesQuery.isLoading && <p className="px-3 py-3 text-sm text-muted-foreground">Loading registries...</p>}
        {!registriesQuery.isLoading && (registriesQuery.data ?? []).length === 0 && (
          <p className="px-3 py-3 text-sm text-muted-foreground">No registries configured.</p>
        )}
      </div>
    </section>
  )
}
