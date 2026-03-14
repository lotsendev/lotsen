import { Check, Globe2, Server } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { cn } from '../lib/utils'
import { DynamicSection } from './DynamicSection'
import { useCreateDeploymentForm, type BasicAuthUserRow, type EnvRow, type FileMountRow, type PortRow, type VolumeMountRow } from './useCreateDeploymentForm'

const fieldErrorCls = 'text-xs text-destructive'

type Props = {
  onSuccess?: () => void
  className?: string
  hideHeader?: boolean
}

export default function CreateDeploymentForm({ onSuccess, className, hideHeader = false }: Props) {
  const {
    name, setName, image, setImage, domain, setDomain,
    isPublic, setIsPublic,
    selectedProxyRowId, setSelectedProxyRowId,
    envRows, portRows, volumeRows, fileRows, basicAuthEnabled, setBasicAuthEnabled, basicAuthRows,
    errors, handleSubmit, isPending,
  } = useCreateDeploymentForm({ onSuccess })

  return (
    <Card className={cn('mb-8', className)}>
      {!hideHeader && (
        <CardHeader>
          <CardTitle>New deployment</CardTitle>
          <CardDescription>Create a new service from an image and runtime settings.</CardDescription>
        </CardHeader>
      )}

      <CardContent className={hideHeader ? 'pt-5' : undefined}>
        <form onSubmit={handleSubmit} noValidate className="space-y-5">
          <section className="grid gap-2 rounded-lg border border-border/60 bg-background/60 p-3 text-xs text-muted-foreground sm:grid-cols-3 sm:p-4">
            <div className="flex items-center gap-2">
              <Server className="h-3.5 w-3.5" />
              <span>Image + runtime</span>
            </div>
            <div className="flex items-center gap-2">
              <Globe2 className="h-3.5 w-3.5" />
              <span>Route + exposure</span>
            </div>
            <div className="flex items-center gap-2">
              <Check className="h-3.5 w-3.5" />
              <span>Ready for deploy</span>
            </div>
          </section>

          <section className="space-y-3 rounded-lg border border-border/60 bg-background/60 p-3 sm:p-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-semibold text-foreground">Core identity</p>
              <Badge variant="outline" className="h-5 px-1.5 text-[10px]">required</Badge>
            </div>

            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="dep-name">Name *</Label>
                <Input
                  id="dep-name"
                  type="text"
                  placeholder="my-app"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  aria-invalid={Boolean(errors.name)}
                />
                {errors.name && <p className={fieldErrorCls}>{errors.name}</p>}
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="dep-image">Image *</Label>
                <Input
                  id="dep-image"
                  type="text"
                  placeholder="nginx:latest"
                  value={image}
                  onChange={e => setImage(e.target.value)}
                  aria-invalid={Boolean(errors.image)}
                />
                {errors.image && <p className={fieldErrorCls}>{errors.image}</p>}
              </div>
            </div>
          </section>

          <section className="space-y-3 rounded-lg border border-border/60 bg-background/60 p-3 sm:p-4">
            <p className="text-sm font-semibold text-foreground">Ingress</p>
            <div className="flex flex-col gap-1">
              <Label htmlFor="dep-domain">Domain (optional)</Label>
              <Input
                id="dep-domain"
                type="text"
                placeholder="app.example.com"
                value={domain}
                onChange={e => setDomain(e.target.value)}
              />
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border/60 bg-background/70 p-3">
              <div>
                <p className="text-sm font-medium text-foreground">Publicly accessible</p>
                <p className="text-xs text-muted-foreground">When enabled, requests go directly to your app without Lotsen login.</p>
              </div>
              <label className="inline-flex cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  role="switch"
                  aria-label="Public deployment"
                  checked={isPublic}
                  onChange={e => setIsPublic(e.target.checked)}
                  className="peer sr-only"
                />
                <span
                  className={cn(
                    'relative h-6 w-11 rounded-full border transition-colors',
                    isPublic
                      ? 'border-primary/40 bg-primary/20'
                      : 'border-border bg-background'
                  )}
                >
                  <span
                    className={cn(
                      'absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-foreground transition-transform',
                      isPublic ? 'translate-x-5' : 'translate-x-0'
                    )}
                  />
                </span>
                <span className="text-xs font-medium text-foreground">{isPublic ? 'Public' : 'Private'}</span>
              </label>
            </div>
          </section>

          <DynamicSection<EnvRow>
            title="Environment variables"
            addLabel="Add env var"
            removeLabel="Remove env var"
            rows={envRows.rows}
            onAdd={envRows.add}
            onRemove={envRows.remove}
            errorFor={row => errors.envs[row.id]}
            renderRow={row => (
              <>
                <Input
                  type="text"
                  placeholder="KEY"
                  value={row.key}
                  onChange={e => envRows.update(row.id, { key: e.target.value })}
                  aria-invalid={Boolean(errors.envs[row.id])}
                  className="font-mono"
                />
                <Input
                  type="text"
                  placeholder="value"
                  value={row.value}
                  onChange={e => envRows.update(row.id, { value: e.target.value })}
                />
              </>
            )}
          />

          <DynamicSection<PortRow>
            title="Port mappings"
            description="Use container-only ports (for example 80) for auto host assignment, or explicit mappings like 53:53 and 53:53/udp. When domain is set, choose one TCP row as proxy target."
            addLabel="Add port mapping"
            removeLabel="Remove port mapping"
            rows={portRows.rows}
            onAdd={portRows.add}
            onRemove={portRows.remove}
            errorFor={row => errors.ports[row.id]}
            renderRow={row => (
              <>
                <Input
                  type="text"
                  placeholder="80 or 53:53/udp"
                  value={row.port}
                  onChange={e => portRows.update(row.id, { port: e.target.value })}
                  aria-invalid={Boolean(errors.ports[row.id])}
                  className="flex-1"
                />
                <label className="inline-flex shrink-0 items-center gap-2 rounded-md border border-border/60 px-2 py-1 text-xs text-foreground">
                  <input
                    type="checkbox"
                    checked={selectedProxyRowId === row.id}
                    disabled={!domain.trim()}
                    onChange={e => setSelectedProxyRowId(e.target.checked ? row.id : null)}
                  />
                  Proxy target
                </label>
              </>
            )}
          />

          <DynamicSection<VolumeMountRow>
            title="Volume mounts"
            description="Managed volumes are created under Lotsen's data directory and persist automatically. Bind mounts map directly to an absolute VPS path for advanced setups."
            addLabel="Add volume mount"
            removeLabel="Remove volume mount"
            rows={volumeRows.rows}
            onAdd={volumeRows.add}
            onRemove={volumeRows.remove}
            errorFor={row => errors.volumes[row.id]}
            renderRow={row => (
              <>
                <select
                  value={row.mode}
                  onChange={e => volumeRows.update(row.id, { mode: e.target.value as VolumeMountRow['mode'] })}
                  className="h-9 shrink-0 rounded-md border border-input bg-background px-2 text-sm"
                  aria-label="Volume mode"
                >
                  <option value="managed">Managed</option>
                  <option value="bind">Bind mount</option>
                </select>
                <Input
                  type="text"
                  placeholder={row.mode === 'managed' ? 'postgres-data' : '/host/path'}
                  value={row.source}
                  onChange={e => volumeRows.update(row.id, { source: e.target.value })}
                  aria-invalid={Boolean(errors.volumes[row.id])}
                  className="font-mono"
                />
                <span className="shrink-0 text-sm text-muted-foreground">:</span>
                <Input
                  type="text"
                  placeholder="/container/path"
                  value={row.target}
                  onChange={e => volumeRows.update(row.id, { target: e.target.value })}
                  aria-invalid={Boolean(errors.volumes[row.id])}
                  className="font-mono"
                />
              </>
            )}
          />

          <DynamicSection<FileMountRow>
            title="Config files"
            description="Create text config files on the VPS and mount them into the container. Useful for Prometheus, Nginx, and other services that require config files."
            addLabel="Add config file"
            removeLabel="Remove config file"
            rows={fileRows.rows}
            onAdd={fileRows.add}
            onRemove={fileRows.remove}
            errorFor={row => errors.files[row.id]}
            renderRow={row => (
              <div className="grid flex-1 gap-2">
                <div className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_auto_1fr_auto] md:items-center">
                  <Input
                    type="text"
                    placeholder="prometheus.yml"
                    value={row.source}
                    onChange={e => fileRows.update(row.id, { source: e.target.value })}
                    aria-invalid={Boolean(errors.files[row.id])}
                    className="font-mono"
                  />
                  <span className="hidden text-sm text-muted-foreground md:inline">-&gt;</span>
                  <Input
                    type="text"
                    placeholder="/etc/prometheus/prometheus.yml"
                    value={row.target}
                    onChange={e => fileRows.update(row.id, { target: e.target.value })}
                    aria-invalid={Boolean(errors.files[row.id])}
                    className="font-mono"
                  />
                  <label className="inline-flex items-center gap-2 rounded-md border border-border/60 px-2 py-1 text-xs text-foreground">
                    <input
                      type="checkbox"
                      checked={row.readOnly}
                      onChange={e => fileRows.update(row.id, { readOnly: e.target.checked })}
                    />
                    Read only
                  </label>
                </div>
                <textarea
                  value={row.content}
                  onChange={e => fileRows.update(row.id, { content: e.target.value })}
                  placeholder="Paste file content here"
                  className="min-h-28 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
                />
              </div>
            )}
          />

          <section className="space-y-3 rounded-lg border border-border/60 bg-background/60 p-3 sm:p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-foreground">Access control</p>
                <p className="text-xs text-muted-foreground">Optional HTTP basic auth at the proxy edge.</p>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={basicAuthEnabled}
                onClick={() => setBasicAuthEnabled(!basicAuthEnabled)}
                className={cn(
                  'inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-medium transition-colors',
                  basicAuthEnabled
                    ? 'border-primary/30 bg-primary/10 text-foreground'
                    : 'border-border bg-background text-muted-foreground'
                )}
              >
                {basicAuthEnabled ? 'Enabled' : 'Disabled'}
              </button>
            </div>

            {basicAuthEnabled && (
              <DynamicSection<BasicAuthUserRow>
                title="Basic auth users"
                addLabel="Add user"
                removeLabel="Remove user"
                rows={basicAuthRows.rows}
                onAdd={basicAuthRows.add}
                onRemove={basicAuthRows.remove}
                errorFor={row => errors.basicAuth[row.id]}
                renderRow={row => (
                  <>
                    <Input
                      type="text"
                      placeholder="Username"
                      value={row.username}
                      onChange={e => basicAuthRows.update(row.id, { username: e.target.value })}
                      aria-invalid={Boolean(errors.basicAuth[row.id])}
                    />
                    <Input
                      type="password"
                      placeholder="Password"
                      value={row.password}
                      onChange={e => basicAuthRows.update(row.id, { password: e.target.value })}
                      aria-invalid={Boolean(errors.basicAuth[row.id])}
                    />
                  </>
                )}
              />
            )}
          </section>

          {errors.form && <p className={fieldErrorCls}>{errors.form}</p>}

          <div className="flex items-center justify-end">
            <Button type="submit" disabled={isPending}>
              {isPending ? 'Creating…' : 'Create'}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
