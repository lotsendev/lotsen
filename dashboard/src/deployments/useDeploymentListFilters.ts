import { useMemo, useState } from 'react'
import type { Deployment, DeploymentStatus } from '../lib/api'

export type DeploymentStatusFilter = DeploymentStatus | 'all'

type StatusCounts = {
  total: number
  healthy: number
  deploying: number
  failed: number
  idle: number
}

const STATUS_PRIORITY: Record<DeploymentStatus, number> = {
  failed: 0,
  deploying: 1,
  healthy: 2,
  idle: 3,
}

export function useDeploymentListFilters(deployments: Deployment[] | undefined) {
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<DeploymentStatusFilter>('all')

  const normalizedSearch = search.trim().toLowerCase()

  const statusCounts = useMemo<StatusCounts>(() => {
    const summary: StatusCounts = {
      total: deployments?.length ?? 0,
      healthy: 0,
      deploying: 0,
      failed: 0,
      idle: 0,
    }

    for (const deployment of deployments ?? []) {
      summary[deployment.status] += 1
    }

    return summary
  }, [deployments])

  const filteredDeployments = useMemo(() => {
    return (deployments ?? [])
      .filter(deployment => {
        if (statusFilter !== 'all' && deployment.status !== statusFilter) {
          return false
        }

        if (!normalizedSearch) {
          return true
        }

        const haystack = `${deployment.name} ${deployment.image} ${deployment.domain}`.toLowerCase()
        return haystack.includes(normalizedSearch)
      })
      .sort((a, b) => {
        const byStatus = STATUS_PRIORITY[a.status] - STATUS_PRIORITY[b.status]
        if (byStatus !== 0) {
          return byStatus
        }
        return a.name.localeCompare(b.name)
      })
  }, [deployments, normalizedSearch, statusFilter])

  const clearFilters = () => {
    setSearch('')
    setStatusFilter('all')
  }

  return {
    search,
    setSearch,
    statusFilter,
    setStatusFilter,
    statusCounts,
    filteredDeployments,
    hasActiveFilters: statusFilter !== 'all' || normalizedSearch.length > 0,
    clearFilters,
  }
}
