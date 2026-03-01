import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { AlertTriangle, ArrowLeft, ChevronDown, ExternalLink, Globe, Hash, Lock, Package, Pencil, RotateCcw, Unlock } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../components/ui/dialog'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentLogsPanel } from '../deployments/DeploymentLogsPanel'
import { DeploymentSecurityPanel } from '../deployments/DeploymentSecurityPanel'
import { StatusBadge } from '../deployments/StatusBadge'
import { getDeployments, restartDeployment, type Deployment } from '../lib/api'

export function DeploymentDetailPage() {
  const { deploymentId } = useParams({ from: '/_app/deployments/$deploymentId' })
  const queryClient = useQueryClient()
  const { data: deployments, isLoading, isError } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
    refetchInterval: 30_000,
  })
  const [editOpen, setEditOpen] = useState(false)

  const restartMutation = useMutation({
    mutationFn: (id: string) => restartDeployment(id),
    onSuccess: updated => {
      queryClient.setQueryData<Deployment[]>(['deployments'], prev =>
        prev?.map(item => (item.id === updated.id ? updated : item))
      )
    },
  })

  const backLink = (
    <Link
      to="/deployments"
      className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="h-3.5 w-3.5" />
      Deployments
    </Link>
  )

  if (isLoading) {
    return (
      <div className="space-y-4">
        {backLink}
        <p className="text-sm text-muted-foreground">Loading deployment...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="space-y-4">
        {backLink}
        <p className="text-sm text-destructive">Failed to load deployment details.</p>
      </div>
    )
  }

  const deployment = deployments?.find(item => item.id === deploymentId)
  if (!deployment) {
    return (
      <div className="space-y-4">
        {backLink}
        <p className="text-sm text-muted-foreground">Deployment not found.</p>
      </div>
    )
  }

  const envEntries = Object.entries(deployment.envs)
  const stats = deployment.status === 'healthy' ? deployment.stats : undefined

  const handleRestart = () => {
    if (!window.confirm(`Restart ${deployment.name}? This will redeploy the current configuration.`)) {
      return
    }
    restartMutation.mutate(deployment.id)
  }

  return (
    <div className="space-y-4">
      {backLink}

      {/* Service identity */}
      <div className="rounded-xl border border-border/60 bg-card p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1.5">
            <div className="flex items-center gap-2.5">
              <h2 className="font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight">
                {deployment.name}
              </h2>
              <StatusBadge status={deployment.status} />
            </div>
            <div className="flex items-center gap-1.5">
              <Package className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />
              <span className="font-mono text-xs text-muted-foreground">{deployment.image}</span>
            </div>
          </div>
          <div className="flex flex-col gap-2 sm:items-end">
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 gap-1.5 px-2.5 text-xs"
                onClick={handleRestart}
                disabled={restartMutation.isPending || deployment.status === 'deploying'}
              >
                <RotateCcw className="h-3 w-3" />
                {restartMutation.isPending ? 'Restarting…' : 'Restart'}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 gap-1.5 px-2.5 text-xs"
                onClick={() => setEditOpen(true)}
              >
                <Pencil className="h-3 w-3" />
                Edit
              </Button>
            </div>
            {restartMutation.isError ? (
              <p className="text-xs text-destructive">Failed to restart deployment.</p>
            ) : null}
            <div className="flex flex-wrap items-center gap-2">
              {deployment.public ? (
                <Unlock className="h-3 w-3 shrink-0 text-[#2a7a64]" />
              ) : (
                <Lock className="h-3 w-3 shrink-0 text-primary" />
              )}
              <Badge variant={deployment.public ? 'success' : 'warning'}>
                {deployment.public ? 'Public' : 'Private'}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {deployment.public
                  ? 'Traffic goes directly to your app.'
                  : 'Protected by Lotsen login before app access.'}
              </span>
            </div>
            {deployment.domain && (
              <div className="flex items-center gap-1.5">
                <Globe className="h-3 w-3 shrink-0 text-muted-foreground/50" />
                <a
                  href={`https://${deployment.domain}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="group inline-flex items-center gap-1 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
                >
                  {deployment.domain}
                  <ExternalLink className="h-2.5 w-2.5 opacity-0 transition-opacity group-hover:opacity-60" />
                </a>
              </div>
            )}
            <div className="flex items-center gap-1.5">
              <Hash className="h-3 w-3 shrink-0 text-muted-foreground/30" />
              <span className="font-mono text-[11px] text-muted-foreground/40">{deployment.id}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Error alert */}
      {deployment.error && (
        <div className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-destructive">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="space-y-0.5">
            <p className="text-sm font-medium">Container exited with error</p>
            <p className="font-mono text-xs opacity-80">{deployment.error}</p>
          </div>
        </div>
      )}

      {stats && (
        <CollapsibleSection
          title="Resources"
          description="CPU and memory telemetry for healthy containers."
        >
          <div className="grid gap-3 md:grid-cols-2">
            <div className="rounded-md border border-border/60 bg-background/70 p-3">
              <div className="flex items-end justify-between gap-2">
                <p className="text-xs text-muted-foreground">CPU</p>
                <p className="font-mono text-sm text-foreground">{formatPercent(stats.cpuPercent)}</p>
              </div>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-muted/60">
                <div
                  className="h-full rounded-full bg-[#2a7a64] transition-[width] duration-300"
                  style={{ width: `${Math.min(stats.cpuPercent, 100)}%` }}
                />
              </div>
            </div>
            <div className="rounded-md border border-border/60 bg-background/70 p-3">
              <div className="flex items-end justify-between gap-2">
                <p className="text-xs text-muted-foreground">Memory</p>
                <p className="font-mono text-sm text-foreground">
                  {formatBytes(stats.memoryUsedBytes)} / {formatBytes(stats.memoryLimitBytes)}
                </p>
              </div>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-muted/60">
                <div
                  className="h-full rounded-full bg-[#1a96e0] transition-[width] duration-300"
                  style={{ width: `${Math.min(stats.memoryPercent, 100)}%` }}
                />
              </div>
              <p className="mt-2 font-mono text-xs text-muted-foreground">{formatPercent(stats.memoryPercent)}</p>
            </div>
          </div>
        </CollapsibleSection>
      )}

      {/* Ports and volumes */}
      <CollapsibleSection
        title="Ports and volumes"
        description="Runtime bindings for traffic and persisted data."
      >
        <div className="grid gap-3 md:grid-cols-2">
          <div className="rounded-md border border-border/60 bg-background/70 p-3">
            <p className="mb-2.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
              Ports
            </p>
            {deployment.ports.length ? (
              <ul className="space-y-1">
                {deployment.ports.map(port => (
                  <li
                    key={port}
                    className="rounded-md bg-background/70 px-2.5 py-1.5 font-mono text-xs text-muted-foreground"
                  >
                    {port}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground/50">None configured</p>
            )}
          </div>
          <div className="rounded-md border border-border/60 bg-background/70 p-3">
            <p className="mb-2.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
              Volumes
            </p>
            {deployment.volumes.length ? (
              <ul className="space-y-1">
                {deployment.volumes.map(volume => (
                  <li
                    key={volume}
                    className="rounded-md bg-background/70 px-2.5 py-1.5 font-mono text-xs text-muted-foreground"
                  >
                    {volume}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground/50">None configured</p>
            )}
          </div>
        </div>
      </CollapsibleSection>

      {/* Environment variables */}
      <CollapsibleSection
        title="Environment variables"
        description={`${envEntries.length} configured value${envEntries.length === 1 ? '' : 's'}.`}
      >
        {envEntries.length ? (
          <div className="space-y-1">
            {envEntries.map(([key, value]) => (
              <div
                key={key}
                className="grid grid-cols-[minmax(120px,200px)_1fr] gap-3 rounded-md bg-background/70 px-2.5 py-1.5"
              >
                <span className="font-mono text-xs text-foreground/80">{key}</span>
                <span className="break-all font-mono text-xs text-muted-foreground">{value || '(empty)'}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs text-muted-foreground/50">None configured</p>
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="Traffic and security"
        description="Ingress access mode and runtime protection controls."
      >
        <DeploymentSecurityPanel deployment={deployment} />
      </CollapsibleSection>

      <CollapsibleSection
        title="Live logs"
        description="Real-time output stream from the active container runtime."
        defaultOpen={deployment.status === 'failed'}
      >
        <DeploymentLogsPanel deploymentId={deployment.id} status={deployment.status} error={deployment.error} />
      </CollapsibleSection>

      {/* Edit dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto border-border/60 sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>Edit deployment</DialogTitle>
            <DialogDescription>
              Update runtime settings for <span className="font-medium text-foreground">{deployment.name}</span>.
            </DialogDescription>
          </DialogHeader>
          <EditDeploymentForm
            key={deployment.id}
            deployment={deployment}
            onClose={() => setEditOpen(false)}
            className="mb-0 border-0 shadow-none"
            hideHeader
          />
        </DialogContent>
      </Dialog>

      <CollapsibleSection
        title="Store snapshot"
        description="Raw deployment document from the persisted state store."
      >
        <pre className="overflow-x-auto rounded-lg border border-border/40 bg-background/70 p-4 font-mono text-xs leading-5 text-foreground/80">
          {JSON.stringify(deployment, null, 2)}
        </pre>
      </CollapsibleSection>
    </div>
  )
}

function CollapsibleSection({
  title,
  description,
  defaultOpen = false,
  children,
}: {
  title: string
  description: string
  defaultOpen?: boolean
  children: ReactNode
}) {
  return (
    <details open={defaultOpen} className="group rounded-xl border border-border/60 bg-card p-4">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">{title}</p>
          <p className="mt-1 text-xs text-muted-foreground">{description}</p>
        </div>
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground/40 transition-transform duration-200 group-open:rotate-180" />
      </summary>
      <div className="mt-3">{children}</div>
    </details>
  )
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) {
    return '0.0%'
  }
  return `${value.toFixed(1)}%`
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }

  if (value >= 100 || unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`
  }

  return `${value.toFixed(1)} ${units[unitIndex]}`
}
