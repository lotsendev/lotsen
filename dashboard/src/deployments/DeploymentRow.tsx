import { ArrowUpRight, ExternalLink, Globe, Pencil, Trash2 } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import type { Deployment } from '../lib/api'
import { StatusBadge } from './StatusBadge'

type Props = {
  deployment: Deployment
  onDelete: (deployment: Deployment) => void
  isDeleting: boolean
  onEdit: (deployment: Deployment) => void
}

export function DeploymentRow({ deployment: d, onDelete, isDeleting, onEdit }: Props) {
  const detailsLabel = d.status === 'failed' ? 'Investigate' : 'Details'
  const hasDomain = Boolean(d.domain)
  const hasPorts = d.ports.length > 0
  const hasVolumes = d.volumes.length > 0

  return (
    <article className="rounded-xl border border-border/60 bg-card px-4 py-4 transition-colors hover:bg-muted/20">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Link
              to="/deployments/$deploymentId"
              params={{ deploymentId: d.id }}
              className="inline-flex min-w-0 items-center gap-1.5 text-base font-semibold text-foreground transition-colors hover:text-primary"
            >
              <span className="truncate">{d.name}</span>
              <ArrowUpRight size={14} className="text-muted-foreground" />
            </Link>
            <StatusBadge status={d.status} />
          </div>

          <div className="grid gap-2 text-sm sm:grid-cols-2 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
            <div className="min-w-0">
              <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Image</p>
              <p className="truncate pt-1 font-mono text-xs text-muted-foreground">{d.image}</p>
            </div>

            <div className="min-w-0">
              <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Route</p>
              <div className="pt-1">
                {hasDomain ? (
                  <Badge variant="outline" className="max-w-full justify-start gap-1 border-border/70 bg-background/70">
                    <Globe size={12} className="shrink-0" />
                    <span className="truncate font-mono text-[11px]">{d.domain}</span>
                  </Badge>
                ) : (
                  <p className="text-xs text-muted-foreground">No domain configured</p>
                )}
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary" className="px-2 py-0.5 text-[11px]">
              {hasPorts ? `${d.ports.length} port${d.ports.length > 1 ? 's' : ''}` : 'No ports'}
            </Badge>
            <Badge variant="secondary" className="px-2 py-0.5 text-[11px]">
              {hasVolumes ? `${d.volumes.length} volume${d.volumes.length > 1 ? 's' : ''}` : 'No volumes'}
            </Badge>
            <Badge variant="secondary" className="px-2 py-0.5 text-[11px]">
              {Object.keys(d.envs).length} env var{Object.keys(d.envs).length === 1 ? '' : 's'}
            </Badge>
          </div>
        </div>

        <div className="flex items-center gap-1 self-end lg:self-start">
          <Button asChild variant="ghost" size="sm" className="h-8 gap-1.5 px-2.5 text-xs">
            <Link to="/deployments/$deploymentId" params={{ deploymentId: d.id }}>
              {detailsLabel}
              <ExternalLink size={13} />
            </Link>
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onEdit(d)}
            aria-label={`Edit ${d.name}`}
            className="h-8 w-8 text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            <Pencil size={15} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onDelete(d)}
            disabled={isDeleting}
            aria-label={`Delete ${d.name}`}
            className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
          >
            <Trash2 size={15} />
          </Button>
        </div>
      </div>
    </article>
  )
}
