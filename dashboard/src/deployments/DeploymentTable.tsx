import { Box } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '../components/ui/table'
import type { Deployment } from '../lib/api'
import { DeploymentRow } from './DeploymentRow'

type Props = {
  deployments: Deployment[] | undefined
  isLoading: boolean
  isError: boolean
  isDeleting: boolean
  onDelete: (deployment: Deployment) => void
  onEdit: (deployment: Deployment) => void
  onCreate: () => void
}

export function DeploymentTable({ deployments, isLoading, isError, isDeleting, onDelete, onEdit, onCreate }: Props) {
  if (isLoading) return <p className="text-sm text-muted-foreground">Loading deployments…</p>
  if (isError) return <p className="text-sm text-destructive">Failed to load deployments.</p>
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
    <Card className="overflow-hidden py-0">
      <CardContent className="p-0">
        <Table>
          <TableHeader className="bg-muted/40">
            <TableRow className="hover:bg-transparent">
              <TableHead>Name</TableHead>
              <TableHead>Image</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[190px] text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {deployments.map(d => (
              <DeploymentRow
                key={d.id}
                deployment={d}
                onDelete={onDelete}
                isDeleting={isDeleting}
                onEdit={onEdit}
              />
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
