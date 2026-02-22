import { Card, CardContent } from '../components/ui/card'
import type { UseMutationResult } from '@tanstack/react-query'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { Deployment } from '../lib/api'
import { DeploymentRow } from './DeploymentRow'

type Props = {
  deployments: Deployment[] | undefined
  isLoading: boolean
  isError: boolean
  deleteMutation: UseMutationResult<void, Error, string>
  onEdit: (deployment: Deployment) => void
}

export function DeploymentTable({ deployments, isLoading, isError, deleteMutation, onEdit }: Props) {
  if (isLoading) return <p className="text-sm text-muted-foreground">Loading deployments…</p>
  if (isError) return <p className="text-sm text-destructive">Failed to load deployments.</p>
  if (!deployments?.length) return <p className="text-sm text-muted-foreground">No deployments yet.</p>

  return (
    <Card className="overflow-hidden py-0">
      <CardContent className="p-0">
        <Table>
          <TableHeader className="bg-muted/40">
            <TableRow className="hover:bg-transparent">
              <TableHead>Name</TableHead>
              <TableHead>Image</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[110px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {deployments.map(d => (
              <DeploymentRow
                key={d.id}
                deployment={d}
                onDelete={id => deleteMutation.mutate(id)}
                isDeleting={deleteMutation.isPending}
                onEdit={onEdit}
              />
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
