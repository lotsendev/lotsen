import { Plus } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../components/ui/dialog'
import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeploymentDialogs } from '../deployments/useDeploymentDialogs'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'

export default function DeploymentList() {
  useDeploymentSSE()
  const listState = useDeploymentList()
  const {
    createDialogOpen,
    openCreateDialog,
    closeCreateDialog,
    setCreateDialogOpen,
    editingDeployment,
    openEditDialog,
    closeEditDialog,
  } = useDeploymentDialogs()

  return (
    <>
      <div className="mb-6 flex items-center justify-end">
        <Button type="button" onClick={openCreateDialog}>
          <Plus size={16} />
          Create deployment
        </Button>
      </div>

      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>New deployment</DialogTitle>
            <DialogDescription>Create a new service from an image and runtime settings.</DialogDescription>
          </DialogHeader>
          <CreateDeploymentForm
            onSuccess={closeCreateDialog}
            className="mb-0 border-0 shadow-none"
            hideHeader
          />
        </DialogContent>
      </Dialog>

      <Dialog open={editingDeployment !== null} onOpenChange={open => !open && closeEditDialog()}>
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
              onClose={closeEditDialog}
              className="mb-0 border-0 shadow-none"
              hideHeader
            />
          )}
        </DialogContent>
      </Dialog>

      <DeploymentTable {...listState} onEdit={openEditDialog} onCreate={openCreateDialog} />
    </>
  )
}
