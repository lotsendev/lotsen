import { Box } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import type { Deployment } from '../lib/api'
import { DeploymentRow } from './DeploymentRow'

type Props = {
  deployments: Deployment[] | undefined
  hasAnyDeployments: boolean
  hasActiveFilters: boolean
  isLoading: boolean
  isError: boolean
  isDeleting: boolean
  onDelete: (deployment: Deployment) => void
  onEdit: (deployment: Deployment) => void
  onCreate: () => void
  onClearFilters: () => void
  onRetry: () => void
}

export function DeploymentTable({
  deployments,
  hasAnyDeployments,
  hasActiveFilters,
  isLoading,
  isError,
  isDeleting,
  onDelete,
  onEdit,
  onCreate,
  onClearFilters,
  onRetry,
}: Props) {
  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }, (_, idx) => (
          <div key={idx} className="space-y-3 rounded-xl border border-border/60 bg-card px-4 py-4">
            <div className="h-4 w-40 animate-pulse rounded bg-muted/70" />
            <div className="h-3 w-3/4 animate-pulse rounded bg-muted/70" />
            <div className="h-8 w-64 animate-pulse rounded bg-muted/70" />
          </div>
        ))}
      </div>
    )
  }

  if (isError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Unable to load fleet state</CardTitle>
          <CardDescription>The deployment list is currently unreachable. Retry to fetch latest runtime data.</CardDescription>
        </CardHeader>
        <CardContent className="pt-0">
          <Button type="button" variant="outline" onClick={onRetry}>Retry now</Button>
        </CardContent>
      </Card>
    )
  }

  if (!deployments?.length && hasAnyDeployments && hasActiveFilters) {
    return (
      <Card>
        <CardHeader className="items-center text-center">
          <CardTitle>No deployments match your filters</CardTitle>
          <CardDescription>Adjust the search text or status lane to see matching services.</CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center gap-2 pt-0">
          <Button type="button" variant="outline" onClick={onClearFilters}>Clear filters</Button>
        </CardContent>
      </Card>
    )
  }

  if (!deployments?.length) {
    return (
      <Card>
        <CardHeader className="items-center text-center">
          <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <Box size={18} />
          </div>
          <CardTitle>Your workspace is ready</CardTitle>
          <CardDescription>
            You do not have any deployments yet. Create your first one to get your app running.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center pt-0">
          <Button type="button" onClick={onCreate}>Create first deployment</Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-3">
      {deployments.map(d => (
        <DeploymentRow
          key={d.id}
          deployment={d}
          onDelete={onDelete}
          isDeleting={isDeleting}
          onEdit={onEdit}
        />
      ))}
    </div>
  )
}
