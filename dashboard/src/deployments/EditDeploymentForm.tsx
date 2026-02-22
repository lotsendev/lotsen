import type { Deployment } from '../lib/api'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { useEditDeploymentForm } from './useEditDeploymentForm'
import { DynamicSection } from './DynamicSection'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { cn } from '../lib/utils'
import type { EnvRow, PairRow } from './useCreateDeploymentForm'

const fieldErrorCls = 'text-xs text-destructive'

type Props = {
  deployment: Deployment
  onClose: () => void
  className?: string
  hideHeader?: boolean
}

export default function EditDeploymentForm({ deployment, onClose, className, hideHeader = false }: Props) {
  const {
    name, setName, image, setImage, domain, setDomain,
    envRows, portRows, volumeRows,
    errors, handleSubmit, isPending,
  } = useEditDeploymentForm(deployment, onClose)

  return (
    <Card className={cn('mb-8', className)}>
      {!hideHeader && (
        <CardHeader>
          <CardTitle>Edit deployment</CardTitle>
          <CardDescription>Update runtime settings for {deployment.name}.</CardDescription>
        </CardHeader>
      )}
      <CardContent className={hideHeader ? 'pt-5' : undefined}>
      <form onSubmit={handleSubmit} noValidate className="space-y-5">

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="flex flex-col gap-1">
            <Label htmlFor="edit-dep-name">Name *</Label>
            <Input id="edit-dep-name" type="text" placeholder="my-app" value={name}
              onChange={e => setName(e.target.value)}
              aria-invalid={Boolean(errors.name)} />
            {errors.name && <p className={fieldErrorCls}>{errors.name}</p>}
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="edit-dep-image">Image *</Label>
            <Input id="edit-dep-image" type="text" placeholder="nginx:latest" value={image}
              onChange={e => setImage(e.target.value)}
              aria-invalid={Boolean(errors.image)} />
            {errors.image && <p className={fieldErrorCls}>{errors.image}</p>}
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <Label htmlFor="edit-dep-domain">Domain (optional)</Label>
          <Input id="edit-dep-domain" type="text" placeholder="app.example.com" value={domain}
            onChange={e => setDomain(e.target.value)} />
        </div>

        <DynamicSection<EnvRow>
          title="Environment variables" addLabel="Add env var" removeLabel="Remove env var"
          rows={envRows.rows} onAdd={envRows.add} onRemove={envRows.remove}
          errorFor={row => errors.envs[row.id]}
          renderRow={row => (<>
            <Input type="text" placeholder="KEY" value={row.key}
              onChange={e => envRows.update(row.id, { key: e.target.value })}
              aria-invalid={Boolean(errors.envs[row.id])}
              className="font-mono" />
            <Input type="text" placeholder="value" value={row.value}
              onChange={e => envRows.update(row.id, { value: e.target.value })}
            />
          </>)}
        />

        <DynamicSection<PairRow>
          title="Port mappings" addLabel="Add port mapping" removeLabel="Remove port mapping"
          rows={portRows.rows} onAdd={portRows.add} onRemove={portRows.remove}
          errorFor={row => errors.ports[row.id]}
          renderRow={row => (<>
            <Input type="text" placeholder="Host port" value={row.left}
              onChange={e => portRows.update(row.id, { left: e.target.value })}
              aria-invalid={Boolean(errors.ports[row.id])} />
            <span className="shrink-0 text-sm text-muted-foreground">:</span>
            <Input type="text" placeholder="Container port" value={row.right}
              onChange={e => portRows.update(row.id, { right: e.target.value })}
              aria-invalid={Boolean(errors.ports[row.id])} />
          </>)}
        />

        <DynamicSection<PairRow>
          title="Volume mounts" addLabel="Add volume mount" removeLabel="Remove volume mount"
          rows={volumeRows.rows} onAdd={volumeRows.add} onRemove={volumeRows.remove}
          errorFor={row => errors.volumes[row.id]}
          renderRow={row => (<>
            <Input type="text" placeholder="/host/path" value={row.left}
              onChange={e => volumeRows.update(row.id, { left: e.target.value })}
              aria-invalid={Boolean(errors.volumes[row.id])}
              className="font-mono" />
            <span className="shrink-0 text-sm text-muted-foreground">:</span>
            <Input type="text" placeholder="/container/path" value={row.right}
              onChange={e => volumeRows.update(row.id, { right: e.target.value })}
              aria-invalid={Boolean(errors.volumes[row.id])}
              className="font-mono" />
          </>)}
        />

        {errors.form && <p className={fieldErrorCls}>{errors.form}</p>}
        <div className="flex items-center gap-3">
          <Button type="submit" disabled={isPending}>
            {isPending ? 'Saving…' : 'Save'}
          </Button>
          <Button type="button" onClick={onClose} disabled={isPending} variant="outline">
            Cancel
          </Button>
        </div>
      </form>
      </CardContent>
    </Card>
  )
}
