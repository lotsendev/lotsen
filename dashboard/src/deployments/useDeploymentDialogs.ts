import { useState } from 'react'
import type { Deployment } from '../lib/api'

export function useDeploymentDialogs() {
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [editingDeployment, setEditingDeployment] = useState<Deployment | null>(null)

  return {
    createDialogOpen,
    openCreateDialog: () => setCreateDialogOpen(true),
    closeCreateDialog: () => setCreateDialogOpen(false),
    setCreateDialogOpen,
    editingDeployment,
    openEditDialog: (deployment: Deployment) => setEditingDeployment(deployment),
    closeEditDialog: () => setEditingDeployment(null),
  }
}
