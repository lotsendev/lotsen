import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
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
            Status: <StatusBadge status={deployment.status} error={deployment.error} />
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
          {deployment.error ? (
            <p className="sm:col-span-2 text-sm text-destructive">Last error: {deployment.error}</p>
          ) : null}
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

      <DeploymentLogsPanel deploymentId={deployment.id} />
    </div>
  )
}
