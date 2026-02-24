import { useQuery } from '@tanstack/react-query'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { getProxyAccessLogs, getProxySecurityConfig } from '../lib/api'

export function TrafficPage() {
  const logsQuery = useQuery({ queryKey: ['proxy-access-logs'], queryFn: () => getProxyAccessLogs(150), refetchInterval: 5000 })
  const securityQuery = useQuery({ queryKey: ['proxy-security-config'], queryFn: getProxySecurityConfig, refetchInterval: 10000 })

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Active hardening & rate-limit settings</CardTitle>
        </CardHeader>
        <CardContent>
          {securityQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading protection settings...</p>
          ) : securityQuery.isError || !securityQuery.data ? (
            <p className="text-sm text-destructive">Unable to load protection settings.</p>
          ) : (
            <div className="grid gap-2 text-sm sm:grid-cols-2">
              <p>
                Profile: <Badge variant="secondary">{securityQuery.data.profile}</Badge>
              </p>
              <p>Suspicious window: {securityQuery.data.suspiciousWindowSeconds}s</p>
              <p>Suspicious threshold: {securityQuery.data.suspiciousThreshold} requests</p>
              <p>Block duration: {securityQuery.data.suspiciousBlockForSeconds}s</p>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Proxy access logs</CardTitle>
        </CardHeader>
        <CardContent>
          {logsQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading access logs...</p>
          ) : logsQuery.isError || !logsQuery.data ? (
            <p className="text-sm text-destructive">Unable to load access logs.</p>
          ) : logsQuery.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">No access logs yet.</p>
          ) : (
            <div className="space-y-2">
              {logsQuery.data.map((entry, idx) => (
                <div key={`${entry.timestamp}-${idx}`} className="grid gap-1 rounded border bg-muted/20 p-2 text-xs sm:grid-cols-[170px_70px_1fr_70px_120px_1fr]">
                  <span className="font-mono text-muted-foreground">{new Date(entry.timestamp).toLocaleString()}</span>
                  <span className="font-mono">{entry.method}</span>
                  <span className="font-mono break-all">{entry.path}</span>
                  <span className="font-mono">{entry.statusCode}</span>
                  <span className="font-mono">{entry.durationMs}ms</span>
                  <span className="font-mono text-muted-foreground">{entry.clientIp || '-'} → {entry.upstreamTarget || '-'}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
