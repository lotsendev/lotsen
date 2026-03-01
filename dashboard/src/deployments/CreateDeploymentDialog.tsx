import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../components/ui/dialog'
import CreateDeploymentForm from './CreateDeploymentForm'

type CreateDeploymentDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function CreateDeploymentDialog({ open, onOpenChange, onSuccess }: CreateDeploymentDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto border-border/60 sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>New deployment</DialogTitle>
          <DialogDescription>Create a new service from an image and runtime settings.</DialogDescription>
        </DialogHeader>
        <CreateDeploymentForm onSuccess={onSuccess} className="mb-0 border-0 shadow-none" hideHeader />
      </DialogContent>
    </Dialog>
  )
}
