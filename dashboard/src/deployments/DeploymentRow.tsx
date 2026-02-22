import { Pencil, Trash2 } from 'lucide-react'
import { Button } from '../components/ui/button'
import { TableCell, TableRow } from '../components/ui/table'
import type { Deployment } from '../lib/api'
import { StatusBadge } from './StatusBadge'

type Props = {
  deployment: Deployment
  onDelete: (id: string) => void
  isDeleting: boolean
  onEdit: (deployment: Deployment) => void
}

export function DeploymentRow({ deployment: d, onDelete, isDeleting, onEdit }: Props) {
  return (
    <TableRow className="bg-card">
      <TableCell className="py-3 font-medium text-foreground">{d.name}</TableCell>
      <TableCell className="py-3 font-mono text-xs text-muted-foreground">{d.image}</TableCell>
      <TableCell className="py-3">
        <StatusBadge status={d.status} error={d.error} />
      </TableCell>
      <TableCell className="py-3 text-right">
        <div className="flex items-center justify-end gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onEdit(d)}
            aria-label={`Edit ${d.name}`}
            className="h-7 w-7 text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            <Pencil size={15} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onDelete(d.id)}
            disabled={isDeleting}
            aria-label={`Delete ${d.name}`}
            className="h-7 w-7 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
          >
            <Trash2 size={15} />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
