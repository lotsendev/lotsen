import { useState } from 'react'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../components/ui/dialog'
import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'
import type { Deployment } from '../lib/api'

export default function DeploymentList() {
  useDeploymentSSE()
  const listState = useDeploymentList()
  const [editingDeployment, setEditingDeployment] = useState<Deployment | null>(null)

  return (
    <>
      <CreateDeploymentForm />

      <Dialog open={editingDeployment !== null} onOpenChange={open => !open && setEditingDeployment(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Edit deployment</DialogTitle>
            <DialogDescription>
              Update runtime settings for <span className="font-medium text-foreground">{editingDeployment?.name}</span>.
            </DialogDescription>
          </DialogHeader>
          {editingDeployment && (
            <EditDeploymentForm
              key={editingDeployment.id}
              deployment={editingDeployment}
              onClose={() => setEditingDeployment(null)}
              className="mb-0 border-0 shadow-none"
              hideHeader
            />
          )}
        </DialogContent>
      </Dialog>

      <DeploymentTable {...listState} onEdit={setEditingDeployment} />
    </>
  )
}
