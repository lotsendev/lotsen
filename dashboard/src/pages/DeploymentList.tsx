import { CreateDeploymentDialog } from '../deployments/CreateDeploymentDialog'
import { DeleteDeploymentDialog } from '../deployments/DeleteDeploymentDialog'
import { DeploymentListControlPanel } from '../deployments/DeploymentListControlPanel'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeleteDeploymentDialog } from '../deployments/useDeleteDeploymentDialog'
import { useDeploymentDialogs } from '../deployments/useDeploymentDialogs'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentListFilters } from '../deployments/useDeploymentListFilters'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'

export default function DeploymentList() {
  useDeploymentSSE()
  const { deployments, isLoading, isError, deleteMutation, refetch } = useDeploymentList()
  const {
    search,
    setSearch,
    statusFilter,
    setStatusFilter,
    statusCounts,
    filteredDeployments,
    hasActiveFilters,
    clearFilters,
  } = useDeploymentListFilters(deployments)
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
  } = useDeploymentDialogs()

  return (
    <>
      <div className="mb-6 space-y-4">
        <DeploymentListControlPanel
          statusCounts={statusCounts}
          search={search}
          statusFilter={statusFilter}
          onSearchChange={setSearch}
          onStatusFilterChange={setStatusFilter}
          onCreate={openCreateDialog}
        />
      </div>

      <CreateDeploymentDialog open={createDialogOpen} onOpenChange={setCreateDialogOpen} onSuccess={closeCreateDialog} />

      <DeleteDeploymentDialog
        deploymentName={deploymentToDelete?.name ?? null}
        typedName={typedName}
        setTypedName={setTypedName}
        isPending={deleteMutation.isPending}
        nameMatches={nameMatches}
        onClose={closeDeleteDialog}
        onConfirm={() => {
          if (!deploymentToDelete) return
          deleteMutation.mutate(deploymentToDelete.id, { onSuccess: closeDeleteDialog })
        }}
      />

      <DeploymentTable
        deployments={filteredDeployments}
        hasAnyDeployments={(deployments?.length ?? 0) > 0}
        hasActiveFilters={hasActiveFilters}
        isLoading={isLoading}
        isError={isError}
        isDeleting={deleteMutation.isPending}
        onDelete={openDeleteDialog}
        onCreate={openCreateDialog}
        onClearFilters={clearFilters}
        onRetry={() => {
          void refetch()
        }}
      />
    </>
  )
}
