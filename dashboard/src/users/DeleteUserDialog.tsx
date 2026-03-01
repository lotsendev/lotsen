import { AlertTriangle } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

type DeleteUserDialogProps = {
  username: string | null
  confirmation: string
  onConfirmationChange: (value: string) => void
  onClose: () => void
  onDelete: () => void
  canDelete: boolean
  isDeleting: boolean
  error: string | null
}

export function DeleteUserDialog({
  username,
  confirmation,
  onConfirmationChange,
  onClose,
  onDelete,
  canDelete,
  isDeleting,
  error,
}: DeleteUserDialogProps) {
  return (
    <Dialog open={username !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete operator</DialogTitle>
          <DialogDescription>
            {username ? `Type ${username} to confirm account removal.` : 'Confirm account removal.'}
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <p>Removing an operator immediately revokes dashboard access and cannot be undone.</p>
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="delete-user-confirm">Confirm username</Label>
          <Input id="delete-user-confirm" autoComplete="off" value={confirmation} onChange={event => onConfirmationChange(event.target.value)} />
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
          <Button type="button" variant="destructive" disabled={!canDelete || isDeleting || !username} onClick={onDelete}>
            {isDeleting ? 'Deleting...' : 'Delete operator'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
