import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { getVersionInfo, triggerUpgrade } from '../lib/api'
import { UpgradeLogPanel } from '../settings/UpgradeLogPanel'
import { useUpgradeLogsSSE } from '../settings/useUpgradeLogsSSE'
import { useVersionCheck } from '../settings/useVersionCheck'

function formatDate(value?: string) {
  if (!value) return 'Unavailable'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return 'Unknown'
  return parsed.toLocaleString()
}

export function SettingsPage() {
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [isUpgradeRunning, setIsUpgradeRunning] = useState(false)
  const [awaitingReconnect, setAwaitingReconnect] = useState(false)
  const [reconnectSawOffline, setReconnectSawOffline] = useState(false)
  const [reconnectReady, setReconnectReady] = useState(false)
  const [targetVersion, setTargetVersion] = useState<string | null>(null)
  const [upgradeError, setUpgradeError] = useState<string | null>(null)
  const [attemptId, setAttemptId] = useState(0)

  const { currentVersion, latestVersion, publishedAt, releaseNotes, upgradeAvailable, cachedAt, isLoading, isError } =
    useVersionCheck()
  const { lines, streamClosed } = useUpgradeLogsSSE(isUpgradeRunning)

  const reconnectProbe = useQuery({
    queryKey: ['upgrade-reconnect-probe', attemptId],
    queryFn: getVersionInfo,
    enabled: awaitingReconnect,
    retry: false,
    refetchInterval: 3000,
    refetchIntervalInBackground: true,
  })

  const finishUpgradeRun = (reconnectedCurrentVersion?: string, reconnectedLatestVersion?: string) => {
    setAwaitingReconnect(false)
    setIsUpgradeRunning(false)
    setReconnectReady(true)

    if (reconnectProbe.data) {
      queryClient.setQueryData(['version-check'], reconnectProbe.data)
    }

    const targetReached =
      targetVersion && (reconnectedCurrentVersion === targetVersion || reconnectedLatestVersion === targetVersion)
    if (targetVersion && !targetReached) {
      const lastLines = lines.slice(-8).join('\n')
      setUpgradeError(
        lastLines
          ? `Upgrade did not reach ${targetVersion}. Last log lines:\n${lastLines}`
          : `Upgrade did not reach ${targetVersion}.`,
      )
      return
    }

    setUpgradeError(null)
  }

  useEffect(() => {
    if (!awaitingReconnect) return

    if (reconnectProbe.isError) {
      setReconnectSawOffline(true)
      return
    }

    if (reconnectProbe.isSuccess && reconnectSawOffline) {
      finishUpgradeRun(reconnectProbe.data.currentVersion, reconnectProbe.data.latestVersion)
      return
    }

  }, [awaitingReconnect, reconnectProbe.isError, reconnectProbe.isSuccess, reconnectSawOffline, reconnectProbe.data])

  const startUpgrade = useMutation({
    mutationFn: triggerUpgrade,
    onSuccess: () => {
      setConfirmOpen(false)
      setUpgradeError(null)
      setReconnectReady(false)
      setReconnectSawOffline(false)
      setAwaitingReconnect(true)
      setIsUpgradeRunning(true)
      setTargetVersion(latestVersion ?? null)
      setAttemptId(prev => prev + 1)
    },
    onError: error => {
      setUpgradeError(error instanceof Error ? error.message : 'Failed to start upgrade')
      setConfirmOpen(false)
    },
  })

  const upgradeButtonLabel = useMemo(() => {
    if (startUpgrade.isPending || isUpgradeRunning) return 'Upgrading...'
    if (!latestVersion) return 'Upgrade unavailable'
    return `Upgrade to ${latestVersion}`
  }, [isUpgradeRunning, latestVersion, startUpgrade.isPending])

  const canUpgrade = upgradeAvailable && !isUpgradeRunning && !startUpgrade.isPending

  return (
    <section className="space-y-6">
      {isLoading && <p className="text-sm text-muted-foreground">Checking version information…</p>}

      {isError && <p className="text-sm text-destructive">Unable to fetch version information right now.</p>}

      {reconnectReady && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
          <p>Upgrade complete - click to reload.</p>
          <Button type="button" size="sm" onClick={() => window.location.reload()}>
            <RefreshCw className="h-4 w-4" />
            Reload dashboard
          </Button>
        </div>
      )}

      {upgradeError && (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-destructive">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="text-sm font-medium">Upgrade failed</p>
          </div>
          <pre className="mt-2 overflow-x-auto whitespace-pre-wrap font-mono text-xs">{upgradeError}</pre>
        </div>
      )}

      <div className="rounded-lg border bg-card p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold">Version</h2>
            <p className="mt-1 text-sm text-muted-foreground">Installed release and latest available version.</p>
          </div>
          <Button type="button" disabled={!canUpgrade} onClick={() => setConfirmOpen(true)}>
            {upgradeButtonLabel}
          </Button>
        </div>

        <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">Installed version</dt>
            <dd className="mt-1 font-mono text-foreground">{currentVersion}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Latest available</dt>
            <dd className="mt-1 font-mono text-foreground">{latestVersion ?? 'Unavailable'}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Published</dt>
            <dd className="mt-1 text-foreground">{formatDate(publishedAt)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Cache refreshed</dt>
            <dd className="mt-1 text-foreground">{formatDate(cachedAt)}</dd>
          </div>
        </dl>

        <details className="mt-4 rounded-lg border bg-muted/20 p-3">
          <summary className="cursor-pointer text-sm font-medium">Release notes</summary>
          <pre className="mt-3 overflow-x-auto whitespace-pre-wrap font-mono text-xs text-muted-foreground">
            {releaseNotes?.trim() || 'No release notes available.'}
          </pre>
        </details>
      </div>

      {(isUpgradeRunning || lines.length > 0) && (
        <UpgradeLogPanel lines={lines} isRunning={isUpgradeRunning} streamClosed={streamClosed} />
      )}

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Confirm upgrade</DialogTitle>
            <DialogDescription>
              Dirigent services will briefly restart while the installer runs. Continue only if a short interruption is safe.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirmOpen(false)}>
              Cancel
            </Button>
            <Button type="button" onClick={() => startUpgrade.mutate()} disabled={startUpgrade.isPending}>
              Start upgrade
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
