import { useCreateDeploymentForm, type EnvRow, type PairRow } from './useCreateDeploymentForm'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { DynamicSection } from './DynamicSection'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

const fieldErrorCls = 'text-xs text-destructive'

export default function CreateDeploymentForm() {
  const {
    name, setName, image, setImage, domain, setDomain,
    envRows, portRows, volumeRows,
    errors, handleSubmit, isPending,
  } = useCreateDeploymentForm()

  return (
    <Card className="mb-8">
      <CardHeader>
        <CardTitle>New deployment</CardTitle>
        <CardDescription>Create a new service from an image and runtime settings.</CardDescription>
      </CardHeader>
      <CardContent>
      <form onSubmit={handleSubmit} noValidate className="space-y-5">

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="flex flex-col gap-1">
            <Label htmlFor="dep-name">Name *</Label>
            <Input id="dep-name" type="text" placeholder="my-app" value={name}
              onChange={e => setName(e.target.value)}
              aria-invalid={Boolean(errors.name)} />
            {errors.name && <p className={fieldErrorCls}>{errors.name}</p>}
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="dep-image">Image *</Label>
            <Input id="dep-image" type="text" placeholder="nginx:latest" value={image}
              onChange={e => setImage(e.target.value)}
              aria-invalid={Boolean(errors.image)} />
            {errors.image && <p className={fieldErrorCls}>{errors.image}</p>}
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <Label htmlFor="dep-domain">Domain (optional)</Label>
          <Input id="dep-domain" type="text" placeholder="app.example.com" value={domain}
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
        <Button type="submit" disabled={isPending}>
          {isPending ? 'Creating…' : 'Create'}
        </Button>
      </form>
      </CardContent>
    </Card>
  )
}
