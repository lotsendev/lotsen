import { Plus } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeleteDeploymentDialog } from '../deployments/useDeleteDeploymentDialog'
import { useDeploymentDialogs } from '../deployments/useDeploymentDialogs'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'

export default function DeploymentList() {
  useDeploymentSSE()
  const { deployments, isLoading, isError, deleteMutation } = useDeploymentList()
  const {
    deploymentToDelete,
    typedName,
    setTypedName,
    nameMatches,
    openDeleteDialog,
    closeDeleteDialog,
  } = useDeleteDeploymentDialog()
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

      <Dialog open={deploymentToDelete !== null} onOpenChange={open => !open && closeDeleteDialog()}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Delete deployment</DialogTitle>
            <DialogDescription>
              Type <span className="font-medium text-foreground">{deploymentToDelete?.name}</span> to confirm deletion.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="delete-deployment-name">Deployment name</Label>
            <Input
              id="delete-deployment-name"
              value={typedName}
              onChange={event => setTypedName(event.target.value)}
              placeholder={deploymentToDelete?.name ?? ''}
              autoComplete="off"
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeDeleteDialog} disabled={deleteMutation.isPending}>
              Cancel
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={!nameMatches || deleteMutation.isPending}
              onClick={() => {
                if (!deploymentToDelete) return
                deleteMutation.mutate(deploymentToDelete.id, { onSuccess: closeDeleteDialog })
              }}
            >
              Delete deployment
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <DeploymentTable
        deployments={deployments}
        isLoading={isLoading}
        isError={isError}
        isDeleting={deleteMutation.isPending}
        onDelete={openDeleteDialog}
        onEdit={openEditDialog}
        onCreate={openCreateDialog}
      />
    </>
  )
}
