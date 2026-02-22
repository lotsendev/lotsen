import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { AlertTriangle, ArrowLeft } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { DeploymentLogsPanel } from '../deployments/DeploymentLogsPanel'
import { StatusBadge } from '../deployments/StatusBadge'
import { getDeployments } from '../lib/api'

export function DeploymentDetailPage() {
  const { deploymentId } = useParams({ from: '/deployments/$deploymentId' })
  const { data: deployments, isLoading, isError } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
  })

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading deployment...</p>
  if (isError) return <p className="text-sm text-destructive">Failed to load deployment details.</p>

  const deployment = deployments?.find(item => item.id === deploymentId)
  if (!deployment) {
    return (
      <div className="space-y-3">
        <Button asChild variant="outline" size="sm" className="w-fit">
          <Link to="/deployments">
            <ArrowLeft className="mr-1 h-4 w-4" />
            Back to deployments
          </Link>
        </Button>
        <p className="text-sm text-muted-foreground">Deployment not found.</p>
      </div>
    )
  }

  const envEntries = Object.entries(deployment.envs)

  const renderList = (values: string[], emptyText: string) => {
    if (!values.length) return <p className="text-sm text-muted-foreground">{emptyText}</p>

    return (
      <ul className="space-y-1 text-xs text-muted-foreground">
        {values.map(value => (
          <li key={value} className="rounded bg-muted/40 px-2 py-1 font-mono">
            {value}
          </li>
        ))}
      </ul>
    )
  }

  return (
    <div className="space-y-4">
      <Button asChild variant="outline" size="sm" className="w-fit">
        <Link to="/deployments">
          <ArrowLeft className="mr-1 h-4 w-4" />
          Back to deployments
        </Link>
      </Button>

      {deployment.error && (
        <div className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-destructive">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="space-y-0.5">
            <p className="text-sm font-medium">Container exited with error</p>
            <p className="font-mono text-xs opacity-80">{deployment.error}</p>
          </div>
        </div>
      )}

      <Card>
        <CardHeader className="pb-2">
          <CardTitle>{deployment.name}</CardTitle>
          <CardDescription>All available deployment data and live logs.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
          <p>
            Deployment ID:{' '}
            <span className="font-mono text-xs text-muted-foreground">{deployment.id}</span>
          </p>
          <p>
            Status: <StatusBadge status={deployment.status} />
          </p>
          <p className="sm:col-span-2">
            Image: <span className="font-mono text-xs text-muted-foreground">{deployment.image}</span>
          </p>
          <p className="sm:col-span-2">
            Domain:{' '}
            <span className="font-mono text-xs text-muted-foreground">
              {deployment.domain || 'No domain configured'}
            </span>
          </p>
        </CardContent>
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Ports</CardTitle>
          </CardHeader>
          <CardContent>{renderList(deployment.ports, 'No ports configured.')}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Volumes</CardTitle>
          </CardHeader>
          <CardContent>{renderList(deployment.volumes, 'No volumes configured.')}</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Environment variables</CardTitle>
        </CardHeader>
        <CardContent>
          {envEntries.length ? (
            <div className="space-y-1">
              {envEntries.map(([key, value]) => (
                <div
                  key={key}
                  className="grid grid-cols-[minmax(120px,220px)_1fr] gap-2 rounded bg-muted/30 px-3 py-2 text-xs"
                >
                  <span className="font-mono text-foreground">{key}</span>
                  <span className="font-mono text-muted-foreground break-all">{value || '(empty)'}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No environment variables configured.</p>
          )}
        </CardContent>
      </Card>

      <DeploymentLogsPanel deploymentId={deployment.id} status={deployment.status} error={deployment.error} />
    </div>
  )
}
