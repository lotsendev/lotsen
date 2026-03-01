import { Plus, Search } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import type { DeploymentStatusFilter } from './useDeploymentListFilters'

type StatusCounts = {
  total: number
  healthy: number
  deploying: number
  failed: number
  idle: number
}

type DeploymentListControlPanelProps = {
  statusCounts: StatusCounts
  search: string
  statusFilter: DeploymentStatusFilter
  onSearchChange: (value: string) => void
  onStatusFilterChange: (value: DeploymentStatusFilter) => void
  onCreate: () => void
}

const STATUS_FILTERS: Array<{ value: DeploymentStatusFilter; label: string; countKey: keyof StatusCounts }> = [
  { value: 'all', label: 'All', countKey: 'total' },
  { value: 'failed', label: 'Failed', countKey: 'failed' },
  { value: 'deploying', label: 'Deploying', countKey: 'deploying' },
  { value: 'healthy', label: 'Healthy', countKey: 'healthy' },
  { value: 'idle', label: 'Idle', countKey: 'idle' },
]

const FLEET_TILES: Array<{ label: string; countKey: keyof StatusCounts }> = [
  { label: 'Total', countKey: 'total' },
  { label: 'Healthy', countKey: 'healthy' },
  { label: 'Deploying', countKey: 'deploying' },
  { label: 'Failed', countKey: 'failed' },
  { label: 'Idle', countKey: 'idle' },
]

export function DeploymentListControlPanel({
  statusCounts,
  search,
  statusFilter,
  onSearchChange,
  onStatusFilterChange,
  onCreate,
}: DeploymentListControlPanelProps) {
  return (
    <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Fleet control board</p>
          <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Deployments in this workspace</h2>
        </div>
        <Button type="button" onClick={onCreate}>
          <Plus size={16} />
          Create deployment
        </Button>
      </div>

      <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-5">
        {FLEET_TILES.map(tile => (
          <article key={tile.label} className="rounded-lg border border-border/60 bg-background/70 p-3">
            <p className="text-[11px] uppercase tracking-wide text-muted-foreground">{tile.label}</p>
            <p className="mt-1 text-lg font-semibold text-foreground">{statusCounts[tile.countKey]}</p>
          </article>
        ))}
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
        <label className="space-y-1.5">
          <span className="text-xs text-muted-foreground">Search by name, image, or domain</span>
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={event => onSearchChange(event.target.value)}
              placeholder="api, postgres, ghcr.io/org/image"
              className="pl-9"
              autoComplete="off"
            />
          </div>
        </label>

        <div className="flex flex-wrap items-center gap-2">
          {STATUS_FILTERS.map(filter => {
            const isActive = statusFilter === filter.value
            return (
              <Button
                key={filter.value}
                type="button"
                size="sm"
                variant={isActive ? 'default' : 'outline'}
                className="h-8 gap-1.5 px-2.5"
                onClick={() => onStatusFilterChange(filter.value)}
              >
                {filter.label}
                <Badge variant={isActive ? 'secondary' : 'outline'} className="h-4 rounded-sm px-1.5 text-[10px]">
                  {statusCounts[filter.countKey]}
                </Badge>
              </Button>
            )
          })}
        </div>
      </div>
    </section>
  )
}
