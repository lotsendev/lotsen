import { useMemo, useState } from 'react'
import { Plus, Search } from 'lucide-react'
import { Badge } from '../components/ui/badge'
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
  const { deployments, isLoading, isError, deleteMutation, refetch } = useDeploymentList()
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'failed' | 'deploying' | 'healthy' | 'idle'>('all')
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

  const statusCounts = useMemo(() => {
    const summary = {
      total: deployments?.length ?? 0,
      healthy: 0,
      deploying: 0,
      failed: 0,
      idle: 0,
    }

    for (const deployment of deployments ?? []) {
      summary[deployment.status] += 1
    }

    return summary
  }, [deployments])

  const normalizedSearch = search.trim().toLowerCase()

  const filteredDeployments = useMemo(() => {
    const source = deployments ?? []
    const statusPriority: Record<'failed' | 'deploying' | 'healthy' | 'idle', number> = {
      failed: 0,
      deploying: 1,
      healthy: 2,
      idle: 3,
    }

    return source
      .filter(deployment => {
        if (statusFilter !== 'all' && deployment.status !== statusFilter) {
          return false
        }

        if (!normalizedSearch) {
          return true
        }

        const haystack = `${deployment.name} ${deployment.image} ${deployment.domain}`.toLowerCase()
        return haystack.includes(normalizedSearch)
      })
      .sort((a, b) => {
        const byStatus = statusPriority[a.status] - statusPriority[b.status]
        if (byStatus !== 0) {
          return byStatus
        }
        return a.name.localeCompare(b.name)
      })
  }, [deployments, normalizedSearch, statusFilter])

  const hasActiveFilters = statusFilter !== 'all' || normalizedSearch.length > 0

  return (
    <>
      <div className="mb-6 space-y-4">
        <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Fleet control board</p>
              <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Deployments in this workspace</h2>
            </div>
            <Button type="button" onClick={openCreateDialog}>
              <Plus size={16} />
              Create deployment
            </Button>
          </div>

          <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-5">
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Total</p>
              <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts.total}</p>
            </article>
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Healthy</p>
              <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts.healthy}</p>
            </article>
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Deploying</p>
              <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts.deploying}</p>
            </article>
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Failed</p>
              <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts.failed}</p>
            </article>
            <article className="rounded-lg border border-border/60 bg-background/70 p-3">
              <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Idle</p>
              <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts.idle}</p>
            </article>
          </div>

          <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <label className="space-y-1.5">
              <span className="text-xs text-muted-foreground">Search by name, image, or domain</span>
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={search}
                  onChange={event => setSearch(event.target.value)}
                  placeholder="api, postgres, ghcr.io/org/image"
                  className="pl-9"
                  autoComplete="off"
                />
              </div>
            </label>

            <div className="flex flex-wrap items-center gap-2">
              {([
                ['all', 'All', statusCounts.total],
                ['failed', 'Failed', statusCounts.failed],
                ['deploying', 'Deploying', statusCounts.deploying],
                ['healthy', 'Healthy', statusCounts.healthy],
                ['idle', 'Idle', statusCounts.idle],
              ] as const).map(([value, label, count]) => {
                const isActive = statusFilter === value
                return (
                  <Button
                    key={value}
                    type="button"
                    size="sm"
                    variant={isActive ? 'default' : 'outline'}
                    className="h-8 gap-1.5 px-2.5"
                    onClick={() => setStatusFilter(value)}
                  >
                    {label}
                    <Badge variant={isActive ? 'secondary' : 'outline'} className="h-4 rounded-sm px-1.5 text-[10px]">
                      {count}
                    </Badge>
                  </Button>
                )
              })}
            </div>
          </div>
        </section>
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
        deployments={filteredDeployments}
        hasAnyDeployments={(deployments?.length ?? 0) > 0}
        hasActiveFilters={hasActiveFilters}
        isLoading={isLoading}
        isError={isError}
        isDeleting={deleteMutation.isPending}
        onDelete={openDeleteDialog}
        onEdit={openEditDialog}
        onCreate={openCreateDialog}
        onClearFilters={() => {
          setSearch('')
          setStatusFilter('all')
        }}
        onRetry={() => {
          void refetch()
        }}
      />
    </>
  )
}
