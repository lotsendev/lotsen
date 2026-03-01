import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

type PasswordResetDialogProps = {
  username: string | null
  value: string
  onValueChange: (value: string) => void
  onClose: () => void
  onSubmit: () => void
  isSubmitting: boolean
  error: string | null
}

export function PasswordResetDialog({ username, value, onValueChange, onClose, onSubmit, isSubmitting, error }: PasswordResetDialogProps) {
  return (
    <Dialog open={username !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>
            {username ? `Set a new password for ${username}.` : 'Set a new password.'}
          </DialogDescription>
        </DialogHeader>

        <form
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault()
            onSubmit()
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="reset-password">New password</Label>
            <Input id="reset-password" type="password" autoComplete="new-password" value={value} onChange={event => onValueChange(event.target.value)} required />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={isSubmitting || !username}>{isSubmitting ? 'Updating...' : 'Update password'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
