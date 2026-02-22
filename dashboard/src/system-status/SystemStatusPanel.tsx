import { useSystemStatus } from './useSystemStatus'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'

function formatTimestamp(timestamp: string) {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }
  return date.toLocaleString()
}

export function SystemStatusPanel() {
  const { status, isLoading, isError } = useSystemStatus()

  return (
    <Card className="bg-card/70">
      <CardHeader>
        <CardTitle>API signal</CardTitle>
        <CardDescription>Current API health and latest update timestamp.</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading && <p className="text-sm text-muted-foreground">Loading system status…</p>}

        {isError && (
          <p className="text-sm text-destructive">Unable to fetch system status right now.</p>
        )}

        {status && !isLoading && !isError && (
          <div className="space-y-2 text-sm text-muted-foreground">
            <p>
              State:{' '}
              <Badge variant={status.api.state === 'healthy' ? 'success' : 'destructive'}>{status.api.state}</Badge>
            </p>
            <p>
              Last updated:{' '}
              <span className="font-medium text-foreground">{formatTimestamp(status.api.lastUpdated)}</span>
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
