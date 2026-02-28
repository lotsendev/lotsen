import { useState } from 'react'
import type { Deployment } from '../lib/api'

export function useDeleteDeploymentDialog() {
  const [deploymentToDelete, setDeploymentToDelete] = useState<Deployment | null>(null)
  const [typedName, setTypedName] = useState('')

  const openDeleteDialog = (deployment: Deployment) => {
    setDeploymentToDelete(deployment)
    setTypedName('')
  }

  const closeDeleteDialog = () => {
    setDeploymentToDelete(null)
    setTypedName('')
  }

  const nameMatches = deploymentToDelete !== null && typedName === deploymentToDelete.name

  return {
    deploymentToDelete,
    typedName,
    setTypedName,
    nameMatches,
    openDeleteDialog,
    closeDeleteDialog,
  }
}
