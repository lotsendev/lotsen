import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

type DeleteDeploymentDialogProps = {
  deploymentName: string | null
  typedName: string
  setTypedName: (value: string) => void
  isPending: boolean
  nameMatches: boolean
  onClose: () => void
  onConfirm: () => void
}

export function DeleteDeploymentDialog({
  deploymentName,
  typedName,
  setTypedName,
  isPending,
  nameMatches,
  onClose,
  onConfirm,
}: DeleteDeploymentDialogProps) {
  return (
    <Dialog open={deploymentName !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Delete deployment</DialogTitle>
          <DialogDescription>
            Type <span className="font-medium text-foreground">{deploymentName}</span> to confirm deletion.
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
          This action removes the deployment from orchestration and cannot be undone.
        </div>

        <div className="space-y-2">
          <Label htmlFor="delete-deployment-name">Deployment name</Label>
          <Input
            id="delete-deployment-name"
            value={typedName}
            onChange={event => setTypedName(event.target.value)}
            placeholder={deploymentName ?? ''}
            autoComplete="off"
          />
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>Cancel</Button>
          <Button type="button" variant="destructive" disabled={!nameMatches || isPending} onClick={onConfirm}>
            Delete deployment
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
